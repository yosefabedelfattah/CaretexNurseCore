package models

import (
	"time"

	"github.com/google/uuid"
)

// Resident represents a person residing in a long-term care facility.
// (Replaces the earlier Patient model; the underlying table is renamed to
// ctxnurse_residents.)
type Resident struct {
	Base
	OrganizationID uuid.UUID  `gorm:"type:uuid;index;not null" json:"organization_id"`
	DepartmentID   *uuid.UUID `gorm:"type:uuid;index" json:"department_id,omitempty"`

	MRN        string     `gorm:"size:64;not null;index:idx_ctxnurse_residents_org_mrn,unique" json:"mrn"`
	FirstName  string     `gorm:"size:120;not null" json:"first_name"`
	LastName   string     `gorm:"size:120;not null" json:"last_name"`
	DOB        *time.Time `json:"dob,omitempty"`
	Gender     string     `gorm:"size:16" json:"gender"`
	RoomNumber string     `gorm:"size:64;index" json:"room_number,omitempty"`
	Phone      string     `gorm:"size:32" json:"phone,omitempty"`
	Email      string     `gorm:"size:255" json:"email,omitempty"`
	PhotoURL   string     `gorm:"size:512" json:"photo_url,omitempty"`
	Notes      string     `gorm:"type:text" json:"notes,omitempty"`

	// Denormalised flag: true when at least one row exists in
	// ctxnurse_treatment_plans for this resident. Maintained by the service
	// layer so list filtering stays fast.
	HasTreatmentPlan bool `gorm:"not null;default:false;index" json:"has_treatment_plan"`

	// Caretx external reference
	CaretxID string `gorm:"size:64;index" json:"caretx_id,omitempty"`

	// Eager-loaded relations (only when explicitly preloaded by the repository)
	Department *Department         `gorm:"foreignKey:DepartmentID" json:"department,omitempty"`
	Statuses   []ResidentStatusCode `gorm:"many2many:ctxnurse_resident_statuses;joinForeignKey:ResidentID;joinReferences:StatusID" json:"statuses,omitempty"`
	Attributes []ResidentAttribute  `gorm:"many2many:ctxnurse_resident_attribute_assignments;joinForeignKey:ResidentID;joinReferences:AttributeID" json:"attributes,omitempty"`
}

func (Resident) TableName() string { return "ctxnurse_residents" }
