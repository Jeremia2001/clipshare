package repository

import (
	"context"
	"database/sql"
	"fmt"

	"clipshare/internal/models"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"
)

type ClipRepository interface {
	Create(ctx context.Context, clip *models.Clip) (*models.Clip, error)
	GetByID(ctx context.Context, id uuid.UUID) (*models.Clip, error)
	ListByUser(ctx context.Context, userID uuid.UUID, limit, offset int) ([]*models.Clip, error)
	CountByUser(ctx context.Context, userID uuid.UUID) (int, error)
	Update(ctx context.Context, clip *models.Clip) error
	Delete(ctx context.Context, id uuid.UUID) error
	IncrementViewCount(ctx context.Context, id uuid.UUID) error
}

type clipRepo struct {
	db *sqlx.DB
}

func NewClipRepository(db *sqlx.DB) ClipRepository {
	return &clipRepo{db: db}
}

func (r *clipRepo) Create(ctx context.Context, clip *models.Clip) (*models.Clip, error) {
	query := `
		INSERT INTO clips (
			user_id, title, description, rustfs_bucket, rustfs_object_key,
			original_filename, file_size_bytes, duration_seconds, width, height,
			fps, bitrate_kbps, thumbnail_key, processed_variant_keys, codec,
			is_public, allow_comments, expires_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10,
			$11, $12, $13, $14, $15, $16, $17, $18
		) RETURNING id, view_count, created_at, updated_at
	`

	err := r.db.QueryRowxContext(ctx, query,
		clip.UserID, clip.Title, clip.Description, clip.RustfsBucket, clip.RustfsObjectKey,
		clip.OriginalFilename, clip.FileSizeBytes, clip.DurationSeconds, clip.Width, clip.Height,
		clip.Fps, clip.BitrateKbps, clip.ThumbnailKey, pq.Array(clip.ProcessedVariantKeys), clip.Codec,
		clip.IsPublic, clip.AllowComments, clip.ExpiresAt,
	).Scan(
		&clip.ID, &clip.ViewCount, &clip.CreatedAt, &clip.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create clip: %w", err)
	}

	return clip, nil
}

func (r *clipRepo) GetByID(ctx context.Context, id uuid.UUID) (*models.Clip, error) {
	var clip models.Clip
	query := `SELECT * FROM clips WHERE id = $1`
	if err := r.db.GetContext(ctx, &clip, query, id); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &clip, nil
}

func (r *clipRepo) ListByUser(ctx context.Context, userID uuid.UUID, limit, offset int) ([]*models.Clip, error) {
	var clips []*models.Clip
	query := `SELECT * FROM clips WHERE user_id = $1 ORDER BY created_at DESC LIMIT $2 OFFSET $3`
	if err := r.db.SelectContext(ctx, &clips, query, userID, limit, offset); err != nil {
		return nil, err
	}
	return clips, nil
}

func (r *clipRepo) CountByUser(ctx context.Context, userID uuid.UUID) (int, error) {
	var count int
	query := `SELECT COUNT(*) FROM clips WHERE user_id = $1`
	if err := r.db.GetContext(ctx, &count, query, userID); err != nil {
		return 0, err
	}
	return count, nil
}

func (r *clipRepo) Update(ctx context.Context, clip *models.Clip) error {
	query := `
		UPDATE clips SET
			title = $2, description = $3, is_public = $4, allow_comments = $5,
			thumbnail_key = $6, trim_start_seconds = $7, trim_end_seconds = $8, updated_at = NOW()
		WHERE id = $1
	`
	_, err := r.db.ExecContext(ctx, query,
		clip.ID, clip.Title, clip.Description, clip.IsPublic, clip.AllowComments, clip.ThumbnailKey,
		clip.TrimStartSeconds, clip.TrimEndSeconds,
	)
	return err
}

func (r *clipRepo) Delete(ctx context.Context, id uuid.UUID) error {
	query := `DELETE FROM clips WHERE id = $1`
	_, err := r.db.ExecContext(ctx, query, id)
	return err
}

func (r *clipRepo) IncrementViewCount(ctx context.Context, id uuid.UUID) error {
	query := `UPDATE clips SET view_count = view_count + 1 WHERE id = $1`
	_, err := r.db.ExecContext(ctx, query, id)
	return err
}
