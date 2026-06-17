package repository

import (
	"github.com/jmoiron/sqlx"

	"ticketdot/internal/auth/model"
)

type PostgresRepository struct {
	db *sqlx.DB
}

func NewPostgresRepository(db *sqlx.DB) *PostgresRepository {
	return &PostgresRepository{
		db: db,
	}
}

var _ AuthRepository = (*PostgresRepository)(nil)

func (r *PostgresRepository) GetUserByEmail(email string) (*model.User, error) {
	query := `
		SELECT
			id,
			name,
			email,
			phone,
			password_hash,
			role,
            operator_id,
			email_verified
		FROM users
		WHERE email = $1
	`

	var user model.User

	err := r.db.Get(&user, query, email)
	if err != nil {
		return nil, err
	}

	return &user, nil
}


func (r *PostgresRepository) CreateUser(user *model.User) error {
	query := `
		INSERT INTO users (
			name,
			email,
			phone,
			password_hash,
			role
		)
		VALUES (
			:name,
			:email,
			:phone,
			:password_hash,
			:role
		)
		RETURNING id
	`

	rows, err := r.db.NamedQuery(query, user)
	if err != nil {
		return err
	}
	defer rows.Close()

	if rows.Next() {
		return rows.Scan(&user.ID)
	}

	return nil
}

func (r *PostgresRepository) GetUserByID(id string) (*model.User, error) {
	query := `
		SELECT
			id,
			name,
			email,
			phone,
			password_hash,
			role,
            operator_id,
			email_verified
		FROM users
		WHERE id = $1
	`

	var user model.User

	err := r.db.Get(&user, query, id)
	if err != nil {
		return nil, err
	}

	return &user, nil
}


func (r *PostgresRepository) GetOperatorIDByUserID(userID string) (string, error) {
	var operatorID string

	err := r.db.Get(&operatorID, `
		SELECT operator_id
		FROM users
		WHERE id = $1
		  AND operator_id IS NOT NULL
	`, userID)

	if err != nil {
		return "", err
	}

	return operatorID, nil
}