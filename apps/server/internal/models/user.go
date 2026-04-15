package models

import (
	"time"

	"github.com/google/uuid"
)

type User struct {
	ID                uuid.UUID  `db:"id" json:"id"`
	Email             string     `db:"email" json:"email"`
	Username          *string    `db:"username" json:"username,omitempty"`
	DisplayName       *string    `db:"display_name" json:"display_name,omitempty"`
	AvatarURL         *string    `db:"avatar_url" json:"avatar_url,omitempty"`
	IsVerified        bool       `db:"is_verified" json:"is_verified"`
	IsAdmin           bool       `db:"is_admin" json:"is_admin"`
	StorageUsedBytes  int64      `db:"storage_used_bytes" json:"storage_used_bytes"`
	StorageQuotaBytes int64      `db:"storage_quota_bytes" json:"storage_quota_bytes"`
	CustomDomain      *string    `db:"custom_domain" json:"custom_domain,omitempty"`
	DomainVerifiedAt  *time.Time `db:"domain_verified_at" json:"domain_verified_at,omitempty"`
	LastLoginAt       *time.Time `db:"last_login_at" json:"last_login_at,omitempty"`
	CreatedAt         time.Time  `db:"created_at" json:"created_at"`
	UpdatedAt         time.Time  `db:"updated_at" json:"updated_at"`
}

type MagicToken struct {
	ID        uuid.UUID  `db:"id" json:"id"`
	UserID    uuid.UUID  `db:"user_id" json:"user_id"`
	TokenHash string     `db:"token_hash" json:"-"`
	ExpiresAt time.Time  `db:"expires_at" json:"expires_at"`
	UsedAt    *time.Time `db:"used_at" json:"used_at,omitempty"`
	CreatedAt time.Time  `db:"created_at" json:"created_at"`
}

type RefreshToken struct {
	ID        uuid.UUID  `db:"id" json:"id"`
	UserID    uuid.UUID  `db:"user_id" json:"user_id"`
	TokenHash string     `db:"token_hash" json:"-"`
	ExpiresAt time.Time  `db:"expires_at" json:"expires_at"`
	RevokedAt *time.Time `db:"revoked_at" json:"revoked_at,omitempty"`
	CreatedAt time.Time  `db:"created_at" json:"created_at"`
}
