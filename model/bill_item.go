package model

import "github.com/google/uuid"

type BillItem struct {
	ID        uuid.UUID `json:"id"`
	BillID    uuid.UUID    `json:"bill_id"`
	NameItem  string  `json:"item_name"`
	Qty       int     `json:"qty"`
	ItemPrice int64   `json:"item_price"`
}