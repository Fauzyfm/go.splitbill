package model

import "github.com/google/uuid"

type ItemAssignment struct {
	ItemID        uuid.UUID `json:"item_id"`
	ParticipantID uuid.UUID `json:"participant_id"`
	Weight        int       `json:"weight"`
}

type ItemAssignmentInput struct {
	ItemIndex int `json:"item_index"`
	ParticipantIndex int `json:"participant_index"`
	Weight int `json:"weight"`
}