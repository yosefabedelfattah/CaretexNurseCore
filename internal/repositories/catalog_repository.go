package repositories

import (
	"context"

	"github.com/caretex/caretexnursing.core/internal/models"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type CatalogRepository interface {
	ListStatuses(ctx context.Context, orgID uuid.UUID) ([]models.ResidentStatusCode, error)
	ListAttributes(ctx context.Context, orgID uuid.UUID) ([]models.ResidentAttribute, error)
}

type catalogRepository struct{ db *gorm.DB }

func NewCatalogRepository(db *gorm.DB) CatalogRepository {
	return &catalogRepository{db: db}
}

func (r *catalogRepository) ListStatuses(ctx context.Context, orgID uuid.UUID) ([]models.ResidentStatusCode, error) {
	var out []models.ResidentStatusCode
	err := r.db.WithContext(ctx).
		Where("organization_id = ? AND active = TRUE", orgID).
		Order("sort_order ASC, name_he ASC").
		Find(&out).Error
	return out, err
}

func (r *catalogRepository) ListAttributes(ctx context.Context, orgID uuid.UUID) ([]models.ResidentAttribute, error) {
	var out []models.ResidentAttribute
	err := r.db.WithContext(ctx).
		Where("organization_id = ? AND active = TRUE", orgID).
		Order("sort_order ASC, name_he ASC").
		Find(&out).Error
	return out, err
}
