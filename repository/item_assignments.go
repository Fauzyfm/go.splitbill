package repository

import (
	"context"
	"database/sql"
	"fmt"
	"gosplitbill/model"
)

type ItemAssignmentRepository struct {
	db *sql.DB
}

func NewItemAssignMentRepository(db *sql.DB) *ItemAssignmentRepository {
	return &ItemAssignmentRepository{db: db}
}

func(r *ItemAssignmentRepository) CreateItemAssignment(ctx context.Context, ItemAssign *model.ItemAssignment) error {

	query := `
	INSERT INTO item_assignments (item_id, participant_id, weight) VALUES ($1, $2, $3) 
	`

	_, err := r.db.ExecContext(ctx, query, ItemAssign.ItemID, ItemAssign.ParticipantID, ItemAssign.Weight)
	if err != nil {
		return fmt.Errorf("gagal membuat item assignment: %w", err)
	}
	

	return nil
}