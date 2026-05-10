package models

import "github.com/google/uuid"

// Department represents a ward / unit (e.g. "Nursing B" / "סיעודית ב").
// In Hebrew long-term-care terminology this is a "מחלקה".
type Department struct {
	Base
	OrganizationID uuid.UUID `gorm:"type:uuid;index;not null" json:"organization_id"`
	Name           string    `gorm:"size:120;not null" json:"name"`
	Code           string    `gorm:"size:64;not null" json:"code"`
	// Optional ordering hint for the UI.
	SortOrder int `gorm:"not null;default:0" json:"sort_order"`
}

func (Department) TableName() string { return "ctxnurse_departments" }
