package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"

	"time"

	"gosplitbill/model"
	"gosplitbill/repository"

	"github.com/google/uuid"
)

const (
	ocrAPIURL          = "https://my-project-paddleocr-api.zfehlc.easypanel.host/ocr"
	MaxOCRUploadSize   = 10 << 20
	maxOCRResponseSize = 4 << 20
)

type BillService struct {
	billRepo *repository.PostgraseBillsRepository
	billItemRepo *repository.BillItemRepsitory
	participantRepo *repository.ParticipantRepository
	assignmentRepo *repository.ItemAssignmentRepository
}

func NewBillService(billRepo *repository.PostgraseBillsRepository, billItemRepo *repository.BillItemRepsitory, participantRepo *repository.ParticipantRepository, assignmentRepo *repository.ItemAssignmentRepository) *BillService {
	return &BillService{
		billRepo: billRepo,
		billItemRepo: billItemRepo,
		participantRepo: participantRepo,
		assignmentRepo: assignmentRepo,
	}
}

// ScanReceipt = gambar -> OCR -> parsing -> draft Bill + BillItem.
// Belum menyimpan apa pun ke database; hasilnya untuk dikoreksi user dulu.
func (s *BillService) ScanReceipt(ctx context.Context, image *multipart.FileHeader, userID uuid.UUID) (*model.ParsedReceipt, error) {
	// 1. Kirim gambar ke API OCR (kode lama Anda, tidak berubah)
	raw, err := s.requestOCR(ctx, image)
	if err != nil {
		return nil, err
	}

	// 2. JSON mentah -> struct. Field yang tidak dipakai (polygon, dll) diabaikan otomatis.
	var ocr OCRResponse
	if err := json.Unmarshal(raw, &ocr); err != nil {
		return nil, fmt.Errorf("format respons OCR tidak sesuai: %w", err)
	}
	if !ocr.Success {
		return nil, fmt.Errorf("OCR API melaporkan proses tidak berhasil")
	}

	// 3. Parsing teks OCR menjadi data terstruktur (receipt_parser.go)
	parsed := ParseReceipt(ocr, userID)
	return &parsed, nil
}

// requestOCR = fungsi SendImageForOcr yang lama, hanya diganti nama dan dibuat private.
func (s *BillService) requestOCR(ctx context.Context, image *multipart.FileHeader) (json.RawMessage, error) {
	if image == nil {
		return nil, fmt.Errorf("file gambar wajib diisi")
	}
	if image.Size > MaxOCRUploadSize {
		return nil, fmt.Errorf("ukuran gambar melebihi batas 10 MB")
	}

	source, err := image.Open()
	if err != nil {
		return nil, fmt.Errorf("gagal membuka file gambar: %w", err)
	}
	defer source.Close()

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", filepath.Base(image.Filename))
	if err != nil {
		return nil, fmt.Errorf("gagal menyiapkan file gambar: %w", err)
	}
	if _, err := io.Copy(part, source); err != nil {
		return nil, fmt.Errorf("gagal membaca file gambar: %w", err)
	}
	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("gagal menyiapkan request OCR: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, ocrAPIURL, &body)
	if err != nil {
		return nil, fmt.Errorf("gagal membuat request OCR: %w", err)
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	if apiKey := os.Getenv("API_KEY"); apiKey != "" {
		req.Header.Set("X-API-Key", apiKey)
	}

	client := &http.Client{Timeout: 2 * time.Minute}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("gagal menghubungi OCR API: %w", err)
	}
	defer resp.Body.Close()

	responseBody, err := io.ReadAll(io.LimitReader(resp.Body, maxOCRResponseSize+1))
	if err != nil {
		return nil, fmt.Errorf("gagal membaca respons OCR API: %w", err)
	}
	if len(responseBody) > maxOCRResponseSize {
		return nil, fmt.Errorf("respons OCR API melebihi batas ukuran")
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("OCR API mengembalikan status %d: %s", resp.StatusCode, responseBody)
	}
	if !json.Valid(responseBody) {
		return nil, fmt.Errorf("OCR API mengembalikan respons JSON tidak valid")
	}

	return json.RawMessage(responseBody), nil
}


func (s *BillService) CreateBillAfterValidasi(ctx context.Context, FixBill *model.BillAfterValidasi) ([]model.ParticipantBill, error) {
	
	if FixBill.Bill.Title == "" {
		return nil, fmt.Errorf("Title Wajib untuk di  isi")
	}
	if len(FixBill.Participants) == 0 {
		return nil, fmt.Errorf("Pastikan participant sudah terdaftar")
	}


	Bill := &model.Bill{
		UserID: FixBill.Bill.UserID,
		Title: FixBill.Bill.Title,
		TaxAmount: FixBill.Bill.TaxAmount,
		ServiceAmount: FixBill.Bill.ServiceAmount,
		DiscountAmount: FixBill.Bill.DiscountAmount,
		ReceiptTotal: FixBill.Bill.ReceiptTotal,
		ReceiptImagePath: FixBill.Bill.ReceiptImagePath,
		OcrRawText: FixBill.Bill.OcrRawText,
		Status: model.StatusBillCompleted,
		CreatedAt: time.Time{},
		UpdateAt: time.Time{},
	}

	if err := s.billRepo.CreateBill(ctx, Bill); err != nil {
		return nil, fmt.Errorf("Gagal membuat bill: %w", err)
	}

	for i := range FixBill.Items {
		item := &FixBill.Items[i]
		item.BillID = Bill.ID

		if err := s.billItemRepo.CreateItem(ctx, item); err != nil {
			return nil, fmt.Errorf("gagal membuat bill item %w", err)
		}
	}


	for i := range FixBill.Participants {
		participant := &FixBill.Participants[i]
		participant.BillID = Bill.ID

		if err := s.participantRepo.CreateParticipant(ctx, participant); err != nil {
			return nil, fmt.Errorf("gagal membuatt participant: %w", err)
		}
	}


	// Hitung total weight per item sebelum menyimpan assignment.
	totalWeightByItem := make(map[int]int)
	lastAssignmentByItem := make(map[int]int)

	for i, input := range FixBill.Assignments {
		if input.ItemIndex < 0 || input.ItemIndex >= len(FixBill.Items) {
			return nil, fmt.Errorf("item_index tidak valid")
		}
		if input.ParticipantIndex < 0 || input.ParticipantIndex >= len(FixBill.Participants) {
			return nil, fmt.Errorf("participant_index tidak valid")
		}
		if input.Weight <= 0 {
			return nil, fmt.Errorf("weight harus lebih besar dari 0")
		}

		totalWeightByItem[input.ItemIndex] += input.Weight
		lastAssignmentByItem[input.ItemIndex] = i
	}

	// Siapkan satu hasil untuk setiap participant yang sudah dibuat.
	result := make([]model.ParticipantBill, len(FixBill.Participants))
	for i, participant := range FixBill.Participants {
		result[i] = model.ParticipantBill{
			Participant: participant,
			Items:       []model.ParticipantBillItem{},
		}
	}

	allocatedByItem := make(map[int]int64)

	for i, input := range FixBill.Assignments {
		item := FixBill.Items[input.ItemIndex]
		participant := FixBill.Participants[input.ParticipantIndex]

		assignment := &model.ItemAssignment{
			ItemID:        item.ID,
			ParticipantID: participant.ID,
			Weight:        input.Weight,
		}

		if err := s.assignmentRepo.CreateItemAssignment(ctx, assignment); err != nil {
			return nil, fmt.Errorf("gagal membuat item assignment: %w", err)
		}

		// Contoh perhitungan: harga satuan × jumlah, dibagi berdasarkan weight.
		itemTotal := item.ItemPrice * int64(item.Qty)
		amountDue := itemTotal * int64(input.Weight) /
			int64(totalWeightByItem[input.ItemIndex])

		// Masukkan sisa pembulatan ke assignment terakhir untuk item ini.
		if i == lastAssignmentByItem[input.ItemIndex] {
			amountDue = itemTotal - allocatedByItem[input.ItemIndex]
		}
		allocatedByItem[input.ItemIndex] += amountDue

		participantResult := &result[input.ParticipantIndex]
		participantResult.Items = append(
			participantResult.Items,
			model.ParticipantBillItem{
				Item:      item,
				Weight:    input.Weight,
				AmountDue: amountDue,
			},
		)
		participantResult.TotalDue += amountDue
	}

return result, nil
}