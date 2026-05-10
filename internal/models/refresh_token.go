package models

import (
	"time"

	"github.com/google/uuid"
)

type RefreshToken struct {
	Base
	UserID    uuid.UUID  `gorm:"type:uuid;index;not null" json:"user_id"`
	TokenHash string     `gorm:"size:128;uniqueIndex;not null" json:"-"`
	ExpiresAt time.Time  `gorm:"index;not null" json:"expires_at"`
	RevokedAt *time.Time `json:"revoked_at,omitempty"`
	UserAgent string     `gorm:"size:255" json:"user_agent,omitempty"`
	IPAddress string     `gorm:"size:64" json:"ip_address,omitempty"`
}

func (RefreshToken) TableName() string { return "ctxnurse_refresh_tokens" }

func (r *RefreshToken) IsActive(now time.Time) bool {
	return r.RevokedAt == nil && now.Before(r.ExpiresAt)
}
