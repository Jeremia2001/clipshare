package services

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"log"
	"net/url"
	"path/filepath"
	"strings"
	"time"

	"clipshare/internal/models"
	"clipshare/internal/repository"
	"clipshare/internal/storage"

	"github.com/google/uuid"
)

type ClipService struct {
	clipRepo    repository.ClipRepository
	shareRepo   repository.ShareRepository
	userRepo    repository.UserRepository
	commentRepo repository.CommentRepository
	rustfs      *storage.RustFSClient
	publicURL   string
}

func NewClipService(
	clipRepo repository.ClipRepository,
	shareRepo repository.ShareRepository,
	userRepo repository.UserRepository,
	commentRepo repository.CommentRepository,
	rustfs *storage.RustFSClient,
	publicURL string,
) *ClipService {
	return &ClipService{
		clipRepo:    clipRepo,
		shareRepo:   shareRepo,
		userRepo:    userRepo,
		commentRepo: commentRepo,
		rustfs:      rustfs,
		publicURL:   publicURL,
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

// ClipContentType derives the MIME type from the original filename, defaulting
// to video/mp4. This avoids a StatObject round-trip to MinIO.
func ClipContentType(filename string) string {
	switch strings.ToLower(filepath.Ext(filename)) {
	case ".webm":
		return "video/webm"
	case ".mov":
		return "video/quicktime"
	case ".mkv":
		return "video/x-matroska"
	case ".avi":
		return "video/x-msvideo"
	default:
		return "video/mp4"
	}
}

// StreamClipFile opens a reader for the full clip object.
// It uses FileSizeBytes from the database record so no StatObject round-trip is needed.
func (s *ClipService) StreamClipFile(ctx context.Context, clip *models.Clip) (io.ReadCloser, int64, string, error) {
	log.Printf("[StreamClipFile] Streaming clip %s, bucket=%s, key=%s", clip.ID, clip.RustfsBucket, clip.RustfsObjectKey)

	obj, err := s.rustfs.GetObject(ctx, clip.RustfsBucket, clip.RustfsObjectKey)
	if err != nil {
		log.Printf("[StreamClipFile] GetObject failed: %v", err)
		return nil, 0, "", fmt.Errorf("failed to get object: %w", err)
	}

	contentType := ClipContentType(clip.OriginalFilename)
	log.Printf("[StreamClipFile] Streaming %d bytes, type=%s", clip.FileSizeBytes, contentType)
	return obj, clip.FileSizeBytes, contentType, nil
}

// StreamClipFileRange opens a reader that returns only bytes [start, end] (both
// inclusive) of the clip object. It uses MinIO's native range request so the
// server never downloads the full file when serving a partial-content response.
func (s *ClipService) StreamClipFileRange(ctx context.Context, clip *models.Clip, start, end int64) (io.ReadCloser, error) {
	log.Printf("[StreamClipFileRange] Streaming clip %s range %d-%d", clip.ID, start, end)

	obj, err := s.rustfs.GetObjectRange(ctx, clip.RustfsBucket, clip.RustfsObjectKey, start, end)
	if err != nil {
		log.Printf("[StreamClipFileRange] GetObjectRange failed: %v", err)
		return nil, fmt.Errorf("failed to get object range: %w", err)
	}
	return obj, nil
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

func (s *ClipService) UploadThumbnail(ctx context.Context, clipID, userID uuid.UUID, reader io.Reader, size int64, contentType string) error {
	clip, err := s.clipRepo.GetByID(ctx, clipID)
	if err != nil {
		return fmt.Errorf("failed to get clip: %w", err)
	}
	if clip == nil {
		return fmt.Errorf("clip not found")
	}
	if clip.UserID != userID {
		return fmt.Errorf("not authorized")
	}

	_, thumbnailBucket, _ := s.rustfs.BucketNames()
	objectKey := fmt.Sprintf("thumbnails/%s.jpg", clipID.String())

	if err := s.rustfs.PutObject(ctx, thumbnailBucket, objectKey, reader, size, contentType); err != nil {
		return fmt.Errorf("failed to store thumbnail: %w", err)
	}

	clip.ThumbnailKey = &objectKey
	if err := s.clipRepo.Update(ctx, clip); err != nil {
		return fmt.Errorf("failed to update clip: %w", err)
	}

	return nil
}

func (s *ClipService) StreamThumbnail(ctx context.Context, clipID uuid.UUID) (io.ReadCloser, int64, string, error) {
	clip, err := s.clipRepo.GetByID(ctx, clipID)
	if err != nil || clip == nil {
		return nil, 0, "", fmt.Errorf("clip not found")
	}
	if clip.ThumbnailKey == nil || *clip.ThumbnailKey == "" {
		return nil, 0, "", fmt.Errorf("no thumbnail")
	}

	_, thumbnailBucket, _ := s.rustfs.BucketNames()
	info, err := s.rustfs.StatObject(ctx, thumbnailBucket, *clip.ThumbnailKey)
	if err != nil {
		return nil, 0, "", fmt.Errorf("thumbnail not found: %w", err)
	}
	obj, err := s.rustfs.GetObject(ctx, thumbnailBucket, *clip.ThumbnailKey)
	if err != nil {
		return nil, 0, "", fmt.Errorf("failed to get thumbnail: %w", err)
	}

	contentType := info.ContentType
	if contentType == "" {
		contentType = "image/jpeg"
	}
	return obj, info.Size, contentType, nil
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

	shareURL := s.buildShareURL(shareCode, req.CustomSlug)

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

func (s *ClipService) ValidateShareAccess(ctx context.Context, code string, password *string) (*models.Share, *models.Clip, error) {
	share, err := s.shareRepo.GetByShareCode(ctx, code)
	if err != nil {
		return nil, nil, err
	}
	if share == nil {
		share, err = s.shareRepo.GetByCustomSlug(ctx, code)
		if err != nil {
			return nil, nil, err
		}
	}

	if share == nil {
		return nil, nil, fmt.Errorf("share not found")
	}

	if !share.IsActive {
		return nil, nil, fmt.Errorf("share is no longer active")
	}

	if share.ExpiresAt != nil && time.Now().After(*share.ExpiresAt) {
		_ = s.shareRepo.Deactivate(ctx, share.ID)
		return nil, nil, fmt.Errorf("share has expired")
	}

	if share.MaxViews != nil && share.ViewCount >= *share.MaxViews {
		_ = s.shareRepo.Deactivate(ctx, share.ID)
		return nil, nil, fmt.Errorf("share has reached maximum views")
	}

	if share.PasswordHash != nil {
		if password == nil || *password == "" {
			return nil, nil, fmt.Errorf("password required")
		}
		hash := sha256Hash(*password)
		if hash != *share.PasswordHash {
			return nil, nil, fmt.Errorf("incorrect password")
		}
	}

	clip, err := s.clipRepo.GetByID(ctx, share.ClipID)
	if err != nil {
		return nil, nil, err
	}
	if clip == nil {
		return nil, nil, fmt.Errorf("clip not found")
	}

	return share, clip, nil
}

func (s *ClipService) IncrementShareViewCounts(ctx context.Context, clipID, shareID uuid.UUID) {
	_ = s.clipRepo.IncrementViewCount(ctx, clipID)
	_ = s.shareRepo.IncrementViewCount(ctx, shareID)
}

func (s *ClipService) GetSharedClip(ctx context.Context, code string, password *string) (*models.Clip, error) {
	share, clip, err := s.ValidateShareAccess(ctx, code, password)
	if err != nil {
		return nil, err
	}

	_ = s.clipRepo.IncrementViewCount(ctx, clip.ID)
	_ = s.shareRepo.IncrementViewCount(ctx, share.ID)

	return clip, nil
}

const (
	maxCommentNameLength    = 64
	maxCommentContentLength = 2000
)

func (s *ClipService) ListClipCommentsForOwner(ctx context.Context, clipID, userID uuid.UUID) ([]*models.Comment, error) {
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
	return s.commentRepo.ListByClipID(ctx, clipID)
}

func (s *ClipService) ListShareComments(ctx context.Context, code string, password *string) ([]*models.Comment, error) {
	_, clip, err := s.ValidateShareAccess(ctx, code, password)
	if err != nil {
		return nil, err
	}
	if !clip.AllowComments {
		return []*models.Comment{}, nil
	}
	return s.commentRepo.ListByClipID(ctx, clip.ID)
}

func (s *ClipService) CreateGuestComment(ctx context.Context, code string, password *string, displayName, content string) (*models.Comment, error) {
	_, clip, err := s.ValidateShareAccess(ctx, code, password)
	if err != nil {
		return nil, err
	}
	if !clip.AllowComments {
		return nil, fmt.Errorf("comments are disabled for this clip")
	}

	name := strings.TrimSpace(displayName)
	body := strings.TrimSpace(content)
	if name == "" {
		return nil, fmt.Errorf("name is required")
	}
	if body == "" {
		return nil, fmt.Errorf("comment is required")
	}
	if len([]rune(name)) > maxCommentNameLength {
		name = string([]rune(name)[:maxCommentNameLength])
	}
	if len([]rune(body)) > maxCommentContentLength {
		body = string([]rune(body)[:maxCommentContentLength])
	}

	comment := &models.Comment{
		ClipID:      clip.ID,
		DisplayName: &name,
		Content:     body,
	}
	return s.commentRepo.Create(ctx, comment)
}

func generateShareCode() string {
	return uuid.New().String()[:8]
}

func (s *ClipService) ShareURL(shareCode string, customSlug *string) string {
	return s.buildShareURL(shareCode, customSlug)
}

func (s *ClipService) buildShareURL(code string, slug *string) string {
	path := fmt.Sprintf("/api/v1/s/%s", code)
	if slug != nil && *slug != "" {
		path = fmt.Sprintf("/api/v1/s/%s", url.PathEscape(*slug))
	}
	return strings.TrimRight(s.publicURL, "/") + path
}

func sha256Hash(s string) string {
	h := sha256.Sum256([]byte(s))
	return fmt.Sprintf("%x", h)
}
