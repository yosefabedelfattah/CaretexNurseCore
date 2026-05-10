package dto

import "time"

// ============================================================
// Create / Update payloads
// ============================================================

type CreateResidentRequest struct {
	MRN          string     `json:"mrn" binding:"required,max=64"`
	FirstName    string     `json:"first_name" binding:"required,max=120"`
	LastName     string     `json:"last_name" binding:"required,max=120"`
	DOB          *time.Time `json:"dob,omitempty"`
	Gender       string     `json:"gender" binding:"omitempty,oneof=male female other unknown"`
	RoomNumber   string     `json:"room_number,omitempty" binding:"omitempty,max=32"`
	Phone        string     `json:"phone,omitempty" binding:"omitempty,max=32"`
	Email        string     `json:"email,omitempty" binding:"omitempty,email,max=255"`
	PhotoURL     string     `json:"photo_url,omitempty" binding:"omitempty,max=512"`
	Notes        string     `json:"notes,omitempty"`
	DepartmentID *string    `json:"department_id,omitempty"`
	StatusIDs    []string   `json:"status_ids,omitempty"`
	AttributeIDs []string   `json:"attribute_ids,omitempty"`
}

type UpdateResidentRequest struct {
	MRN          *string    `json:"mrn,omitempty" binding:"omitempty,max=64"`
	FirstName    *string    `json:"first_name,omitempty" binding:"omitempty,max=120"`
	LastName     *string    `json:"last_name,omitempty" binding:"omitempty,max=120"`
	DOB          *time.Time `json:"dob,omitempty"`
	Gender       *string    `json:"gender,omitempty" binding:"omitempty,oneof=male female other unknown"`
	RoomNumber   *string    `json:"room_number,omitempty" binding:"omitempty,max=32"`
	Phone        *string    `json:"phone,omitempty" binding:"omitempty,max=32"`
	Email        *string    `json:"email,omitempty" binding:"omitempty,email,max=255"`
	PhotoURL     *string    `json:"photo_url,omitempty" binding:"omitempty,max=512"`
	Notes        *string    `json:"notes,omitempty"`
	DepartmentID *string    `json:"department_id,omitempty"`
	StatusIDs    *[]string  `json:"status_ids,omitempty"`
	AttributeIDs *[]string  `json:"attribute_ids,omitempty"`
}

// ============================================================
// Response shapes
// ============================================================

type DepartmentRef struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Code string `json:"code"`
}

type StatusRef struct {
	ID     string `json:"id"`
	Code   string `json:"code"`
	NameHe string `json:"name_he"`
	NameEn string `json:"name_en"`
}

type AttributeRef struct {
	ID       string `json:"id"`
	Code     string `json:"code"`
	NameHe   string `json:"name_he"`
	NameEn   string `json:"name_en"`
	Category string `json:"category,omitempty"`
}

type ResidentResponse struct {
	ID               string         `json:"id"`
	OrganizationID   string         `json:"organization_id"`
	Department       *DepartmentRef `json:"department,omitempty"`
	MRN              string         `json:"mrn"`
	FirstName        string         `json:"first_name"`
	LastName         string         `json:"last_name"`
	DOB              *time.Time     `json:"dob,omitempty"`
	Gender           string         `json:"gender,omitempty"`
	RoomNumber       string         `json:"room_number,omitempty"`
	Phone            string         `json:"phone,omitempty"`
	Email            string         `json:"email,omitempty"`
	PhotoURL         string         `json:"photo_url,omitempty"`
	Notes            string         `json:"notes,omitempty"`
	HasTreatmentPlan bool           `json:"has_treatment_plan"`
	CaretxID         string         `json:"caretx_id,omitempty"`
	Statuses         []StatusRef    `json:"statuses"`
	Attributes       []AttributeRef `json:"attributes"`
	CreatedAt        time.Time      `json:"created_at"`
	UpdatedAt        time.Time      `json:"updated_at"`
}

// ============================================================
// List filter query (the heart of the residents page)
// ============================================================

// ResidentListFilter mirrors the URL query parameters accepted by
// GET /api/v1/residents. Boolean tri-states (any / yes / no) are modelled as
// *bool so the caller can distinguish "not specified" from explicit values.
type ResidentListFilter struct {
	// Department selector at the top of the page.
	// If empty, the service falls back to the user's preferred department;
	// pass "all" to skip department filtering entirely.
	DepartmentID string `form:"department_id"`

	// Free-text search over name and MRN.
	Q string `form:"q"`

	// Sort: "last_name" (default) or "first_name", optionally "-" prefixed.
	Sort string `form:"sort"`

	// Status filter: residents with ANY of these status codes (OR-match).
	StatusCodes []string `form:"status_codes"`

	// Room number filter: residents whose room is in this list.
	Rooms []string `form:"rooms"`

	// Attribute filter (tri-state): null=any, true=has at least one attribute
	// assigned, false=has none.
	HasAttributes *bool `form:"has_attributes"`

	// Treatment-plan filter (tri-state): null=any, true=has plan, false=no plan.
	HasTreatmentPlan *bool `form:"has_treatment_plan"`

	Page     int `form:"page"`
	PageSize int `form:"page_size"`
}
