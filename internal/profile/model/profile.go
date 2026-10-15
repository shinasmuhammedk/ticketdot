package model

import (
	"time"
)

type ProfileResponse struct {
	ID            string    `db:"id" json:"id"`
	Name          string    `db:"name" json:"name"`
	Email         string    `db:"email" json:"email"`
	Phone         string    `db:"phone" json:"phone"`
	AvatarURL     string    `db:"avatar_url" json:"avatar_url"`
	Role          string    `db:"role" json:"role"`
	EmailVerified bool      `db:"email_verified" json:"email_verified"`
	CreatedAt     time.Time `db:"created_at" json:"created_at"`
}

type UpdateProfileRequest struct {
	Name  string `json:"name"`
	Phone string `json:"phone"`
}
