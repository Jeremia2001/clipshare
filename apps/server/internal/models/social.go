package models

import (
	"time"

	"github.com/google/uuid"
)

type Comment struct {
	ID        uuid.UUID  `db:"id" json:"id"`
	ClipID    uuid.UUID  `db:"clip_id" json:"clip_id"`
	UserID    uuid.UUID  `db:"user_id" json:"user_id"`
	ParentID  *uuid.UUID `db:"parent_id" json:"parent_id,omitempty"`
	Content   string     `db:"content" json:"content"`
	IsEdited  bool       `db:"is_edited" json:"is_edited"`
	EditedAt  *time.Time `db:"edited_at" json:"edited_at,omitempty"`
	CreatedAt time.Time  `db:"created_at" json:"created_at"`
}

type ClipReaction struct {
	ID        uuid.UUID `db:"id" json:"id"`
	ClipID    uuid.UUID `db:"clip_id" json:"clip_id"`
	UserID    uuid.UUID `db:"user_id" json:"user_id"`
	Reaction  string    `db:"reaction" json:"reaction"`
	CreatedAt time.Time `db:"created_at" json:"created_at"`
}

type InstanceConfig struct {
	ID                       int       `db:"id" json:"id"`
	InstanceName             string    `db:"instance_name" json:"instance_name"`
	InstanceURL              *string   `db:"instance_url" json:"instance_url,omitempty"`
	AllowSignups             bool      `db:"allow_signups" json:"allow_signups"`
	MaxClipDurationSeconds   int       `db:"max_clip_duration_seconds" json:"max_clip_duration_seconds"`
	DefaultClipLifetimeDays  *int      `db:"default_clip_lifetime_days" json:"default_clip_lifetime_days,omitempty"`
	MaxUploadSizeBytes       int64     `db:"max_upload_size_bytes" json:"max_upload_size_bytes"`
	RequireEmailVerification bool      `db:"require_email_verification" json:"require_email_verification"`
	CustomDomainsEnabled     bool      `db:"custom_domains_enabled" json:"custom_domains_enabled"`
	WildcardDomain           *string   `db:"wildcard_domain" json:"wildcard_domain,omitempty"`
	UpdatedAt                time.Time `db:"updated_at" json:"updated_at"`
}

type CustomDomain struct {
	ID                   uuid.UUID  `db:"id" json:"id"`
	UserID               uuid.UUID  `db:"user_id" json:"user_id"`
	Domain               string     `db:"domain" json:"domain"`
	Status               string     `db:"status" json:"status"`
	VerificationMethod   string     `db:"verification_method" json:"verification_method"`
	DNSRecord            string     `db:"dns_record" json:"dns_record"`
	VerifiedAt           *time.Time `db:"verified_at" json:"verified_at,omitempty"`
	CertificateExpiresAt *time.Time `db:"certificate_expires_at" json:"certificate_expires_at,omitempty"`
	CreatedAt            time.Time  `db:"created_at" json:"created_at"`
}
