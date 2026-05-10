package caretx

import (
	"strings"
	"time"

	"github.com/google/uuid"
)

// ─── FlexTime handles Caretex's mixed date formats ─────────────────────────
// Caretex returns dates in various formats:
//   "1955-06-10"                    (date only)
//   "2024-03-15T10:30:00Z"         (RFC3339)
//   "2024-03-15T10:30:00.000Z"     (RFC3339 with millis)
//   "2024-03-15T10:30:00+02:00"    (RFC3339 with timezone)
//   ""                              (empty)
//   "0001-01-01T00:00:00Z"         (Go zero time)

type FlexTime struct {
	time.Time
	Valid bool
}

var flexFormats = []string{
	time.RFC3339,
	time.RFC3339Nano,
	"2006-01-02T15:04:05",
	"2006-01-02T15:04:05Z",
	"2006-01-02",
	"2006/01/02",
	"01/02/2006",
	"02-01-2006",
}

func (ft *FlexTime) UnmarshalJSON(b []byte) error {
	s := strings.Trim(string(b), "\"")

	// Handle null, empty, or zero
	if s == "" || s == "null" || s == "0001-01-01T00:00:00Z" || s == "0001-01-01" {
		ft.Time = time.Time{}
		ft.Valid = false
		return nil
	}

	// Try each format
	for _, layout := range flexFormats {
		if t, err := time.Parse(layout, s); err == nil {
			ft.Time = t
			ft.Valid = true
			return nil
		}
	}

	// Last resort: zero time, no error (don't break the whole sync for a date)
	ft.Time = time.Time{}
	ft.Valid = false
	return nil
}

func (ft FlexTime) ToTimePtr() *time.Time {
	if !ft.Valid || ft.Time.IsZero() {
		return nil
	}
	t := ft.Time
	return &t
}

// ─── External DTOs ──────────────────────────────────────────────────────────

type CaretxDepartment struct {
	ID             uuid.UUID `json:"ID"`
	DepartmentName string    `json:"DepartmentName"`
	RealmID        string    `json:"RealmID"`
	OrgID          string    `json:"OrgID"`
	Sequence       int32     `json:"Sequence"`
	IsGlobal       bool      `json:"IsGlobal"`
}

// CaretxRoom mirrors OrgRoom from Caretex
type CaretxRoom struct {
	ID           uuid.UUID  `json:"ID"`
	RoomNumber   string     `json:"room_number"`
	Floor        string     `json:"floor"`
	Wing         string     `json:"wing"`
	DepartmentID string     `json:"department_id"`
	Beds         []CaretxBed `json:"beds,omitempty"`
}

// CaretxBed mirrors OrgBed from Caretex
type CaretxBed struct {
	ID        uuid.UUID `json:"ID"`
	BedNumber string    `json:"bed_number"`
	BedType   string    `json:"bed_type"`
	Status    string    `json:"status"`
	RoomID    uuid.UUID `json:"room_id"`
}

type CaretxPerson struct {
	PersonelID string `json:"PersonelID"`
	RealmID    string `json:"RealmID"`
	OrgID      string `json:"OrgID"`

	// Room / Bed (UUIDs)
	RoomID *string `json:"RoomID"`
	BedID  *string `json:"BedID"`

	// Nested objects (populated by Caretex API)
	Room *CaretxRoom `json:"room,omitempty"`
	Bed  *CaretxBed  `json:"bed,omitempty"`

	// Department (nested object from Caretex)
	DepartmentId uuid.UUID         `json:"DepartmentId"`
	Department   *CaretxDepartment `json:"Department,omitempty"`
	DepartmentNo string            `json:"DepartmentNo"`

	// Identity
	FirstName  string   `json:"FirstName"`
	LastName   string   `json:"LastName"`
	FatherName string   `json:"FatherName"`
	MotherName string   `json:"MotherName"`
	Gender     string   `json:"Gender"`
	BirthDay   FlexTime `json:"BirthDay"`
	ImagePath  string   `json:"ImagePath"`

	// Contact
	Phone  string `json:"Phone"`
	Phone2 string `json:"Phone2"`
	Email  string `json:"Email"`

	// Address
	Address string `json:"Address"`
	City    string `json:"City"`
	Zip     string `json:"Zip"`

	// Admission
	JoinedAt   FlexTime `json:"JoinedAt"`
	ReleasedAt FlexTime `json:"ReleasedAt"`

	// Status
	StatusId              uuid.UUID `json:"StatusId"`
	IsArchived            bool      `json:"IsArchived"`
	IsHospitalized        bool      `json:"IsHospitalized"`
	IsNusrsingReleased    bool      `json:"IsNusrsingReleased"`
	IsNusrsingAdmissioned bool      `json:"IsNusrsingAdmissioned"`
	IsHazard              bool      `json:"IsHazard"`

	// Medical
	HasAllergy   bool   `json:"HasAllergy"`
	Allergy      string `json:"Allergy"`
	HMO          string `json:"HMO"`
	HospitalName string `json:"HospitalName"`

	// Guardian
	Guardian      string `json:"Guardian"`
	GuardianName  string `json:"GuardianName"`
	GuardianPhone string `json:"GuardianPhone"`

	// Misc
	Language       string `json:"Language"`
	Religion       string `json:"Religion"`
	PersonalStatus string `json:"PersonalStatus"`

	// Funder
	FunderId uuid.UUID `json:"FunderId"`

	// Timestamps
	DepartmentAcceptanceDate FlexTime `json:"DepartmentAcceptanceDate"`
	LastUpdated_at           FlexTime `json:"LastUpdated_at"`

	// Family contacts (embedded array from Caretex)
	Family []CaretxFamily `json:"Family"`
}

// CaretxFamily mirrors PaitentFamily from Caretex
type CaretxFamily struct {
	ID                  uuid.UUID `json:"ID"`
	PersonelID          string    `json:"PersonelID"`
	FullName            string    `json:"FullName"`
	Phone               string    `json:"Phone"`
	Phone2              string    `json:"Phone2"`
	Address             string    `json:"Address"`
	LivesWithPatient    bool      `json:"LivesWithPatient"`
	CanBeContacted      bool      `json:"CanBeContacted"`
	OtherFamilyRelation string    `json:"OtherFamilyRelation"`
}

type CaretxNursingStatus struct {
	ID         uuid.UUID `json:"ID"`
	StatusName string    `json:"StatusName"`
	RealmID    string    `json:"RealmID"`
}

// CaretxUser mirrors the User document in the external Caretx platform.
//
// Field tags match Caretx exactly: `UID` (note: not `ID`), `firstName`/`lastName`
// (camelCase), `displayName`, `tenantID`, `role`, `photoURL`. Anything we don't
// strictly need for caregiver assignment is left out — adding more fields here
// is harmless but keeps the JSON decoder doing less work.
//
// Note: `UID` is a string in Caretx (not necessarily a UUID), which is exactly
// why we store it as `caretx_uid` in our local user table rather than using it
// as the primary key. Our local UUID is what the rest of the system references.
type CaretxUser struct {
	UID         string `json:"UID"`
	Email       string `json:"email"`
	DisplayName string `json:"displayName"`
	FirstName   string `json:"firstName"`
	LastName    string `json:"lastName"`
	Phone       string `json:"phone"`
	Role        string `json:"role"`
	PhotoURL    string `json:"photoURL"`
	TenantID    string `json:"tenantID"`
}

