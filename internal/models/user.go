package models

import (
	"time"

	"github.com/google/uuid"
)

// User represents a staff member in the nursing system. Users include nurses,
// aides, physiotherapists, and doctors — collectively addressed as "caregivers"
// in scheduling/assignment UIs.
//
// Users may be synchronized from the external Caretx platform via the
// CaretxSyncService. When that's the case, CaretxUID holds the external UID
// (the Caretx User.UID, which is the stable external key — not a UUID, just a
// string). Only Caretx tenants matching this organization are imported.
type User struct {
	Base
	OrganizationID        uuid.UUID  `gorm:"type:uuid;index;not null" json:"organization_id"`
	Email                 string     `gorm:"size:255;uniqueIndex;not null" json:"email"`
	PasswordHash          string     `gorm:"not null" json:"-"`
	FullName              string     `gorm:"size:200;not null" json:"full_name"`
	Status                string     `gorm:"size:32;not null;default:'active'" json:"status"`
	LastLoginAt           *time.Time `json:"last_login_at,omitempty"`
	PreferredDepartmentID *uuid.UUID `gorm:"type:uuid;index" json:"preferred_department_id,omitempty"`

	// ── Caregiver-specific profile (populated for staff users) ──────────────
	// Role is a free-form code: "nurse" | "aide" | "physio" | "doctor" | "admin".
	// We keep the string here because the strict role/permission tuples live
	// in ctxnurse_roles / ctxnurse_user_roles; this field is a lightweight
	// classifier used by UI pickers (e.g. "show only nurses").
	Role     string `gorm:"size:32;index" json:"role,omitempty"`
	Phone    string `gorm:"size:64" json:"phone,omitempty"`
	PhotoURL string `gorm:"size:512" json:"photo_url,omitempty"`

	// ── External identity (Caretx) ──────────────────────────────────────────
	// CaretxUID is the User.UID from the external Caretx platform. It's the
	// stable upsert key during sync, but never exposed to other services as
	// the canonical identifier — internal references always use the local UUID.
	CaretxUID string `gorm:"size:128;index" json:"caretx_uid,omitempty"`

	Organization *Organization `gorm:"foreignKey:OrganizationID" json:"organization,omitempty"`
	Roles        []Role        `gorm:"many2many:ctxnurse_user_roles;" json:"roles,omitempty"`
}

func (User) TableName() string { return "ctxnurse_users" }
