package repository

import (
	"ticketdot/internal/profile/model"

	"github.com/jmoiron/sqlx"
)

type PostgresRepository struct {
	db *sqlx.DB
}

func NewPostgresRepository(db *sqlx.DB) *PostgresRepository {
	return &PostgresRepository{
		db: db,
	}
}

func (r *PostgresRepository) GetProfile(
	userID string,
) (*model.ProfileResponse, error) {

	var profile model.ProfileResponse

	query := `
	SELECT
    id,
    name,
    email,
    phone,
    COALESCE(avatar_url, '') AS avatar_url,
    role,
    email_verified,
    created_at
FROM users
WHERE id = $1
	`

	err := r.db.Get(
		&profile,
		query,
		userID,
	)

	if err != nil {
		return nil, err
	}

	return &profile, nil
}

func (r *PostgresRepository) UpdateProfile(
	userID string,
	req *model.UpdateProfileRequest,
) error {

	query := `
	UPDATE users
	SET
		name = $1,
		phone = $2,
		updated_at = NOW()
	WHERE id = $3
	`

	_, err := r.db.Exec(
		query,
		req.Name,
		req.Phone,
		userID,
	)

	return err
}
