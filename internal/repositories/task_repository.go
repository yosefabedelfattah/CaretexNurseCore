package repositories

import (
	"context"

	"github.com/caretex/caretexnursing.core/internal/models"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type TaskFilter struct {
	Scope        string // resident | department
	DepartmentID string
	ResidentID   string
	CaregiverID  string
	Status       string // unassigned | assigned | in_progress | done
	Shift        string // morning | afternoon | night
	Priority     string
	Page         int
	PageSize     int
}

type TaskRepository interface {
	List(ctx context.Context, orgID uuid.UUID, f TaskFilter) ([]models.AssignedTask, int64, error)
	FindByID(ctx context.Context, orgID, id uuid.UUID) (*models.AssignedTask, error)
	Create(ctx context.Context, t *models.AssignedTask) error
	Update(ctx context.Context, t *models.AssignedTask) error
	Delete(ctx context.Context, orgID, id uuid.UUID) error
}

type taskRepository struct{ db *gorm.DB }

func NewTaskRepository(db *gorm.DB) TaskRepository { return &taskRepository{db: db} }

func (r *taskRepository) List(ctx context.Context, orgID uuid.UUID, f TaskFilter) ([]models.AssignedTask, int64, error) {
	q := r.db.WithContext(ctx).Where("organization_id = ?", orgID)

	if f.Scope != "" {
		q = q.Where("scope = ?", f.Scope)
	}
	if f.DepartmentID != "" && f.DepartmentID != "all" {
		if id, err := uuid.Parse(f.DepartmentID); err == nil {
			q = q.Where("department_id = ?", id)
		}
	}
	if f.ResidentID != "" {
		if id, err := uuid.Parse(f.ResidentID); err == nil {
			q = q.Where("resident_id = ?", id)
		}
	}
	if f.CaregiverID != "" {
		if id, err := uuid.Parse(f.CaregiverID); err == nil {
			q = q.Where("assigned_caregiver_id = ?", id)
		}
	}
	if f.Status != "" {
		q = q.Where("status = ?", f.Status)
	}
	if f.Shift != "" {
		q = q.Where("shift = ?", f.Shift)
	}
	if f.Priority != "" {
		q = q.Where("priority = ?", f.Priority)
	}

	var total int64
	q.Model(&models.AssignedTask{}).Count(&total)

	pageSize := f.PageSize
	if pageSize < 1 {
		pageSize = 50
	}
	if pageSize > 200 {
		pageSize = 200
	}
	page := f.Page
	if page < 1 {
		page = 1
	}
	offset := (page - 1) * pageSize

	// Sort: urgent first, then by due_at
	q = q.Order(`
		CASE priority
			WHEN 'urgent' THEN 0
			WHEN 'high' THEN 1
			WHEN 'normal' THEN 2
			WHEN 'low' THEN 3
			ELSE 4
		END, due_at ASC
	`)

	var items []models.AssignedTask
	if err := q.Offset(offset).Limit(pageSize).Find(&items).Error; err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func (r *taskRepository) FindByID(ctx context.Context, orgID, id uuid.UUID) (*models.AssignedTask, error) {
	var t models.AssignedTask
	err := r.db.WithContext(ctx).Where("organization_id = ? AND id = ?", orgID, id).First(&t).Error
	return &t, err
}

func (r *taskRepository) Create(ctx context.Context, t *models.AssignedTask) error {
	return r.db.WithContext(ctx).Create(t).Error
}

func (r *taskRepository) Update(ctx context.Context, t *models.AssignedTask) error {
	return r.db.WithContext(ctx).Save(t).Error
}

func (r *taskRepository) Delete(ctx context.Context, orgID, id uuid.UUID) error {
	return r.db.WithContext(ctx).Where("organization_id = ? AND id = ?", orgID, id).Delete(&models.AssignedTask{}).Error
}
