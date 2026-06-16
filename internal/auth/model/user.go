package model

type User struct {
	ID             string `db:"id" json:"id"`
	Name           string `db:"name" json:"name"`
	Email          string `db:"email" json:"email"`
	Phone          string `db:"phone" json:"phone"`
	PasswordHash   string `db:"password_hash" json:"-"`
	Role           string `db:"role" json:"role"`
	EmailVerified  bool   `db:"email_verified" json:"email_verified"`
}