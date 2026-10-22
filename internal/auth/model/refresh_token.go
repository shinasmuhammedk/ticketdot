package model

import "time"

type RefreshToken struct {
	ID        string     `db:"id" json:"id"`
	UserID    string     `db:"user_id" json:"user_id"`
	FamilyID  string     `db:"family_id" json:"family_id"`
	TokenHash string     `db:"token_hash" json:"token_hash"`
	ExpiresAt time.Time  `db:"expires_at" json:"expires_at"`
	RevokedAt *time.Time `db:"revoked_at" json:"revoked_at"`
	CreatedAt time.Time  `db:"created_at" json:"created_at"`
}
