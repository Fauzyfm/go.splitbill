package model

import "github.com/google/uuid"

type Participant struct {
	ID     uuid.UUID `json:"id"`
	BillID uuid.UUID  `json:"bill_id"`
	Name   string    `json:"name"`
}


type ParticipantBillItem struct {
    Item     BillItem `json:"item"`
    Weight   int      `json:"weight"`
    AmountDue int64   `json:"amount_due"`
}

type ParticipantBill struct {
	Participant Participant `json:"participant"`
	Items []ParticipantBillItem `json:"items"`
	TotalDue int64 `json:"total_due"`
}