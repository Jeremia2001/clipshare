package repository

import (
	"context"

	"github.com/jmoiron/sqlx"
)

// InstanceRepository wraps the single-row instance_config table.

type InstanceRepository interface {
	GetStorageLimit(ctx context.Context) (int64, error)
	SetStorageLimit(ctx context.Context, bytes int64) error
	GetTotalStorageUsed(ctx context.Context) (int64, error)
}

type instanceRepo struct{ db *sqlx.DB }

func NewInstanceRepository(db *sqlx.DB) InstanceRepository {
	return &instanceRepo{db: db}
}

func (r *instanceRepo) GetStorageLimit(ctx context.Context) (int64, error) {
	var v int64
	if err := r.db.GetContext(ctx, &v, `SELECT storage_limit_bytes FROM instance_config WHERE id = 1`); err != nil {
		return 0, err
	}
	return v, nil
}

func (r *instanceRepo) SetStorageLimit(ctx context.Context, bytes int64) error {
	if bytes < 0 {
		bytes = 0
	}
	_, err := r.db.ExecContext(ctx,
		`UPDATE instance_config SET storage_limit_bytes = $1, updated_at = NOW() WHERE id = 1`,
		bytes,
	)
	return err
}

// GetTotalStorageUsed sums file_size_bytes across all clips. Single aggregate
// query — cheap enough to call on every upload for the gate check.
func (r *instanceRepo) GetTotalStorageUsed(ctx context.Context) (int64, error) {
	var v int64
	// Cast to BIGINT — Postgres returns SUM of a bigint column as NUMERIC,
	// which sqlx/lib/pq won't scan into int64.
	if err := r.db.GetContext(ctx, &v, `SELECT COALESCE(SUM(file_size_bytes), 0)::BIGINT FROM clips`); err != nil {
		return 0, err
	}
	return v, nil
}
