package models

import (
	"time"

	"github.com/google/uuid"
)

type User struct {
	ID                uuid.UUID  `db:"id" json:"id"`
	Username          string     `db:"username" json:"username"`
	DisplayName       *string    `db:"display_name" json:"display_name,omitempty"`
	AvatarURL         *string    `db:"avatar_url" json:"avatar_url,omitempty"`
	IsAdmin           bool       `db:"is_admin" json:"is_admin"`
	PasswordHash      *string    `db:"password_hash" json:"-"`
	StorageUsedBytes  int64      `db:"storage_used_bytes" json:"-"`
	StorageQuotaBytes int64      `db:"storage_quota_bytes" json:"-"`
	CustomDomain      *string    `db:"custom_domain" json:"custom_domain,omitempty"`
	DomainVerifiedAt  *time.Time `db:"domain_verified_at" json:"domain_verified_at,omitempty"`
	LastLoginAt       *time.Time `db:"last_login_at" json:"last_login_at,omitempty"`
	CreatedAt         time.Time  `db:"created_at" json:"created_at"`
	UpdatedAt         time.Time  `db:"updated_at" json:"updated_at"`
}

type InviteCode struct {
	ID          uuid.UUID  `db:"id" json:"id"`
	CodeHash    string     `db:"code_hash" json:"-"`
	CreatedBy   *uuid.UUID `db:"created_by" json:"created_by,omitempty"`
	Note        *string    `db:"note" json:"note,omitempty"`
	RedeemedBy  *uuid.UUID `db:"redeemed_by" json:"redeemed_by,omitempty"`
	RedeemedAt  *time.Time `db:"redeemed_at" json:"redeemed_at,omitempty"`
	ExpiresAt   *time.Time `db:"expires_at" json:"expires_at,omitempty"`
	CreatedAt   time.Time  `db:"created_at" json:"created_at"`
}

type DeviceToken struct {
	ID          uuid.UUID  `db:"id" json:"id"`
	UserID      uuid.UUID  `db:"user_id" json:"user_id"`
	TokenHash   string     `db:"token_hash" json:"-"`
	DeviceLabel *string    `db:"device_label" json:"device_label,omitempty"`
	LastSeenAt  *time.Time `db:"last_seen_at" json:"last_seen_at,omitempty"`
	CreatedAt   time.Time  `db:"created_at" json:"created_at"`
}

