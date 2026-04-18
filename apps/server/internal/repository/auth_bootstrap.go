package repository

import (
	"context"
	"database/sql"
	"time"

	"clipshare/internal/models"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

// InviteCodeRepository

type InviteCodeRepository interface {
	Create(ctx context.Context, codeHash string, createdBy uuid.UUID, note *string, expiresAt *time.Time) (*models.InviteCode, error)
	GetByHash(ctx context.Context, hash string) (*models.InviteCode, error)
	MarkRedeemed(ctx context.Context, id, userID uuid.UUID) error
	List(ctx context.Context) ([]*models.InviteCode, error)
	Delete(ctx context.Context, id uuid.UUID) error
}

type inviteCodeRepo struct{ db *sqlx.DB }

func NewInviteCodeRepository(db *sqlx.DB) InviteCodeRepository {
	return &inviteCodeRepo{db: db}
}

func (r *inviteCodeRepo) Create(ctx context.Context, codeHash string, createdBy uuid.UUID, note *string, expiresAt *time.Time) (*models.InviteCode, error) {
	var inv models.InviteCode
	query := `
		INSERT INTO invite_codes (code_hash, created_by, note, expires_at)
		VALUES ($1, $2, $3, $4)
		RETURNING id, code_hash, created_by, note, redeemed_by, redeemed_at, expires_at, created_at
	`
	if err := r.db.GetContext(ctx, &inv, query, codeHash, createdBy, note, expiresAt); err != nil {
		return nil, err
	}
	return &inv, nil
}

func (r *inviteCodeRepo) GetByHash(ctx context.Context, hash string) (*models.InviteCode, error) {
	var inv models.InviteCode
	query := `SELECT * FROM invite_codes WHERE code_hash = $1`
	if err := r.db.GetContext(ctx, &inv, query, hash); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &inv, nil
}

func (r *inviteCodeRepo) MarkRedeemed(ctx context.Context, id, userID uuid.UUID) error {
	// Only transitions unredeemed → redeemed atomically so concurrent attempts can't double-spend.
	res, err := r.db.ExecContext(ctx,
		`UPDATE invite_codes SET redeemed_by = $2, redeemed_at = NOW() WHERE id = $1 AND redeemed_at IS NULL`,
		id, userID,
	)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (r *inviteCodeRepo) List(ctx context.Context) ([]*models.InviteCode, error) {
	var rows []*models.InviteCode
	if err := r.db.SelectContext(ctx, &rows, `SELECT * FROM invite_codes ORDER BY created_at DESC`); err != nil {
		return nil, err
	}
	return rows, nil
}

func (r *inviteCodeRepo) Delete(ctx context.Context, id uuid.UUID) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM invite_codes WHERE id = $1`, id)
	return err
}

// DeviceTokenRepository

type DeviceTokenRepository interface {
	Create(ctx context.Context, userID uuid.UUID, tokenHash string, label *string) (*models.DeviceToken, error)
	GetByHash(ctx context.Context, hash string) (*models.DeviceToken, error)
	GetByUserID(ctx context.Context, userID uuid.UUID) (*models.DeviceToken, error)
	Touch(ctx context.Context, id uuid.UUID) error
	DeleteByUserID(ctx context.Context, userID uuid.UUID) error
}

type deviceTokenRepo struct{ db *sqlx.DB }

func NewDeviceTokenRepository(db *sqlx.DB) DeviceTokenRepository {
	return &deviceTokenRepo{db: db}
}

func (r *deviceTokenRepo) Create(ctx context.Context, userID uuid.UUID, tokenHash string, label *string) (*models.DeviceToken, error) {
	var tok models.DeviceToken
	query := `
		INSERT INTO device_tokens (user_id, token_hash, device_label)
		VALUES ($1, $2, $3)
		RETURNING id, user_id, token_hash, device_label, last_seen_at, created_at
	`
	if err := r.db.GetContext(ctx, &tok, query, userID, tokenHash, label); err != nil {
		return nil, err
	}
	return &tok, nil
}

func (r *deviceTokenRepo) GetByHash(ctx context.Context, hash string) (*models.DeviceToken, error) {
	var tok models.DeviceToken
	if err := r.db.GetContext(ctx, &tok, `SELECT * FROM device_tokens WHERE token_hash = $1`, hash); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &tok, nil
}

func (r *deviceTokenRepo) GetByUserID(ctx context.Context, userID uuid.UUID) (*models.DeviceToken, error) {
	var tok models.DeviceToken
	if err := r.db.GetContext(ctx, &tok, `SELECT * FROM device_tokens WHERE user_id = $1`, userID); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &tok, nil
}

func (r *deviceTokenRepo) Touch(ctx context.Context, id uuid.UUID) error {
	_, err := r.db.ExecContext(ctx, `UPDATE device_tokens SET last_seen_at = NOW() WHERE id = $1`, id)
	return err
}

func (r *deviceTokenRepo) DeleteByUserID(ctx context.Context, userID uuid.UUID) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM device_tokens WHERE user_id = $1`, userID)
	return err
}

// SetupTokenRepository — the one-time bootstrap token printed on first launch.

type SetupTokenRepository interface {
	Create(ctx context.Context, tokenHash string) error
	ConsumeByHash(ctx context.Context, hash string) (bool, error)
	AnyUnused(ctx context.Context) (bool, error)
	DeleteAll(ctx context.Context) error
}

type setupTokenRepo struct{ db *sqlx.DB }

func NewSetupTokenRepository(db *sqlx.DB) SetupTokenRepository {
	return &setupTokenRepo{db: db}
}

func (r *setupTokenRepo) Create(ctx context.Context, tokenHash string) error {
	_, err := r.db.ExecContext(ctx, `INSERT INTO setup_tokens (token_hash) VALUES ($1)`, tokenHash)
	return err
}

func (r *setupTokenRepo) ConsumeByHash(ctx context.Context, hash string) (bool, error) {
	res, err := r.db.ExecContext(ctx,
		`UPDATE setup_tokens SET used_at = NOW() WHERE token_hash = $1 AND used_at IS NULL`,
		hash,
	)
	if err != nil {
		return false, err
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}

func (r *setupTokenRepo) AnyUnused(ctx context.Context) (bool, error) {
	var exists bool
	if err := r.db.GetContext(ctx, &exists, `SELECT EXISTS(SELECT 1 FROM setup_tokens WHERE used_at IS NULL)`); err != nil {
		return false, err
	}
	return exists, nil
}

func (r *setupTokenRepo) DeleteAll(ctx context.Context) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM setup_tokens`)
	return err
}
