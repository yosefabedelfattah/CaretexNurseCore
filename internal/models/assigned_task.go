package models

import (
	"time"

	"github.com/google/uuid"
)

// AssignedTask represents a task assigned to a caregiver — either for a
// specific resident (scope=resident) or for the department (scope=department).
// Tasks are created by nurses/admins and progressed by caregivers.
type AssignedTask struct {
	Base
	OrganizationID uuid.UUID  `gorm:"type:uuid;index;not null" json:"organization_id"`
	Scope          string     `gorm:"size:16;not null;default:'resident'" json:"scope"` // resident | department
	ActionCode     string     `gorm:"size:64" json:"action_code"`
	ActionNameHe   string     `gorm:"size:200;not null" json:"action_name_he"`
	ActionNameEn   string     `gorm:"size:200" json:"action_name_en"`
	ActionIcon     string     `gorm:"size:64;not null;default:'task_alt'" json:"action_icon"`

	// Resident (for scope=resident)
	ResidentID   *uuid.UUID `gorm:"type:uuid;index" json:"resident_id,omitempty"`
	ResidentName string     `gorm:"size:200" json:"resident_name"`
	ResidentMRN  string     `gorm:"size:64" json:"resident_mrn"`

	// Department
	DepartmentID   *uuid.UUID `gorm:"type:uuid;index" json:"department_id,omitempty"`
	DepartmentName string     `gorm:"size:200" json:"department_name"`

	// Assignment
	AssignedCaregiverID   *uuid.UUID `gorm:"type:uuid;index" json:"assigned_caregiver_id,omitempty"`
	AssignedCaregiverName string     `gorm:"size:200" json:"assigned_caregiver_name"`

	// Status progression: unassigned → assigned → in_progress → done
	Status    string `gorm:"size:16;not null;default:'unassigned';index" json:"status"`
	Priority  string `gorm:"size:16;not null;default:'normal'" json:"priority"` // low | normal | high | urgent
	Shift     string `gorm:"size:16;not null;default:'morning'" json:"shift"`   // morning | afternoon | night

	DueAt       time.Time  `gorm:"not null" json:"due_at"`
	StartedAt   *time.Time `json:"started_at,omitempty"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`

	Notes         string `gorm:"type:text" json:"notes"`
	RequiresPhoto bool   `gorm:"not null;default:false" json:"requires_photo"`
}

func (AssignedTask) TableName() string { return "ctxnurse_assigned_tasks" }
