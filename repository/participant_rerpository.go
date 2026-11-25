package repository

import (
	"context"
	"database/sql"
	"fmt"
	"gosplitbill/model"
)

type ParticipantRepository struct {
	db *sql.DB
}

func NewParticipantRepository(db *sql.DB) *ParticipantRepository {
	return &ParticipantRepository{db: db}
}


func(r *ParticipantRepository) CreateParticipant(ctx context.Context, participant *model.Participant) error {
	query := `
	INSERT INTO participants (bill_id, name) values ($1, $2) 
	RETURNING id
	`

	err := r.db.QueryRowContext(ctx, query, participant.BillID, participant.Name).Scan(&participant.ID)
	if err != nil {
		return fmt.Errorf("gagal menambahkan participant: %w", err)
	}

	return nil
}