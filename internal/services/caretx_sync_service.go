package services

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/caretex/caretexnursing.core/internal/integrations/caretx"
	"github.com/caretex/caretexnursing.core/internal/models"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	"gorm.io/gorm"
)

// SyncResult holds counters for a sync run.
type SyncResult struct {
	DepartmentsCreated int `json:"departments_created"`
	DepartmentsUpdated int `json:"departments_updated"`
	ResidentsCreated   int `json:"residents_created"`
	ResidentsUpdated   int `json:"residents_updated"`
	ResidentsArchived  int `json:"residents_archived"`
	UsersCreated       int `json:"users_created"`
	UsersUpdated       int `json:"users_updated"`
	Errors             int `json:"errors"`
}

// CaretxSyncService pulls data from the external Caretex platform and upserts
// it into the local database with field mapping.
type CaretxSyncService interface {
	SyncAll(ctx context.Context, orgID uuid.UUID) (*SyncResult, error)
	SyncDepartments(ctx context.Context, orgID uuid.UUID) (*SyncResult, error)
	SyncResidents(ctx context.Context, orgID uuid.UUID) (*SyncResult, error)
	SyncUsers(ctx context.Context, orgID uuid.UUID) (*SyncResult, error)
}

type caretxSyncService struct {
	db     *gorm.DB
	client caretx.Client
}

func NewCaretxSyncService(db *gorm.DB, client caretx.Client) CaretxSyncService {
	return &caretxSyncService{db: db, client: client}
}

// SyncAll runs departments first (so we can resolve department mappings),
// then residents. Starts with a WhoAmI check to verify credentials.
func (s *caretxSyncService) SyncAll(ctx context.Context, orgID uuid.UUID) (*SyncResult, error) {
	result := &SyncResult{}

	// Verify credentials first
	whoami, err := s.client.WhoAmI(ctx)
	if err != nil {
		return nil, fmt.Errorf("credential check failed: %w", err)
	}
	log.Info().Str("tenant", whoami.TenantID).Msg("sync: credential verified")

	depResult, err := s.SyncDepartments(ctx, orgID)
	if err != nil {
		// Departments endpoint may not exist — log and continue
		log.Warn().Err(err).Msg("sync: department sync failed, continuing with residents")
	} else {
		result.DepartmentsCreated = depResult.DepartmentsCreated
		result.DepartmentsUpdated = depResult.DepartmentsUpdated
		result.Errors += depResult.Errors
	}

	resResult, err := s.SyncResidents(ctx, orgID)
	if err != nil {
		return nil, fmt.Errorf("sync residents: %w", err)
	}
	result.ResidentsCreated = resResult.ResidentsCreated
	result.ResidentsUpdated = resResult.ResidentsUpdated
	result.ResidentsArchived = resResult.ResidentsArchived
	result.Errors += resResult.Errors

	// Users are best-effort — older Caretex deployments may not expose /users.
	// A failure here logs but doesn't fail the whole sync.
	if userResult, err := s.SyncUsers(ctx, orgID); err != nil {
		log.Warn().Err(err).Msg("sync: user sync failed, skipping")
	} else {
		result.UsersCreated = userResult.UsersCreated
		result.UsersUpdated = userResult.UsersUpdated
		result.Errors += userResult.Errors
	}

	return result, nil
}

// ─── Department sync ────────────────────────────────────────────────────────

func (s *caretxSyncService) SyncDepartments(ctx context.Context, orgID uuid.UUID) (*SyncResult, error) {
	result := &SyncResult{}

	caretxDepts, err := s.client.FetchDepartments(ctx)
	if err != nil {
		return result, err
	}
	if caretxDepts == nil {
		// /departments endpoint not available — departments will be extracted
		// from residents during SyncResidents
		log.Info().Msg("sync: no departments endpoint, will extract from residents")
		return result, nil
	}

	for _, cd := range caretxDepts {
		if cd.DepartmentName == "" {
			continue
		}

		// Look up by CaretxID (the external UUID stored as a string in our Code field)
		caretxIDStr := cd.ID.String()
		var existing models.Department
		found := s.db.WithContext(ctx).
			Where("organization_id = ? AND code = ?", orgID, caretxIDStr).
			First(&existing).Error

		if found == gorm.ErrRecordNotFound {
			// Create new
			dept := models.Department{
				Base:           models.Base{ID: uuid.New()},
				OrganizationID: orgID,
				Name:           strings.TrimSpace(cd.DepartmentName),
				Code:           caretxIDStr,
				SortOrder:      int(cd.Sequence),
			}
			if err := s.db.WithContext(ctx).Create(&dept).Error; err != nil {
				log.Warn().Err(err).Str("dept_name", cd.DepartmentName).Msg("sync: failed to create department")
				result.Errors++
				continue
			}
			result.DepartmentsCreated++
			log.Debug().Str("name", cd.DepartmentName).Msg("sync: created department")
		} else if found == nil {
			// Update name / sort if changed
			updates := map[string]interface{}{}
			if existing.Name != cd.DepartmentName {
				updates["name"] = strings.TrimSpace(cd.DepartmentName)
			}
			if existing.SortOrder != int(cd.Sequence) {
				updates["sort_order"] = int(cd.Sequence)
			}
			if len(updates) > 0 {
				s.db.WithContext(ctx).Model(&existing).Updates(updates)
				result.DepartmentsUpdated++
			}
		} else {
			log.Warn().Err(found).Msg("sync: department lookup error")
			result.Errors++
		}
	}

	log.Info().
		Int("created", result.DepartmentsCreated).
		Int("updated", result.DepartmentsUpdated).
		Msg("sync: departments complete")
	return result, nil
}

// ─── Resident sync ──────────────────────────────────────────────────────────

func (s *caretxSyncService) SyncResidents(ctx context.Context, orgID uuid.UUID) (*SyncResult, error) {
	result := &SyncResult{}

	persons, err := s.client.FetchPersons(ctx)
	if err != nil {
		return nil, err
	}

	// Auto-create departments from residents if they don't exist yet
	s.ensureDepartmentsFromPersons(ctx, orgID, persons)

	// Pre-load department index: Caretex DepartmentId (UUID string) → our Department.ID
	deptIndex := s.buildDeptIndex(ctx, orgID)

	for _, p := range persons {
		if p.PersonelID == "" {
			continue
		}

		// ─── FIELD MAPPING ────────────────────────────────────────────
		//
		//  Caretex PersonDocument        →  Our models.Resident
		//  ─────────────────────────────────────────────────────────────
		//  PersonelID                    →  MRN (medical record number)
		//  FirstName                     →  FirstName
		//  LastName                      →  LastName
		//  BirthDay                      →  DOB
		//  Gender                        →  Gender (mapped: "male"/"female"/"other")
		//  Phone                         →  Phone
		//  Email                         →  Email
		//  RoomID                        →  RoomNumber
		//  ImagePath                     →  PhotoURL
		//  DepartmentId                  →  DepartmentID (looked up via deptIndex)
		//  PersonelID                    →  CaretxID (external reference)
		//  Notes                         ←  built from Allergy + Language + HMO
		//  IsArchived                    →  soft-delete if true
		//  IsHospitalized                →  status "hospitalized"
		//  IsHazard                      →  status "fall_risk"

		resident := s.mapPersonToResident(p, orgID, deptIndex)

		// Upsert by CaretxID (PersonelID is the stable external key)
		var existing models.Resident
		found := s.db.WithContext(ctx).
			Where("organization_id = ? AND caretx_id = ?", orgID, p.PersonelID).
			First(&existing).Error

		if found == gorm.ErrRecordNotFound {
			// New resident
			resident.ID = uuid.New()
			if err := s.db.WithContext(ctx).Create(&resident).Error; err != nil {
				log.Warn().Err(err).Str("personel_id", p.PersonelID).Msg("sync: failed to create resident")
				result.Errors++
				continue
			}
			result.ResidentsCreated++
		} else if found == nil {
			// Update existing
			resident.ID = existing.ID
			resident.CreatedAt = existing.CreatedAt
			resident.CreatedBy = existing.CreatedBy
			if err := s.db.WithContext(ctx).Save(&resident).Error; err != nil {
				log.Warn().Err(err).Str("personel_id", p.PersonelID).Msg("sync: failed to update resident")
				result.Errors++
				continue
			}
			result.ResidentsUpdated++
		} else {
			log.Warn().Err(found).Msg("sync: resident lookup error")
			result.Errors++
			continue
		}

		// Handle archived → soft-delete
		if p.IsArchived {
			s.db.WithContext(ctx).Delete(&models.Resident{}, "id = ?", resident.ID)
			result.ResidentsArchived++
		}
	}

	log.Info().
		Int("created", result.ResidentsCreated).
		Int("updated", result.ResidentsUpdated).
		Int("archived", result.ResidentsArchived).
		Int("errors", result.Errors).
		Msg("sync: residents complete")
	return result, nil
}

// ─── Field mapping ──────────────────────────────────────────────────────────

func (s *caretxSyncService) mapPersonToResident(
	p caretx.CaretxPerson,
	orgID uuid.UUID,
	deptIndex map[string]uuid.UUID,
) models.Resident {
	r := models.Resident{
		OrganizationID: orgID,
		MRN:            p.PersonelID,
		FirstName:      strings.TrimSpace(p.FirstName),
		LastName:        strings.TrimSpace(p.LastName),
		Gender:         mapGender(p.Gender),
		Phone:          firstNonEmpty(p.Phone, p.Phone2),
		Email:          p.Email,
		PhotoURL:       p.ImagePath,
		CaretxID:       p.PersonelID,
	}

	// DOB
	r.DOB = p.BirthDay.ToTimePtr()

	// Room + Bed: prefer nested objects with readable names, fall back to UUIDs
	if p.Room != nil && p.Room.RoomNumber != "" {
		r.RoomNumber = p.Room.RoomNumber
		if p.Bed != nil && p.Bed.BedNumber != "" {
			r.RoomNumber = p.Room.RoomNumber + " / " + p.Bed.BedNumber
		}
	} else if p.Bed != nil && p.Bed.BedNumber != "" {
		r.RoomNumber = p.Bed.BedNumber
	} else if p.RoomID != nil && *p.RoomID != "" {
		// Fallback: use first 8 chars of UUID as placeholder
		rid := *p.RoomID
		if len(rid) > 8 {
			rid = rid[:8]
		}
		r.RoomNumber = rid
	}

	// Department: use nested Department.DepartmentName for readable name
	caretxDeptID := p.DepartmentId.String()
	if caretxDeptID != uuid.Nil.String() {
		if ourDeptID, ok := deptIndex[caretxDeptID]; ok {
			r.DepartmentID = &ourDeptID
		}
	}

	// Build notes from extra Caretex fields we don't have dedicated columns for
	var notes []string
	if p.HasAllergy && p.Allergy != "" {
		notes = append(notes, "אלרגיה: "+p.Allergy)
	}
	if p.HMO != "" {
		notes = append(notes, "קופ״ח: "+p.HMO)
	}
	if p.Language != "" {
		notes = append(notes, "שפה: "+p.Language)
	}
	if p.GuardianName != "" {
		guardian := "אפוטרופוס: " + p.GuardianName
		if p.GuardianPhone != "" {
			guardian += " (" + p.GuardianPhone + ")"
		}
		notes = append(notes, guardian)
	}
	if p.HospitalName != "" {
		notes = append(notes, "בית חולים: "+p.HospitalName)
	}
	// Family contacts
	for i, f := range p.Family {
		if f.FullName == "" {
			continue
		}
		fam := fmt.Sprintf("איש קשר %d: %s", i+1, f.FullName)
		if f.Phone != "" {
			fam += " (" + f.Phone + ")"
		}
		notes = append(notes, fam)
	}
	if len(notes) > 0 {
		r.Notes = strings.Join(notes, " | ")
	}

	return r
}

// mapGender normalises Caretex gender strings to our convention.
func mapGender(g string) string {
	switch strings.ToLower(strings.TrimSpace(g)) {
	case "male", "m", "זכר":
		return "male"
	case "female", "f", "נקבה":
		return "female"
	default:
		return "other"
	}
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

// buildDeptIndex loads all departments for the org and builds a map from
// Caretex department UUID (stored in our Code field) → our internal department UUID.
func (s *caretxSyncService) buildDeptIndex(ctx context.Context, orgID uuid.UUID) map[string]uuid.UUID {
	var depts []models.Department
	s.db.WithContext(ctx).Where("organization_id = ?", orgID).Find(&depts)

	idx := make(map[string]uuid.UUID, len(depts))
	for _, d := range depts {
		idx[d.Code] = d.ID
	}
	return idx
}

// ensureDepartmentsFromPersons creates departments discovered in resident data
// that don't yet exist in our database. This handles the case where Caretex
// doesn't expose a /departments endpoint.
func (s *caretxSyncService) ensureDepartmentsFromPersons(ctx context.Context, orgID uuid.UUID, persons []caretx.CaretxPerson) {
	existing := s.buildDeptIndex(ctx, orgID)
	seen := make(map[string]bool)

	for _, p := range persons {
		deptID := p.DepartmentId.String()
		if deptID == uuid.Nil.String() || seen[deptID] || existing[deptID] != uuid.Nil {
			continue
		}
		seen[deptID] = true

		// Get department name from nested object, fall back to DepartmentNo
		name := ""
		if p.Department != nil && p.Department.DepartmentName != "" {
			name = strings.TrimSpace(p.Department.DepartmentName)
		} else if p.DepartmentNo != "" {
			name = p.DepartmentNo
		} else {
			name = "מחלקה " + deptID[:8]
		}

		dept := models.Department{
			Base:           models.Base{ID: uuid.New()},
			OrganizationID: orgID,
			Name:           name,
			Code:           deptID,
			SortOrder:      0,
		}
		if err := s.db.WithContext(ctx).Create(&dept).Error; err != nil {
			log.Warn().Err(err).Str("dept_code", deptID).Msg("sync: failed to auto-create department")
			continue
		}
		existing[deptID] = dept.ID
		log.Debug().Str("name", name).Str("code", deptID).Msg("sync: auto-created department from resident")
	}
}

// ─── Statuses from Caretex flags ────────────────────────────────────────────
// TODO: When you need to sync Caretex flags like IsHospitalized, IsHazard,
// IsNusrsingReleased → our ResidentStatusCode m2m, implement it here.
// The approach:
//   1. Load status codes: isolation, hospitalized, fall_risk, etc.
//   2. For each person, build a []uuid.UUID of matching status IDs
//   3. Call repo.ReplaceStatuses(ctx, residentID, statusIDs)
//
// For now the sync creates residents with basic fields only. Status syncing
// can be added incrementally.

// ─── User sync ──────────────────────────────────────────────────────────────
//
// Imports the staff list from Caretx into ctxnurse_users. This is what makes
// "assign caregiver" possible against real, persistent UUIDs (rather than
// mock seed ids that the resident-task FK won't accept).
//
// Strategy:
//   * Upsert key is `caretx_uid` (the Caretex User.UID, a stable string).
//     If that's missing on a row, we fall back to email — case-insensitive —
//     so manually-created admin accounts stay attached to the same row when
//     a Caretex sync arrives later.
//   * Synced users are NEVER created with a usable password. PasswordHash is
//     set to a single byte ("!"), which bcrypt cannot validate — login is
//     impossible until an admin sets a real password through a separate flow.
//     This keeps Caretx-only staff out of the auth surface but visible to
//     pickers.
//   * Status mirrors Caretex implicitly: present in /users → "active". We
//     never auto-deactivate on absence (Caretex paginates inconsistently,
//     and accidentally locking out staff is much worse than a stale row).
func (s *caretxSyncService) SyncUsers(ctx context.Context, orgID uuid.UUID) (*SyncResult, error) {
	result := &SyncResult{}

	users, err := s.client.FetchUsers(ctx)
	if err != nil {
		return nil, err
	}

	for _, u := range users {
		if u.UID == "" && u.Email == "" {
			continue
		}

		mapped := s.mapCaretxUser(u, orgID)

		// Upsert by caretx_uid first; fall back to email for legacy rows.
		var existing models.User
		var found error
		if u.UID != "" {
			found = s.db.WithContext(ctx).
				Where("organization_id = ? AND caretx_uid = ?", orgID, u.UID).
				First(&existing).Error
		} else {
			found = gorm.ErrRecordNotFound
		}
		if errors.Is(found, gorm.ErrRecordNotFound) && u.Email != "" {
			found = s.db.WithContext(ctx).
				Where("organization_id = ? AND LOWER(email) = ?", orgID, strings.ToLower(u.Email)).
				First(&existing).Error
		}

		switch {
		case errors.Is(found, gorm.ErrRecordNotFound):
			mapped.ID = uuid.New()
			// Bcrypt-incompatible placeholder so this user can't authenticate
			// until an admin issues a password reset. NEVER replace this with
			// a usable hash without an explicit, audited UI flow.
			mapped.PasswordHash = "!"
			mapped.Status = "active"
			if err := s.db.WithContext(ctx).Create(&mapped).Error; err != nil {
				log.Warn().Err(err).Str("uid", u.UID).Str("email", u.Email).Msg("sync: failed to create user")
				result.Errors++
				continue
			}
			result.UsersCreated++

		case found == nil:
			// Preserve PasswordHash, ID, audit fields, and any locally-set
			// preferred_department_id — Caretex doesn't carry that concept.
			mapped.ID = existing.ID
			mapped.PasswordHash = existing.PasswordHash
			mapped.CreatedAt = existing.CreatedAt
			mapped.CreatedBy = existing.CreatedBy
			mapped.PreferredDepartmentID = existing.PreferredDepartmentID
			// If Caretex sends an empty role/photo, don't blow away what we have.
			if mapped.Role == "" {
				mapped.Role = existing.Role
			}
			if mapped.PhotoURL == "" {
				mapped.PhotoURL = existing.PhotoURL
			}
			if mapped.Status == "" {
				mapped.Status = existing.Status
			}
			if err := s.db.WithContext(ctx).Save(&mapped).Error; err != nil {
				log.Warn().Err(err).Str("uid", u.UID).Msg("sync: failed to update user")
				result.Errors++
				continue
			}
			result.UsersUpdated++

		default:
			log.Warn().Err(found).Msg("sync: user lookup error")
			result.Errors++
		}
	}

	log.Info().
		Int("created", result.UsersCreated).
		Int("updated", result.UsersUpdated).
		Int("errors", result.Errors).
		Msg("sync: users complete")
	return result, nil
}

// mapCaretxUser projects a Caretex User onto our local User model.
//
// Field mapping:
//   Caretex User           →  Our models.User
//   ─────────────────────────────────────────────────────────────
//   UID                    →  CaretxUID (external upsert key)
//   email                  →  Email
//   displayName            →  FullName (falls back to first+last, then UID)
//   firstName/lastName     →  used to build FullName when displayName empty
//   phone                  →  Phone
//   role                   →  Role  (mapped via mapCaretxRole; see below)
//   photoURL               →  PhotoURL
//   tenantID               →  (not stored — used at the org-resolution layer)
func (s *caretxSyncService) mapCaretxUser(u caretx.CaretxUser, orgID uuid.UUID) models.User {
	full := strings.TrimSpace(u.DisplayName)
	if full == "" {
		full = strings.TrimSpace(u.FirstName + " " + u.LastName)
	}
	if full == "" {
		full = u.UID
	}
	return models.User{
		OrganizationID: orgID,
		Email:          strings.ToLower(strings.TrimSpace(u.Email)),
		FullName:       full,
		Role:           mapCaretxRole(u.Role),
		Phone:          u.Phone,
		PhotoURL:       u.PhotoURL,
		CaretxUID:      u.UID,
	}
}

// mapCaretxRole normalizes Caretex role strings to our four-value vocabulary.
// Anything unknown stays as-is so the data round-trips losslessly — the UI
// just won't show a Hebrew label for unknown roles.
func mapCaretxRole(r string) string {
	switch strings.ToLower(strings.TrimSpace(r)) {
	case "nurse", "rn", "lpn":
		return "nurse"
	case "aide", "caregiver", "assistant":
		return "aide"
	case "physio", "physiotherapist", "pt":
		return "physio"
	case "doctor", "physician", "md":
		return "doctor"
	case "admin", "administrator", "manager":
		return "admin"
	default:
		return strings.ToLower(strings.TrimSpace(r))
	}
}
