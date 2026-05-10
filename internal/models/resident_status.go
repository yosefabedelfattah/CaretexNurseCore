package models

import "github.com/google/uuid"

// ResidentStatusCode is the catalog of statuses that can be assigned to a
// resident. The catalog is per-organization so each customer can rename or
// extend the list (e.g. "isolation" / "בידוד") without code changes.
type ResidentStatusCode struct {
	Base
	OrganizationID uuid.UUID `gorm:"type:uuid;index;not null" json:"organization_id"`
	Code           string    `gorm:"size:64;not null" json:"code"`
	NameHe         string    `gorm:"size:120;not null" json:"name_he"`
	NameEn         string    `gorm:"size:120;not null;default:''" json:"name_en"`
	SortOrder      int       `gorm:"not null;default:0" json:"sort_order"`
	Active         bool      `gorm:"not null;default:true" json:"active"`
}

func (ResidentStatusCode) TableName() string { return "ctxnurse_resident_status_codes" }

// ResidentStatusAssignment links a resident to a status code (many-to-many).
type ResidentStatusAssignment struct {
	Base
	ResidentID uuid.UUID `gorm:"type:uuid;index;not null" json:"resident_id"`
	StatusID   uuid.UUID `gorm:"type:uuid;index;not null" json:"status_id"`
}

func (ResidentStatusAssignment) TableName() string { return "ctxnurse_resident_statuses" }
