package repositories

import (
	"context"

	"github.com/caretex/caretexnursing.core/internal/models"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type DepartmentRepository interface {
	List(ctx context.Context, orgID uuid.UUID) ([]models.Department, error)
	FindByID(ctx context.Context, orgID, id uuid.UUID) (*models.Department, error)
	Rooms(ctx context.Context, orgID uuid.UUID, departmentID *uuid.UUID) ([]string, error)
}

type departmentRepository struct{ db *gorm.DB }

func NewDepartmentRepository(db *gorm.DB) DepartmentRepository {
	return &departmentRepository{db: db}
}

func (r *departmentRepository) List(ctx context.Context, orgID uuid.UUID) ([]models.Department, error) {
	var out []models.Department
	err := r.db.WithContext(ctx).
		Where("organization_id = ?", orgID).
		Order("sort_order ASC, name ASC").
		Find(&out).Error
	return out, err
}

func (r *departmentRepository) FindByID(ctx context.Context, orgID, id uuid.UUID) (*models.Department, error) {
	var d models.Department
	err := r.db.WithContext(ctx).
		Where("organization_id = ? AND id = ?", orgID, id).
		First(&d).Error
	if err != nil {
		return nil, err
	}
	return &d, nil
}

// Rooms returns the distinct, non-empty room numbers used by residents in the
// organisation, optionally constrained to a single department. Used to back the
// "Room number" filter checkboxes.
func (r *departmentRepository) Rooms(ctx context.Context, orgID uuid.UUID, departmentID *uuid.UUID) ([]string, error) {
	q := r.db.WithContext(ctx).
		Model(&models.Resident{}).
		Where("organization_id = ? AND room_number <> ''", orgID)
	if departmentID != nil {
		q = q.Where("department_id = ?", *departmentID)
	}
	var rooms []string
	err := q.Distinct("room_number").Order("room_number ASC").Pluck("room_number", &rooms).Error
	return rooms, err
}
