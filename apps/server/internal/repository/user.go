package repository

import (
	"context"
	"database/sql"

	"clipshare/internal/models"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

type UserRepository interface {
	Create(ctx context.Context, username string) (*models.User, error)
	CreateAdmin(ctx context.Context, username, passwordHash string) (*models.User, error)
	GetByID(ctx context.Context, id uuid.UUID) (*models.User, error)
	GetByUsername(ctx context.Context, username string) (*models.User, error)
	GetAdmin(ctx context.Context) (*models.User, error)
	AnyAdminExists(ctx context.Context) (bool, error)
	Update(ctx context.Context, user *models.User) error
	UpdateLastLogin(ctx context.Context, id uuid.UUID) error
	UpdateStorageUsed(ctx context.Context, id uuid.UUID, delta int64) error
}

type userRepo struct {
	db *sqlx.DB
}

func NewUserRepository(db *sqlx.DB) UserRepository {
	return &userRepo{db: db}
}

func (r *userRepo) Create(ctx context.Context, username string) (*models.User, error) {
	user := &models.User{
		Username:          username,
		StorageQuotaBytes: 5368709120, // 5GB default
	}

	query := `
		INSERT INTO users (username, storage_quota_bytes)
		VALUES ($1, $2)
		RETURNING id, username, is_admin, storage_used_bytes, storage_quota_bytes, created_at, updated_at
	`

	row := r.db.QueryRowxContext(ctx, query, user.Username, user.StorageQuotaBytes)
	if err := row.Scan(
		&user.ID,
		&user.Username,
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

func (r *userRepo) CreateAdmin(ctx context.Context, username, passwordHash string) (*models.User, error) {
	user := &models.User{
		Username:          username,
		StorageQuotaBytes: 5368709120,
	}

	query := `
		INSERT INTO users (username, password_hash, is_admin, storage_quota_bytes)
		VALUES ($1, $2, true, $3)
		RETURNING id, username, is_admin, storage_used_bytes, storage_quota_bytes, created_at, updated_at
	`
	row := r.db.QueryRowxContext(ctx, query, username, passwordHash, user.StorageQuotaBytes)
	if err := row.Scan(
		&user.ID,
		&user.Username,
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

func (r *userRepo) GetAdmin(ctx context.Context) (*models.User, error) {
	var user models.User
	query := `SELECT * FROM users WHERE is_admin = true ORDER BY created_at ASC LIMIT 1`
	if err := r.db.GetContext(ctx, &user, query); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &user, nil
}

func (r *userRepo) AnyAdminExists(ctx context.Context) (bool, error) {
	var exists bool
	if err := r.db.GetContext(ctx, &exists, `SELECT EXISTS(SELECT 1 FROM users WHERE is_admin = true AND password_hash IS NOT NULL)`); err != nil {
		return false, err
	}
	return exists, nil
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

func (r *userRepo) GetByUsername(ctx context.Context, username string) (*models.User, error) {
	var user models.User
	query := `SELECT * FROM users WHERE username = $1`
	if err := r.db.GetContext(ctx, &user, query, username); err != nil {
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
		SET display_name = $2, avatar_url = $3, custom_domain = $4, updated_at = NOW()
		WHERE id = $1
	`
	_, err := r.db.ExecContext(ctx, query, user.ID, user.DisplayName, user.AvatarURL, user.CustomDomain)
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
