package repository

import (
	"context"
	"database/sql"
	"fmt"

	"clipshare/internal/models"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

type ShareRepository interface {
	Create(ctx context.Context, share *models.Share) (*models.Share, error)
	GetByID(ctx context.Context, id uuid.UUID) (*models.Share, error)
	GetByShareCode(ctx context.Context, code string) (*models.Share, error)
	GetByCustomSlug(ctx context.Context, slug string) (*models.Share, error)
	ListByClipID(ctx context.Context, clipID uuid.UUID) ([]*models.Share, error)
	IncrementViewCount(ctx context.Context, id uuid.UUID) error
	Deactivate(ctx context.Context, id uuid.UUID) error
	Delete(ctx context.Context, id uuid.UUID) error
}

type shareRepo struct {
	db *sqlx.DB
}

func NewShareRepository(db *sqlx.DB) ShareRepository {
	return &shareRepo{db: db}
}

func (r *shareRepo) Create(ctx context.Context, share *models.Share) (*models.Share, error) {
	query := `
		INSERT INTO shares (clip_id, user_id, share_code, custom_slug, password_hash, expires_at, max_views)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, view_count, is_active, created_at
	`

	err := r.db.QueryRowxContext(ctx, query,
		share.ClipID, share.UserID, share.ShareCode, share.CustomSlug,
		share.PasswordHash, share.ExpiresAt, share.MaxViews,
	).Scan(
		&share.ID, &share.ViewCount, &share.IsActive, &share.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create share: %w", err)
	}

	return share, nil
}

func (r *shareRepo) GetByID(ctx context.Context, id uuid.UUID) (*models.Share, error) {
	var share models.Share
	query := `SELECT * FROM shares WHERE id = $1`
	if err := r.db.GetContext(ctx, &share, query, id); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &share, nil
}

func (r *shareRepo) GetByShareCode(ctx context.Context, code string) (*models.Share, error) {
	var share models.Share
	query := `SELECT * FROM shares WHERE share_code = $1 AND is_active = true`
	if err := r.db.GetContext(ctx, &share, query, code); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &share, nil
}

func (r *shareRepo) GetByCustomSlug(ctx context.Context, slug string) (*models.Share, error) {
	var share models.Share
	query := `SELECT * FROM shares WHERE custom_slug = $1 AND is_active = true`
	if err := r.db.GetContext(ctx, &share, query, slug); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &share, nil
}

func (r *shareRepo) ListByClipID(ctx context.Context, clipID uuid.UUID) ([]*models.Share, error) {
	var shares []*models.Share
	query := `SELECT * FROM shares WHERE clip_id = $1 ORDER BY created_at DESC`
	if err := r.db.SelectContext(ctx, &shares, query, clipID); err != nil {
		return nil, err
	}
	return shares, nil
}

func (r *shareRepo) IncrementViewCount(ctx context.Context, id uuid.UUID) error {
	query := `UPDATE shares SET view_count = view_count + 1 WHERE id = $1`
	_, err := r.db.ExecContext(ctx, query, id)
	return err
}

func (r *shareRepo) Deactivate(ctx context.Context, id uuid.UUID) error {
	query := `UPDATE shares SET is_active = false WHERE id = $1`
	_, err := r.db.ExecContext(ctx, query, id)
	return err
}

func (r *shareRepo) Delete(ctx context.Context, id uuid.UUID) error {
	query := `DELETE FROM shares WHERE id = $1`
	_, err := r.db.ExecContext(ctx, query, id)
	return err
}
