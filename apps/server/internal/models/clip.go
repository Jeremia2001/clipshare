package models

import (
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"
)

type Clip struct {
	ID                   uuid.UUID      `db:"id" json:"id"`
	UserID               uuid.UUID      `db:"user_id" json:"user_id"`
	Title                string         `db:"title" json:"title"`
	Description          *string        `db:"description" json:"description,omitempty"`
	RustfsBucket         string         `db:"rustfs_bucket" json:"rustfs_bucket"`
	RustfsObjectKey      string         `db:"rustfs_object_key" json:"rustfs_object_key"`
	OriginalFilename     string         `db:"original_filename" json:"original_filename"`
	FileSizeBytes        int64          `db:"file_size_bytes" json:"file_size_bytes"`
	DurationSeconds      int            `db:"duration_seconds" json:"duration_seconds"`
	Width                int            `db:"width" json:"width"`
	Height               int            `db:"height" json:"height"`
	Fps                  *int           `db:"fps" json:"fps,omitempty"`
	BitrateKbps          *int           `db:"bitrate_kbps" json:"bitrate_kbps,omitempty"`
	ThumbnailKey         *string        `db:"thumbnail_key" json:"thumbnail_key,omitempty"`
	ProcessedVariantKeys pq.StringArray `db:"processed_variant_keys" json:"processed_variant_keys,omitempty"`
	Codec                *string        `db:"codec" json:"codec,omitempty"`
	TrimStartSeconds     float64        `db:"trim_start_seconds" json:"trim_start_seconds"`
	TrimEndSeconds       float64        `db:"trim_end_seconds" json:"trim_end_seconds"`
	IsPublic             bool           `db:"is_public" json:"is_public"`
	AllowComments        bool           `db:"allow_comments" json:"allow_comments"`
	ExpiresAt            *time.Time     `db:"expires_at" json:"expires_at,omitempty"`
	ViewCount            int            `db:"view_count" json:"view_count"`
	CreatedAt            time.Time      `db:"created_at" json:"created_at"`
	UpdatedAt            time.Time      `db:"updated_at" json:"updated_at"`
}

type Share struct {
	ID           uuid.UUID  `db:"id" json:"id"`
	ClipID       uuid.UUID  `db:"clip_id" json:"clip_id"`
	UserID       uuid.UUID  `db:"user_id" json:"user_id"`
	ShareCode    string     `db:"share_code" json:"share_code"`
	CustomSlug   *string    `db:"custom_slug" json:"custom_slug,omitempty"`
	PasswordHash *string    `db:"password_hash" json:"-"`
	HasPassword  bool       `db:"-" json:"has_password"`
	ExpiresAt    *time.Time `db:"expires_at" json:"expires_at,omitempty"`
	MaxViews     *int       `db:"max_views" json:"max_views,omitempty"`
	ViewCount    int        `db:"view_count" json:"view_count"`
	IsActive     bool       `db:"is_active" json:"is_active"`
	CreatedAt    time.Time  `db:"created_at" json:"created_at"`
}

type ClipView struct {
	ID             uuid.UUID  `db:"id" json:"id"`
	ClipID         uuid.UUID  `db:"clip_id" json:"clip_id"`
	ShareID        *uuid.UUID `db:"share_id" json:"share_id,omitempty"`
	ViewerUserID   *uuid.UUID `db:"viewer_user_id" json:"viewer_user_id,omitempty"`
	ViewerIP       *string    `db:"viewer_ip" json:"viewer_ip,omitempty"`
	UserAgent      *string    `db:"user_agent" json:"user_agent,omitempty"`
	CountryCode    *string    `db:"country_code" json:"country_code,omitempty"`
	Referrer       *string    `db:"referrer" json:"referrer,omitempty"`
	WatchedSeconds int        `db:"watched_seconds" json:"watched_seconds"`
	CreatedAt      time.Time  `db:"created_at" json:"created_at"`
}
