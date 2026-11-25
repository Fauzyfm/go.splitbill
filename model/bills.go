package model

import (
	"time"

	"github.com/google/uuid"
)

type StatusBill string

const (
	StatusBillDraft     StatusBill = "draft"
	StatusBillAssigning StatusBill = "assigning"
	StatusBillCompleted StatusBill = "completed"
)

type Bill struct {
	ID               uuid.UUID   `json:"id"`
	UserID           uuid.UUID  `json:"user_id"`
	Title            string     `json:"title"`
	TaxAmount        int64      `json:"tax_amount"`
	ServiceAmount    int64      `json:"service_amount"`
	DiscountAmount   int64      `json:"discount_amount"`
	ReceiptTotal     int64      `json:"receipt_total"`
	ReceiptImagePath string     `json:"receipt_image_path"`
	OcrRawText       string     `json:"ocr_raw_text"`
	Status           StatusBill `json:"status"`
	CreatedAt        time.Time	`json:"created_at"`
	UpdateAt		 time.Time	`json:"update_at"`
}

// Hasil parsing: draft yang siap ditampilkan ke user untuk dikoreksi.
type ParsedReceipt struct {
	Bill        Bill       `json:"bill"`
	Items       []BillItem `json:"items"`
	NeedsReview bool             `json:"needs_review"`
	Warnings    []string         `json:"warnings"`
}


type BillValidasi struct {
	Data ParsedReceipt `json:"data"`
	Participants []Participant `json:"participants"`
}


// hasil edit setelah user menerima parsing dari be
type BillAfterValidasi struct {
	Bill Bill		`json:"bill"`
	Items []BillItem	`json:"items"`
	Participants []Participant	`jaon:"participants"`
	Assignments []ItemAssignmentInput `json:"assignments"`
}
