package repository

import (
	"context"

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

func (r *PostgresRepository) CreatePasswordReset(reset *model.PasswordReset) error {
	query := `
		INSERT INTO password_resets (
			user_id,
			email,
			token,
			expires_at,
			used
		) VALUES (
			:user_id,
			:email,
			:token,
			:expires_at,
			:used
		)
		RETURNING id, created_at
	`

	rows, err := r.db.NamedQuery(query, reset)
	if err != nil {
		return err
	}
	defer rows.Close()

	if rows.Next() {
		return rows.Scan(&reset.ID, &reset.CreatedAt)
	}

	return nil
}

func (r *PostgresRepository) GetValidPasswordReset(token string) (*model.PasswordReset, error) {
	query := `
		SELECT
			id,
			user_id,
			email,
			token,
			expires_at,
			used,
			created_at
		FROM password_resets
		WHERE token = $1
		  AND used = FALSE
		  AND expires_at > NOW()
	`

	var reset model.PasswordReset
	err := r.db.Get(&reset, query, token)
	if err != nil {
		return nil, err
	}

	return &reset, nil
}

func (r *PostgresRepository) MarkPasswordResetUsed(token string) error {
	query := `
		UPDATE password_resets
		SET used = TRUE
		WHERE token = $1
	`
	_, err := r.db.Exec(query, token)
	return err
}

func (r *PostgresRepository) UpdateUserPassword(userID string, passwordHash string) error {
	query := `
		UPDATE users
		SET password_hash = $1,
		    updated_at = NOW()
		WHERE id = $2
	`
	_, err := r.db.Exec(query, passwordHash, userID)
	return err
}

const refreshTokenCleanupLockID = 982347128934

func (r *PostgresRepository) CreateRefreshToken(token *model.RefreshToken) error {
	query := `
		INSERT INTO refresh_tokens (
			user_id,
			family_id,
			token_hash,
			expires_at,
			revoked_at
		) VALUES (
			$1, $2, $3, $4, $5
		)
		RETURNING id, created_at
	`
	return r.db.QueryRow(query, token.UserID, token.FamilyID, token.TokenHash, token.ExpiresAt, token.RevokedAt).Scan(&token.ID, &token.CreatedAt)
}

func (r *PostgresRepository) GetRefreshToken(tokenHash string) (*model.RefreshToken, error) {
	query := `
		SELECT id, user_id, family_id, token_hash, expires_at, revoked_at, created_at
		FROM refresh_tokens
		WHERE token_hash = $1
	`
	var t model.RefreshToken
	err := r.db.Get(&t, query, tokenHash)
	if err != nil {
		return nil, err
	}
	return &t, nil
}

func (r *PostgresRepository) RotateRefreshToken(tokenHash string) (*model.RefreshToken, error) {
	query := `
		UPDATE refresh_tokens
		SET revoked_at = NOW()
		WHERE token_hash = $1
		  AND revoked_at IS NULL
		  AND expires_at > NOW()
		RETURNING id, user_id, family_id, token_hash, expires_at, revoked_at, created_at
	`
	var t model.RefreshToken
	err := r.db.Get(&t, query, tokenHash)
	if err != nil {
		return nil, err
	}
	return &t, nil
}

func (r *PostgresRepository) RevokeFamily(familyID string) error {
	query := `
		UPDATE refresh_tokens
		SET revoked_at = NOW()
		WHERE family_id = $1
		  AND revoked_at IS NULL
	`
	_, err := r.db.Exec(query, familyID)
	return err
}

func (r *PostgresRepository) RevokeRefreshToken(tokenHash string) error {
	query := `
		UPDATE refresh_tokens
		SET revoked_at = NOW()
		WHERE token_hash = $1
		  AND revoked_at IS NULL
	`
	_, err := r.db.Exec(query, tokenHash)
	return err
}

func (r *PostgresRepository) RevokeUserRefreshTokens(userID string) error {
	query := `
		UPDATE refresh_tokens
		SET revoked_at = NOW()
		WHERE user_id = $1
		  AND revoked_at IS NULL
	`
	_, err := r.db.Exec(query, userID)
	return err
}

func (r *PostgresRepository) DeleteExpiredRefreshTokens(ctx context.Context) (int64, error) {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	var acquired bool
	if err := tx.GetContext(ctx, &acquired, "SELECT pg_try_advisory_xact_lock($1)", refreshTokenCleanupLockID); err != nil {
		return 0, err
	}
	if !acquired {
		return 0, nil
	}

	res, err := tx.ExecContext(ctx, `
		DELETE FROM refresh_tokens
		WHERE expires_at < NOW() - INTERVAL '7 days'
	`)
	if err != nil {
		return 0, err
	}

	count, _ := res.RowsAffected()
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return count, nil
}

