package models

import "github.com/google/uuid"

// TreatmentPlan is a stub for the resident's care plan. The real shape will
// be fleshed out when the treatment-plan feature lands; for now we only need
// enough columns to back the "has plan / no plan" filter on the residents page
// and to drive the denormalised Resident.HasTreatmentPlan flag.
type TreatmentPlan struct {
	Base
	OrganizationID uuid.UUID `gorm:"type:uuid;index;not null" json:"organization_id"`
	ResidentID     uuid.UUID `gorm:"type:uuid;index;not null" json:"resident_id"`
	Title          string    `gorm:"size:200;not null" json:"title"`
	Notes          string    `gorm:"type:text" json:"notes,omitempty"`
	Active         bool      `gorm:"not null;default:true" json:"active"`
}

func (TreatmentPlan) TableName() string { return "ctxnurse_treatment_plans" }
