package models

import "github.com/google/uuid"

// ResidentAttribute is the catalog of attributes a resident may have, e.g.
// "electric dynamic mattress" / "מזרן חשמלי דינאמי", "heel guard" / "מגן עקב".
// Like statuses this is per-organization so customers can extend the list.
type ResidentAttribute struct {
	Base
	OrganizationID uuid.UUID `gorm:"type:uuid;index;not null" json:"organization_id"`
	Code           string    `gorm:"size:64;not null" json:"code"`
	NameHe         string    `gorm:"size:160;not null" json:"name_he"`
	NameEn         string    `gorm:"size:160;not null;default:''" json:"name_en"`
	Category       string    `gorm:"size:64;not null;default:''" json:"category"`
	SortOrder      int       `gorm:"not null;default:0" json:"sort_order"`
	Active         bool      `gorm:"not null;default:true" json:"active"`
}

func (ResidentAttribute) TableName() string { return "ctxnurse_resident_attributes" }

// ResidentAttributeAssignment links a resident to an attribute (many-to-many).
type ResidentAttributeAssignment struct {
	Base
	ResidentID  uuid.UUID `gorm:"type:uuid;index;not null" json:"resident_id"`
	AttributeID uuid.UUID `gorm:"type:uuid;index;not null" json:"attribute_id"`
}

func (ResidentAttributeAssignment) TableName() string { return "ctxnurse_resident_attribute_assignments" }
