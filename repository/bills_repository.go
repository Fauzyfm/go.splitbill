package repository

import (
	"context"
	"database/sql"
	"fmt"
	"gosplitbill/model"
)

type PostgraseBillsRepository struct {
	db *sql.DB
}


func NewPostgraseBillsRepository(db *sql.DB) *PostgraseBillsRepository {
	return &PostgraseBillsRepository{db: db}
}

func(r *PostgraseBillsRepository) CreateBill(ctx context.Context, bill *model.Bill) error {
	query := `INSERT INTO bills (user_id, title, tax_amount, service_amount, discount_amount, receipt_total,receipt_image_path, ocr_raw_text, status, created_at, updated_at) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, now(), now()) RETURNING id, created_at`

	err := r.db.QueryRowContext(ctx, query, bill.UserID, bill.Title, bill.TaxAmount, bill.ServiceAmount, bill.DiscountAmount, bill.ReceiptTotal, bill.ReceiptImagePath, bill.OcrRawText, bill.Status).Scan(&bill.ID, &bill.CreatedAt)
	if err != nil {
		return fmt.Errorf("gagal membuat bill: %w", err)
	}

	return nil
}