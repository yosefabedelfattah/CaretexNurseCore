package repositories

import (
	"context"
	"errors"
	"strings"

	"github.com/caretex/caretexnursing.core/internal/dto"
	"github.com/caretex/caretexnursing.core/internal/models"
	apperr "github.com/caretex/caretexnursing.core/pkg/errors"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type ResidentRepository interface {
	Create(ctx context.Context, r *models.Resident) error
	Update(ctx context.Context, r *models.Resident) error
	Delete(ctx context.Context, orgID, id uuid.UUID) error
	FindByID(ctx context.Context, orgID, id uuid.UUID, withRelations bool) (*models.Resident, error)
	List(ctx context.Context, orgID uuid.UUID, filter dto.ResidentListFilter) ([]models.Resident, int64, error)
	ExistsMRN(ctx context.Context, orgID uuid.UUID, mrn string, excludeID *uuid.UUID) (bool, error)

	// Status / attribute assignment helpers (called by the service in a tx).
	ReplaceStatuses(ctx context.Context, residentID uuid.UUID, statusIDs []uuid.UUID) error
	ReplaceAttributes(ctx context.Context, residentID uuid.UUID, attributeIDs []uuid.UUID) error

	// Treatment-plan flag bookkeeping
	RefreshHasTreatmentPlan(ctx context.Context, residentID uuid.UUID) error
}

type residentRepository struct{ db *gorm.DB }

func NewResidentRepository(db *gorm.DB) ResidentRepository {
	return &residentRepository{db: db}
}

func (r *residentRepository) Create(ctx context.Context, m *models.Resident) error {
	return r.db.WithContext(ctx).Create(m).Error
}

func (r *residentRepository) Update(ctx context.Context, m *models.Resident) error {
	// Use Save so zero-valued fields are persisted (intentional updates).
	return r.db.WithContext(ctx).Save(m).Error
}

func (r *residentRepository) Delete(ctx context.Context, orgID, id uuid.UUID) error {
	res := r.db.WithContext(ctx).
		Where("organization_id = ? AND id = ?", orgID, id).
		Delete(&models.Resident{})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return apperr.ErrNotFound
	}
	return nil
}

func (r *residentRepository) FindByID(ctx context.Context, orgID, id uuid.UUID, withRelations bool) (*models.Resident, error) {
	q := r.db.WithContext(ctx)
	if withRelations {
		q = q.Preload("Department").Preload("Statuses").Preload("Attributes")
	}
	var m models.Resident
	err := q.Where("organization_id = ? AND id = ?", orgID, id).First(&m).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, apperr.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &m, nil
}

func (r *residentRepository) ExistsMRN(ctx context.Context, orgID uuid.UUID, mrn string, excludeID *uuid.UUID) (bool, error) {
	q := r.db.WithContext(ctx).Model(&models.Resident{}).
		Where("organization_id = ? AND mrn = ?", orgID, mrn)
	if excludeID != nil {
		q = q.Where("id <> ?", *excludeID)
	}
	var count int64
	if err := q.Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

// List applies all the filter knobs from the residents page.
//
// Department: equality. If the caller passes a UUID we filter to that
// department. If they pass nothing, the upstream service has already chosen
// either the user's preferred department or "all". The repository just
// honours what it was told.
//
// Status: residents with at least one of the given status codes. We resolve
// status codes (e.g. "isolation") to status IDs and then EXISTS on the
// assignment table. This avoids a JOIN that could duplicate rows when a
// resident has many statuses.
//
// Attributes tri-state: existence sub-query on the assignment table.
//
// Treatment plan tri-state: equality on the denormalised flag.
//
// Search: ILIKE on first/last/MRN; safe because we use bound parameters.
//
// Sort: whitelisted columns only.
func (r *residentRepository) List(ctx context.Context, orgID uuid.UUID, f dto.ResidentListFilter) ([]models.Resident, int64, error) {
	q := r.db.WithContext(ctx).
		Model(&models.Resident{}).
		Where("ctxnurse_residents.organization_id = ?", orgID)

	// --- Department ---
	if f.DepartmentID != "" && f.DepartmentID != "all" {
		if depID, err := uuid.Parse(f.DepartmentID); err == nil {
			q = q.Where("ctxnurse_residents.department_id = ?", depID)
		}
	}

	// --- Search ---
	if s := strings.TrimSpace(f.Q); s != "" {
		like := "%" + s + "%"
		q = q.Where(`(
			ctxnurse_residents.first_name ILIKE ? OR
			ctxnurse_residents.last_name ILIKE ? OR
			ctxnurse_residents.mrn ILIKE ? OR
			ctxnurse_residents.room_number ILIKE ?
		)`, like, like, like, like)
	}

	// --- Room numbers ---
	if len(f.Rooms) > 0 {
		q = q.Where("ctxnurse_residents.room_number IN ?", f.Rooms)
	}

	// --- Status codes (any-of, OR-semantics) ---
	if len(f.StatusCodes) > 0 {
		// Sub-query: resolve codes -> IDs, then EXISTS on the assignment table.
		statusSub := r.db.
			Table("ctxnurse_resident_statuses rs").
			Select("1").
			Joins("JOIN ctxnurse_resident_status_codes sc ON sc.id = rs.status_id").
			Where("rs.resident_id = ctxnurse_residents.id").
			Where("rs.deleted_at IS NULL AND sc.deleted_at IS NULL").
			Where("sc.code IN ?", f.StatusCodes).
			Where("sc.organization_id = ?", orgID)
		q = q.Where("EXISTS (?)", statusSub)
	}

	// --- Has-attributes tri-state ---
	if f.HasAttributes != nil {
		hasAttrSub := r.db.
			Table("ctxnurse_resident_attribute_assignments raa").
			Select("1").
			Where("raa.resident_id = ctxnurse_residents.id").
			Where("raa.deleted_at IS NULL")
		if *f.HasAttributes {
			q = q.Where("EXISTS (?)", hasAttrSub)
		} else {
			q = q.Where("NOT EXISTS (?)", hasAttrSub)
		}
	}

	// --- Has-treatment-plan tri-state ---
	if f.HasTreatmentPlan != nil {
		q = q.Where("ctxnurse_residents.has_treatment_plan = ?", *f.HasTreatmentPlan)
	}

	// --- Total count BEFORE pagination ---
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// --- Sort (whitelisted) ---
	order := "ctxnurse_residents.last_name ASC, ctxnurse_residents.first_name ASC"
	switch f.Sort {
	case "first_name":
		order = "ctxnurse_residents.first_name ASC, ctxnurse_residents.last_name ASC"
	case "-first_name":
		order = "ctxnurse_residents.first_name DESC, ctxnurse_residents.last_name DESC"
	case "last_name", "":
		// already default
	case "-last_name":
		order = "ctxnurse_residents.last_name DESC, ctxnurse_residents.first_name DESC"
	case "room":
		order = "ctxnurse_residents.room_number ASC NULLS LAST, ctxnurse_residents.last_name ASC"
	case "-room":
		order = "ctxnurse_residents.room_number DESC NULLS LAST, ctxnurse_residents.last_name ASC"
	case "mrn":
		order = "ctxnurse_residents.mrn ASC"
	case "-updated_at":
		order = "ctxnurse_residents.updated_at DESC"
	}

	// --- Pagination ---
	page := f.Page
	if page < 1 {
		page = 1
	}
	pageSize := f.PageSize
	if pageSize < 1 {
		pageSize = 25
	}
	if pageSize > 200 {
		pageSize = 200
	}
	offset := (page - 1) * pageSize

	// --- Fetch with relations ---
	var out []models.Resident
	if err := q.
		Preload("Department").
		Preload("Statuses").
		Preload("Attributes").
		Order(order).
		Offset(offset).
		Limit(pageSize).
		Find(&out).Error; err != nil {
		return nil, 0, err
	}
	return out, total, nil
}

func (r *residentRepository) ReplaceStatuses(ctx context.Context, residentID uuid.UUID, statusIDs []uuid.UUID) error {
	tx := r.db.WithContext(ctx)
	if err := tx.Where("resident_id = ?", residentID).Delete(&models.ResidentStatusAssignment{}).Error; err != nil {
		return err
	}
	if len(statusIDs) == 0 {
		return nil
	}
	rows := make([]models.ResidentStatusAssignment, 0, len(statusIDs))
	for _, sid := range statusIDs {
		rows = append(rows, models.ResidentStatusAssignment{
			Base:       models.Base{ID: uuid.New()},
			ResidentID: residentID,
			StatusID:   sid,
		})
	}
	return tx.Create(&rows).Error
}

func (r *residentRepository) ReplaceAttributes(ctx context.Context, residentID uuid.UUID, attributeIDs []uuid.UUID) error {
	tx := r.db.WithContext(ctx)
	if err := tx.Where("resident_id = ?", residentID).Delete(&models.ResidentAttributeAssignment{}).Error; err != nil {
		return err
	}
	if len(attributeIDs) == 0 {
		return nil
	}
	rows := make([]models.ResidentAttributeAssignment, 0, len(attributeIDs))
	for _, aid := range attributeIDs {
		rows = append(rows, models.ResidentAttributeAssignment{
			Base:        models.Base{ID: uuid.New()},
			ResidentID:  residentID,
			AttributeID: aid,
		})
	}
	return tx.Create(&rows).Error
}

// RefreshHasTreatmentPlan recomputes the denormalised flag for one resident.
// Called whenever a treatment plan is created or removed.
func (r *residentRepository) RefreshHasTreatmentPlan(ctx context.Context, residentID uuid.UUID) error {
	var count int64
	if err := r.db.WithContext(ctx).
		Model(&models.TreatmentPlan{}).
		Where("resident_id = ? AND active = TRUE", residentID).
		Count(&count).Error; err != nil {
		return err
	}
	return r.db.WithContext(ctx).
		Model(&models.Resident{}).
		Where("id = ?", residentID).
		Update("has_treatment_plan", count > 0).Error
}
