package services

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"log"
	"net/url"
	"time"

	"clipshare/internal/models"
	"clipshare/internal/repository"
	"clipshare/internal/storage"

	"github.com/google/uuid"
)

type ClipService struct {
	clipRepo  repository.ClipRepository
	shareRepo repository.ShareRepository
	userRepo  repository.UserRepository
	rustfs    *storage.RustFSClient
}

func NewClipService(
	clipRepo repository.ClipRepository,
	shareRepo repository.ShareRepository,
	userRepo repository.UserRepository,
	rustfs *storage.RustFSClient,
) *ClipService {
	return &ClipService{
		clipRepo:  clipRepo,
		shareRepo: shareRepo,
		userRepo:  userRepo,
		rustfs:    rustfs,
	}
}

type CreateClipRequest struct {
	Title            string  `json:"title"`
	Description      *string `json:"description,omitempty"`
	OriginalFilename string  `json:"original_filename"`
	FileSizeBytes    int64   `json:"file_size_bytes"`
	DurationSeconds  int     `json:"duration_seconds"`
	Width            int     `json:"width"`
	Height           int     `json:"height"`
	Fps              *int    `json:"fps,omitempty"`
	BitrateKbps      *int    `json:"bitrate_kbps,omitempty"`
	Codec            *string `json:"codec,omitempty"`
	IsPublic         bool    `json:"is_public"`
	AllowComments    bool    `json:"allow_comments"`
	TrimStartSeconds float64 `json:"trim_start_seconds"`
	TrimEndSeconds   float64 `json:"trim_end_seconds"`
}

type UpdateClipRequest struct {
	Title            *string  `json:"title,omitempty"`
	Description      *string  `json:"description,omitempty"`
	IsPublic         *bool    `json:"is_public,omitempty"`
	AllowComments    *bool    `json:"allow_comments,omitempty"`
	ThumbnailKey     *string  `json:"thumbnail_key,omitempty"`
	TrimStartSeconds *float64 `json:"trim_start_seconds,omitempty"`
	TrimEndSeconds   *float64 `json:"trim_end_seconds,omitempty"`
}

type ClipListResponse struct {
	Clips   []*models.Clip `json:"clips"`
	Total   int            `json:"total"`
	Page    int            `json:"page"`
	PerPage int            `json:"per_page"`
}

func (s *ClipService) CreateClipRecord(ctx context.Context, clip *models.Clip) (*models.Clip, error) {
	return s.clipRepo.Create(ctx, clip)
}

func (s *ClipService) UploadToStorage(ctx context.Context, objectKey string, reader io.Reader, size int64, contentType string) error {
	return s.rustfs.PutObject(ctx, "clips-raw", objectKey, reader, size, contentType)
}

func (s *ClipService) FinalizeUpload(ctx context.Context, clipID, userID uuid.UUID, req CreateClipRequest) (*models.Clip, error) {
	clip, err := s.clipRepo.GetByID(ctx, clipID)
	if err != nil {
		return nil, fmt.Errorf("failed to get clip: %w", err)
	}
	if clip == nil {
		return nil, fmt.Errorf("clip not found")
	}
	if clip.UserID != userID {
		return nil, fmt.Errorf("not authorized to update this clip")
	}

	clip.Title = req.Title
	clip.Description = req.Description
	clip.FileSizeBytes = req.FileSizeBytes
	clip.DurationSeconds = req.DurationSeconds
	clip.Width = req.Width
	clip.Height = req.Height
	clip.Fps = req.Fps
	clip.BitrateKbps = req.BitrateKbps
	clip.Codec = req.Codec
	clip.IsPublic = req.IsPublic
	clip.AllowComments = req.AllowComments
	if req.TrimStartSeconds > 0 {
		clip.TrimStartSeconds = req.TrimStartSeconds
	}
	if req.TrimEndSeconds > 0 {
		clip.TrimEndSeconds = req.TrimEndSeconds
	}

	if err := s.clipRepo.Update(ctx, clip); err != nil {
		return nil, fmt.Errorf("failed to update clip: %w", err)
	}

	if err := s.userRepo.UpdateStorageUsed(ctx, userID, req.FileSizeBytes); err != nil {
		_ = err
	}

	return clip, nil
}

func (s *ClipService) GetClip(ctx context.Context, clipID uuid.UUID) (*models.Clip, error) {
	return s.clipRepo.GetByID(ctx, clipID)
}

func (s *ClipService) GetClipViewURL(ctx context.Context, clip *models.Clip) (string, error) {
	viewURL, err := s.rustfs.GeneratePresignedViewURL(ctx, clip.RustfsBucket, clip.RustfsObjectKey, 1*time.Hour)
	if err != nil {
		return "", fmt.Errorf("failed to generate view URL: %w", err)
	}
	return viewURL.String(), nil
}

func (s *ClipService) StreamClipFile(ctx context.Context, clipID uuid.UUID) (io.ReadCloser, int64, string, error) {
	clip, err := s.clipRepo.GetByID(ctx, clipID)
	if err != nil {
		return nil, 0, "", err
	}
	if clip == nil {
		return nil, 0, "", fmt.Errorf("clip not found")
	}

	log.Printf("[StreamClipFile] Streaming clip %s, bucket=%s, key=%s", clipID, clip.RustfsBucket, clip.RustfsObjectKey)

	info, err := s.rustfs.StatObject(ctx, clip.RustfsBucket, clip.RustfsObjectKey)
	if err != nil {
		log.Printf("[StreamClipFile] StatObject failed: %v", err)
		return nil, 0, "", fmt.Errorf("failed to stat object: %w", err)
	}

	obj, err := s.rustfs.GetObject(ctx, clip.RustfsBucket, clip.RustfsObjectKey)
	if err != nil {
		log.Printf("[StreamClipFile] GetObject failed: %v", err)
		return nil, 0, "", fmt.Errorf("failed to get object: %w", err)
	}

	contentType := "video/mp4"
	if info.ContentType != "" {
		contentType = info.ContentType
	}

	log.Printf("[StreamClipFile] Streaming %d bytes, type=%s", info.Size, contentType)

	return obj, info.Size, contentType, nil
}

func (s *ClipService) ListUserClips(ctx context.Context, userID uuid.UUID, page, perPage int) (*ClipListResponse, error) {
	if perPage <= 0 {
		perPage = 20
	}
	if perPage > 100 {
		perPage = 100
	}
	offset := (page - 1) * perPage

	clips, err := s.clipRepo.ListByUser(ctx, userID, perPage, offset)
	if err != nil {
		return nil, err
	}

	total, err := s.clipRepo.CountByUser(ctx, userID)
	if err != nil {
		return nil, err
	}

	return &ClipListResponse{
		Clips:   clips,
		Total:   total,
		Page:    page,
		PerPage: perPage,
	}, nil
}

func (s *ClipService) UpdateClip(ctx context.Context, clipID, userID uuid.UUID, req UpdateClipRequest) (*models.Clip, error) {
	clip, err := s.clipRepo.GetByID(ctx, clipID)
	if err != nil {
		return nil, err
	}
	if clip == nil {
		return nil, fmt.Errorf("clip not found")
	}
	if clip.UserID != userID {
		return nil, fmt.Errorf("not authorized to update this clip")
	}

	if req.Title != nil {
		clip.Title = *req.Title
	}
	if req.Description != nil {
		clip.Description = req.Description
	}
	if req.IsPublic != nil {
		clip.IsPublic = *req.IsPublic
	}
	if req.AllowComments != nil {
		clip.AllowComments = *req.AllowComments
	}
	if req.ThumbnailKey != nil {
		clip.ThumbnailKey = req.ThumbnailKey
	}
	if req.TrimStartSeconds != nil {
		clip.TrimStartSeconds = *req.TrimStartSeconds
	}
	if req.TrimEndSeconds != nil {
		clip.TrimEndSeconds = *req.TrimEndSeconds
	}

	if err := s.clipRepo.Update(ctx, clip); err != nil {
		return nil, err
	}

	return clip, nil
}

func (s *ClipService) DeleteClip(ctx context.Context, clipID, userID uuid.UUID) error {
	clip, err := s.clipRepo.GetByID(ctx, clipID)
	if err != nil {
		return err
	}
	if clip == nil {
		return fmt.Errorf("clip not found")
	}
	if clip.UserID != userID {
		return fmt.Errorf("not authorized to delete this clip")
	}

	if err := s.rustfs.DeleteObject(ctx, clip.RustfsBucket, clip.RustfsObjectKey); err != nil {
		_ = err
	}

	if err := s.clipRepo.Delete(ctx, clipID); err != nil {
		return err
	}

	_ = s.userRepo.UpdateStorageUsed(ctx, userID, -clip.FileSizeBytes)

	return nil
}

type CreateShareRequest struct {
	ClipID     uuid.UUID  `json:"clip_id"`
	CustomSlug *string    `json:"custom_slug,omitempty"`
	Password   *string    `json:"password,omitempty"`
	ExpiresAt  *time.Time `json:"expires_at,omitempty"`
	MaxViews   *int       `json:"max_views,omitempty"`
}

type ShareResponse struct {
	ShareCode string        `json:"share_code"`
	ShareURL  string        `json:"share_url"`
	Share     *models.Share `json:"share"`
}

func (s *ClipService) CreateShare(ctx context.Context, userID uuid.UUID, req CreateShareRequest) (*ShareResponse, error) {
	clip, err := s.clipRepo.GetByID(ctx, req.ClipID)
	if err != nil {
		return nil, err
	}
	if clip == nil {
		return nil, fmt.Errorf("clip not found")
	}
	if clip.UserID != userID {
		return nil, fmt.Errorf("not authorized to share this clip")
	}

	shareCode := generateShareCode()

	var passwordHash *string
	if req.Password != nil && *req.Password != "" {
		hash := sha256Hash(*req.Password)
		passwordHash = &hash
	}

	share := &models.Share{
		ClipID:       req.ClipID,
		UserID:       userID,
		ShareCode:    shareCode,
		CustomSlug:   req.CustomSlug,
		PasswordHash: passwordHash,
		ExpiresAt:    req.ExpiresAt,
		MaxViews:     req.MaxViews,
	}

	created, err := s.shareRepo.Create(ctx, share)
	if err != nil {
		return nil, fmt.Errorf("failed to create share: %w", err)
	}

	shareURL := buildShareURL(shareCode, req.CustomSlug)

	return &ShareResponse{
		ShareCode: shareCode,
		ShareURL:  shareURL,
		Share:     created,
	}, nil
}

func (s *ClipService) ListClipShares(ctx context.Context, clipID, userID uuid.UUID) ([]*models.Share, error) {
	clip, err := s.clipRepo.GetByID(ctx, clipID)
	if err != nil {
		return nil, err
	}
	if clip == nil {
		return nil, fmt.Errorf("clip not found")
	}
	if clip.UserID != userID {
		return nil, fmt.Errorf("not authorized")
	}

	return s.shareRepo.ListByClipID(ctx, clipID)
}

func (s *ClipService) DeleteShare(ctx context.Context, shareID, userID uuid.UUID) error {
	share, err := s.shareRepo.GetByID(ctx, shareID)
	if err != nil {
		return err
	}
	if share == nil {
		return fmt.Errorf("share not found")
	}
	if share.UserID != userID {
		return fmt.Errorf("not authorized")
	}

	return s.shareRepo.Delete(ctx, shareID)
}

func (s *ClipService) GetSharedClip(ctx context.Context, code string, password *string) (*models.Clip, string, error) {
	var share *models.Share
	var err error

	share, err = s.shareRepo.GetByShareCode(ctx, code)
	if err != nil {
		return nil, "", err
	}
	if share == nil {
		share, err = s.shareRepo.GetByCustomSlug(ctx, code)
		if err != nil {
			return nil, "", err
		}
	}

	if share == nil {
		return nil, "", fmt.Errorf("share not found")
	}

	if !share.IsActive {
		return nil, "", fmt.Errorf("share is no longer active")
	}

	if share.ExpiresAt != nil && time.Now().After(*share.ExpiresAt) {
		_ = s.shareRepo.Deactivate(ctx, share.ID)
		return nil, "", fmt.Errorf("share has expired")
	}

	if share.MaxViews != nil && share.ViewCount >= *share.MaxViews {
		_ = s.shareRepo.Deactivate(ctx, share.ID)
		return nil, "", fmt.Errorf("share has reached maximum views")
	}

	if share.PasswordHash != nil {
		if password == nil || *password == "" {
			return nil, "", fmt.Errorf("password required")
		}
		hash := sha256Hash(*password)
		if hash != *share.PasswordHash {
			return nil, "", fmt.Errorf("incorrect password")
		}
	}

	clip, err := s.clipRepo.GetByID(ctx, share.ClipID)
	if err != nil {
		return nil, "", err
	}
	if clip == nil {
		return nil, "", fmt.Errorf("clip not found")
	}

	_ = s.clipRepo.IncrementViewCount(ctx, clip.ID)
	_ = s.shareRepo.IncrementViewCount(ctx, share.ID)

	viewURL, err := s.GetClipViewURL(ctx, clip)
	if err != nil {
		return nil, "", err
	}

	return clip, viewURL, nil
}

func generateShareCode() string {
	return uuid.New().String()[:8]
}

func buildShareURL(code string, slug *string) string {
	if slug != nil && *slug != "" {
		return fmt.Sprintf("/s/%s", url.PathEscape(*slug))
	}
	return fmt.Sprintf("/s/%s", code)
}

func sha256Hash(s string) string {
	h := sha256.Sum256([]byte(s))
	return fmt.Sprintf("%x", h)
}
