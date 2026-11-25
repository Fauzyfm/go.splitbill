package repository

import (
	"context"
	"database/sql"
	"fmt"
	"gosplitbill/model"
)

type BillItemRepsitory struct {
	db *sql.DB
}

func NewBillItemRepository(db *sql.DB) *BillItemRepsitory {
	return &BillItemRepsitory{db: db}
}

func (r *BillItemRepsitory) CreateItem(ctx context.Context, BillItem *model.BillItem) error {
	query := `
	INSERT INTO bill_items (bill_id, name, qty, unit_price) VALUES ($1, $2, $3, $4)
	RETURNING id
	`

	err := r.db.QueryRowContext(ctx, query, BillItem.BillID, BillItem.NameItem, BillItem.Qty, BillItem.ItemPrice).Scan(&BillItem.ID)
	if err != nil {
		return fmt.Errorf("gagal membuat item: %w", err)
	}

	return nil
}