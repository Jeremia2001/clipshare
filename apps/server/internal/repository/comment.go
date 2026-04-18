package repository

import (
	"context"
	"fmt"

	"clipshare/internal/models"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

type CommentRepository interface {
	Create(ctx context.Context, comment *models.Comment) (*models.Comment, error)
	ListByClipID(ctx context.Context, clipID uuid.UUID) ([]*models.Comment, error)
}

type commentRepo struct {
	db *sqlx.DB
}

func NewCommentRepository(db *sqlx.DB) CommentRepository {
	return &commentRepo{db: db}
}

func (r *commentRepo) Create(ctx context.Context, comment *models.Comment) (*models.Comment, error) {
	query := `
		INSERT INTO comments (clip_id, user_id, parent_id, display_name, content)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, is_edited, created_at
	`
	err := r.db.QueryRowxContext(ctx, query,
		comment.ClipID, comment.UserID, comment.ParentID, comment.DisplayName, comment.Content,
	).Scan(&comment.ID, &comment.IsEdited, &comment.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("failed to create comment: %w", err)
	}
	return comment, nil
}

func (r *commentRepo) ListByClipID(ctx context.Context, clipID uuid.UUID) ([]*models.Comment, error) {
	var comments []*models.Comment
	query := `SELECT * FROM comments WHERE clip_id = $1 ORDER BY created_at ASC`
	if err := r.db.SelectContext(ctx, &comments, query, clipID); err != nil {
		return nil, err
	}
	return comments, nil
}
