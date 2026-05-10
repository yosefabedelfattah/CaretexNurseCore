package repositories

import (
	"context"
	"errors"
	"strings"

	"github.com/caretex/caretexnursing.core/internal/models"
	apperr "github.com/caretex/caretexnursing.core/pkg/errors"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type UserRepository interface {
	FindByEmail(ctx context.Context, email string) (*models.User, error)
	FindByID(ctx context.Context, id uuid.UUID) (*models.User, error)
	FindByCaretxUID(ctx context.Context, orgID uuid.UUID, caretxUID string) (*models.User, error)
	GetRoles(ctx context.Context, userID uuid.UUID) ([]models.Role, error)
	UpdateLastLogin(ctx context.Context, id uuid.UUID) error
	List(ctx context.Context, orgID uuid.UUID, f UserFilter) ([]models.User, int64, error)
	Create(ctx context.Context, u *models.User) error
	Save(ctx context.Context, u *models.User) error
}

// UserFilter narrows the result set for List. All fields are optional.
// Q is a case-insensitive substring match against full_name, email, and phone.
type UserFilter struct {
	DepartmentID string // UUID string. Empty/"all" = no filter.
	Role         string // "nurse" | "aide" | "physio" | "doctor" | "admin"; empty = all
	Status       string // "active" | "inactive"; empty = all (excludes soft-deleted regardless)
	Q            string
	Page         int
	PageSize     int
}

type userRepository struct{ db *gorm.DB }

func NewUserRepository(db *gorm.DB) UserRepository { return &userRepository{db: db} }

func (r *userRepository) FindByEmail(ctx context.Context, email string) (*models.User, error) {
	var u models.User
	err := r.db.WithContext(ctx).
		Where("LOWER(email) = ?", strings.ToLower(email)).
		First(&u).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, apperr.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &u, nil
}

func (r *userRepository) FindByID(ctx context.Context, id uuid.UUID) (*models.User, error) {
	var u models.User
	err := r.db.WithContext(ctx).First(&u, "id = ?", id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, apperr.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &u, nil
}

func (r *userRepository) GetRoles(ctx context.Context, userID uuid.UUID) ([]models.Role, error) {
	var roles []models.Role
	err := r.db.WithContext(ctx).
		Joins("JOIN ctxnurse_user_roles ur ON ur.role_id = ctxnurse_roles.id").
		Where("ur.user_id = ? AND ur.deleted_at IS NULL", userID).
		Find(&roles).Error
	return roles, err
}

func (r *userRepository) UpdateLastLogin(ctx context.Context, id uuid.UUID) error {
	return r.db.WithContext(ctx).
		Model(&models.User{}).
		Where("id = ?", id).
		Update("last_login_at", gorm.Expr("now()")).Error
}

// FindByCaretxUID is the upsert-key lookup used by the sync service.
func (r *userRepository) FindByCaretxUID(ctx context.Context, orgID uuid.UUID, caretxUID string) (*models.User, error) {
	if caretxUID == "" {
		return nil, apperr.ErrNotFound
	}
	var u models.User
	err := r.db.WithContext(ctx).
		Where("organization_id = ? AND caretx_uid = ?", orgID, caretxUID).
		First(&u).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, apperr.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &u, nil
}

// List returns users matching the filter, scoped to the org.
//
// Caveat: when DepartmentID is set, we filter by the user's preferred_department_id
// — the local schema has no per-user assignment list yet, only the "preferred"
// pointer. Callers that need cross-department visibility should pass DepartmentID="all".
func (r *userRepository) List(ctx context.Context, orgID uuid.UUID, f UserFilter) ([]models.User, int64, error) {
	q := r.db.WithContext(ctx).
		Model(&models.User{}).
		Where("organization_id = ?", orgID)

	if f.DepartmentID != "" && f.DepartmentID != "all" {
		if dep, err := uuid.Parse(f.DepartmentID); err == nil {
			q = q.Where("preferred_department_id = ?", dep)
		}
	}
	if f.Role != "" {
		q = q.Where("role = ?", f.Role)
	}
	if f.Status != "" {
		q = q.Where("status = ?", f.Status)
	}
	if s := strings.TrimSpace(f.Q); s != "" {
		needle := "%" + strings.ToLower(s) + "%"
		q = q.Where(
			"LOWER(full_name) LIKE ? OR LOWER(email) LIKE ? OR LOWER(phone) LIKE ?",
			needle, needle, needle,
		)
	}

	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	page, pageSize := f.Page, f.PageSize
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 100
	}
	if pageSize > 500 {
		pageSize = 500
	}

	var items []models.User
	err := q.Order("full_name ASC").
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		Find(&items).Error
	if err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

// Create inserts a brand-new user.
func (r *userRepository) Create(ctx context.Context, u *models.User) error {
	return r.db.WithContext(ctx).Create(u).Error
}

// Save persists changes to an existing user (full update).
func (r *userRepository) Save(ctx context.Context, u *models.User) error {
	return r.db.WithContext(ctx).Save(u).Error
}
