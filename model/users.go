package model

import (
	"time"

	"github.com/google/uuid"
)

type User struct {
	ID        uuid.UUID   `json:"id"`
	Email     string `json:"email"`
	Password  string `json:"password"`
	UserName  string `json:"username"`
	CreatedAt time.Time `json:"created_at"`
}