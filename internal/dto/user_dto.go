package dto

// UserResponse is the wire shape returned by GET /api/v1/users.
//
// It deliberately omits PasswordHash, the full Roles[] association, and audit
// fields — caregiver pickers in the UI only need identification, contact, and
// scope info. If a feature later requires more (e.g. roles for permission UIs),
// add a separate, more detailed DTO rather than fattening this one.
type UserResponse struct {
	ID           string  `json:"id"`
	Email        string  `json:"email"`
	FullName     string  `json:"full_name"`
	Role         string  `json:"role,omitempty"`
	Phone        string  `json:"phone,omitempty"`
	PhotoURL     string  `json:"photo_url,omitempty"`
	Status       string  `json:"status,omitempty"`
	DepartmentID *string `json:"department_id,omitempty"`
	CaretxUID    string  `json:"caretx_uid,omitempty"`
}
