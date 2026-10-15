package model

import "time"

type PasswordReset struct {
	ID        string    `db:"id" json:"id"`
	UserID    string    `db:"user_id" json:"user_id"`
	Email     string    `db:"email" json:"email"`
	Token     string    `db:"token" json:"token"`
	ExpiresAt time.Time `db:"expires_at" json:"expires_at"`
	Used      bool      `db:"used" json:"used"`
	CreatedAt time.Time `db:"created_at" json:"created_at"`
}
