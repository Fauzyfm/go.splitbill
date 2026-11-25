package repository

import (
	"context"
	"database/sql"
	"fmt"
	"gosplitbill/model"

	"github.com/google/uuid"
)


type AuthRepository interface {
	Create(ctx context.Context, user *model.User) error
	GetByEmail(ctx context.Context, email string) (*model.User, error)
	GetByID(ctx context.Context, id uuid.UUID) (*model.User, error)
	UpdatePassword(ctx context.Context, NewPassword string, userid uuid.UUID) error
}


type PostgraseAuthRepository struct {
	db *sql.DB
}

func NewPostgraseAuthRepository(db *sql.DB) *PostgraseAuthRepository {
	return &PostgraseAuthRepository{db: db}
}

func(r *PostgraseAuthRepository) Create(ctx context.Context, user *model.User) error {
	query := `
		INSERT INTO users (email, password_hash, name, created_at) VALUES ($1, $2, $3, now())
		RETURNING id, created_at
	`

	err := r.db.QueryRowContext(ctx, query, user.Email, user.Password, user.UserName).Scan(&user.ID, &user.CreatedAt)
	if err != nil {
		return fmt.Errorf("gagal memuat user: %w", err)
	}

	return nil
}

func (r *PostgraseAuthRepository) GetByEmail(ctx context.Context, email string) (*model.User, error) {
	query := `
		SELECT id, email, password_hash, name, created_at
		FROM users
		WHERE email = $1
	`

	var user model.User
	err := r.db.QueryRowContext(ctx, query, email).Scan(&user.ID, &user.Email, &user.Password, &user.UserName, &user.CreatedAt)

	if err != nil {
		return nil, fmt.Errorf("gagal mengambil data user: %w", err)
	}


	return &user, nil
}


func (r *PostgraseAuthRepository) GetByID(ctx context.Context, id uuid.UUID) (*model.User, error) {
	query := `
		SELECT id, email, password_hash, name, created_at
		FROM users
		WHERE id = $1
	`

	var user model.User
		err := r.db.QueryRowContext(ctx, query, id).Scan(&user.ID, &user.Email, &user.Password, &user.UserName, &user.CreatedAt)

	if err != nil {
		return nil, fmt.Errorf("gagal mengambil data user: %w", err)
	}

	return &user, nil
}


func (r *PostgraseAuthRepository) UpdatePassword(ctx context.Context, NewPassword string, userid uuid.UUID) error {
	query := `
		UPDATE users
		SET password = $1
		WHERE id = $2
	`

	res, err := r.db.ExecContext(ctx, query, NewPassword, userid)
	if err != nil {
		return fmt.Errorf("Gagal update password!")
	}

	rows, _ := res.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("user tidak ditemukan")
	}

	return nil
}

