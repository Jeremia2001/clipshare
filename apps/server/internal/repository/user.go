package repository

import (
	"context"
	"database/sql"
	"time"

	"clipshare/internal/models"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

type UserRepository interface {
	Create(ctx context.Context, email string) (*models.User, error)
	GetByID(ctx context.Context, id uuid.UUID) (*models.User, error)
	GetByEmail(ctx context.Context, email string) (*models.User, error)
	Update(ctx context.Context, user *models.User) error
	UpdateLastLogin(ctx context.Context, id uuid.UUID) error
	UpdateStorageUsed(ctx context.Context, id uuid.UUID, delta int64) error
	VerifyEmail(ctx context.Context, id uuid.UUID) error
}

type userRepo struct {
	db *sqlx.DB
}

func NewUserRepository(db *sqlx.DB) UserRepository {
	return &userRepo{db: db}
}

func (r *userRepo) Create(ctx context.Context, email string) (*models.User, error) {
	user := &models.User{
		Email:             email,
		StorageQuotaBytes: 5368709120, // 5GB default
	}

	query := `
		INSERT INTO users (email, storage_quota_bytes)
		VALUES ($1, $2)
		RETURNING id, email, is_verified, is_admin, storage_used_bytes, storage_quota_bytes, created_at, updated_at
	`

	row := r.db.QueryRowxContext(ctx, query, user.Email, user.StorageQuotaBytes)
	if err := row.Scan(
		&user.ID,
		&user.Email,
		&user.IsVerified,
		&user.IsAdmin,
		&user.StorageUsedBytes,
		&user.StorageQuotaBytes,
		&user.CreatedAt,
		&user.UpdatedAt,
	); err != nil {
		return nil, err
	}

	return user, nil
}

func (r *userRepo) GetByID(ctx context.Context, id uuid.UUID) (*models.User, error) {
	var user models.User
	query := `SELECT * FROM users WHERE id = $1`
	if err := r.db.GetContext(ctx, &user, query, id); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &user, nil
}

func (r *userRepo) GetByEmail(ctx context.Context, email string) (*models.User, error) {
	var user models.User
	query := `SELECT * FROM users WHERE email = $1`
	if err := r.db.GetContext(ctx, &user, query, email); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &user, nil
}

func (r *userRepo) Update(ctx context.Context, user *models.User) error {
	query := `
		UPDATE users 
		SET username = $2, display_name = $3, avatar_url = $4, custom_domain = $5, updated_at = NOW()
		WHERE id = $1
	`
	_, err := r.db.ExecContext(ctx, query, user.ID, user.Username, user.DisplayName, user.AvatarURL, user.CustomDomain)
	return err
}

func (r *userRepo) UpdateLastLogin(ctx context.Context, id uuid.UUID) error {
	query := `UPDATE users SET last_login_at = NOW() WHERE id = $1`
	_, err := r.db.ExecContext(ctx, query, id)
	return err
}

func (r *userRepo) UpdateStorageUsed(ctx context.Context, id uuid.UUID, delta int64) error {
	query := `
		UPDATE users 
		SET storage_used_bytes = GREATEST(0, storage_used_bytes + $2)
		WHERE id = $1
	`
	_, err := r.db.ExecContext(ctx, query, id, delta)
	return err
}

func (r *userRepo) VerifyEmail(ctx context.Context, id uuid.UUID) error {
	query := `UPDATE users SET is_verified = true, updated_at = NOW() WHERE id = $1`
	_, err := r.db.ExecContext(ctx, query, id)
	return err
}

// MagicTokenRepository

type MagicTokenRepository interface {
	Create(ctx context.Context, userID uuid.UUID, tokenHash string, expiresAt time.Time) (*models.MagicToken, error)
	GetByHash(ctx context.Context, hash string) (*models.MagicToken, error)
	MarkUsed(ctx context.Context, id uuid.UUID) error
	DeleteExpired(ctx context.Context) error
}

type magicTokenRepo struct {
	db *sqlx.DB
}

func NewMagicTokenRepository(db *sqlx.DB) MagicTokenRepository {
	return &magicTokenRepo{db: db}
}

func (r *magicTokenRepo) Create(ctx context.Context, userID uuid.UUID, tokenHash string, expiresAt time.Time) (*models.MagicToken, error) {
	token := &models.MagicToken{
		UserID:    userID,
		TokenHash: tokenHash,
		ExpiresAt: expiresAt,
	}

	query := `
		INSERT INTO magic_tokens (user_id, token_hash, expires_at)
		VALUES ($1, $2, $3)
		RETURNING id, user_id, token_hash, expires_at, used_at, created_at
	`

	row := r.db.QueryRowxContext(ctx, query, userID, tokenHash, expiresAt)
	if err := row.Scan(
		&token.ID, &token.UserID, &token.TokenHash, &token.ExpiresAt, &token.UsedAt, &token.CreatedAt,
	); err != nil {
		return nil, err
	}

	return token, nil
}

func (r *magicTokenRepo) GetByHash(ctx context.Context, hash string) (*models.MagicToken, error) {
	var token models.MagicToken
	query := `SELECT * FROM magic_tokens WHERE token_hash = $1 AND used_at IS NULL AND expires_at > NOW()`
	if err := r.db.GetContext(ctx, &token, query, hash); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &token, nil
}

func (r *magicTokenRepo) MarkUsed(ctx context.Context, id uuid.UUID) error {
	query := `UPDATE magic_tokens SET used_at = NOW() WHERE id = $1`
	_, err := r.db.ExecContext(ctx, query, id)
	return err
}

func (r *magicTokenRepo) DeleteExpired(ctx context.Context) error {
	query := `DELETE FROM magic_tokens WHERE expires_at < NOW() OR used_at IS NOT NULL`
	_, err := r.db.ExecContext(ctx, query)
	return err
}

// RefreshTokenRepository

type RefreshTokenRepository interface {
	Create(ctx context.Context, userID uuid.UUID, tokenHash string, expiresAt time.Time) (*models.RefreshToken, error)
	GetByHash(ctx context.Context, hash string) (*models.RefreshToken, error)
	Revoke(ctx context.Context, id uuid.UUID) error
	RevokeAllForUser(ctx context.Context, userID uuid.UUID) error
}

type refreshTokenRepo struct {
	db *sqlx.DB
}

func NewRefreshTokenRepository(db *sqlx.DB) RefreshTokenRepository {
	return &refreshTokenRepo{db: db}
}

func (r *refreshTokenRepo) Create(ctx context.Context, userID uuid.UUID, tokenHash string, expiresAt time.Time) (*models.RefreshToken, error) {
	token := &models.RefreshToken{
		UserID:    userID,
		TokenHash: tokenHash,
		ExpiresAt: expiresAt,
	}

	query := `
		INSERT INTO refresh_tokens (user_id, token_hash, expires_at)
		VALUES ($1, $2, $3)
		RETURNING id, user_id, token_hash, expires_at, revoked_at, created_at
	`

	row := r.db.QueryRowxContext(ctx, query, userID, tokenHash, expiresAt)
	if err := row.Scan(
		&token.ID, &token.UserID, &token.TokenHash, &token.ExpiresAt, &token.RevokedAt, &token.CreatedAt,
	); err != nil {
		return nil, err
	}

	return token, nil
}

func (r *refreshTokenRepo) GetByHash(ctx context.Context, hash string) (*models.RefreshToken, error) {
	var token models.RefreshToken
	query := `SELECT * FROM refresh_tokens WHERE token_hash = $1 AND revoked_at IS NULL AND expires_at > NOW()`
	if err := r.db.GetContext(ctx, &token, query, hash); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &token, nil
}

func (r *refreshTokenRepo) Revoke(ctx context.Context, id uuid.UUID) error {
	query := `UPDATE refresh_tokens SET revoked_at = NOW() WHERE id = $1`
	_, err := r.db.ExecContext(ctx, query, id)
	return err
}

func (r *refreshTokenRepo) RevokeAllForUser(ctx context.Context, userID uuid.UUID) error {
	query := `UPDATE refresh_tokens SET revoked_at = NOW() WHERE user_id = $1 AND revoked_at IS NULL`
	_, err := r.db.ExecContext(ctx, query, userID)
	return err
}
