package handlers

import (
	"net/http"

	"github.com/caretex/caretexnursing.core/internal/dto"
	"github.com/caretex/caretexnursing.core/internal/middleware"
	"github.com/caretex/caretexnursing.core/internal/repositories"
	"github.com/caretex/caretexnursing.core/pkg/response"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type CatalogHandler struct {
	repo repositories.CatalogRepository
}

func NewCatalogHandler(r repositories.CatalogRepository) *CatalogHandler {
	return &CatalogHandler{repo: r}
}

func (h *CatalogHandler) ListStatuses(c *gin.Context) {
	claims := middleware.MustClaims(c)
	orgID, _ := uuid.Parse(claims.OrganizationID)
	items, err := h.repo.ListStatuses(c.Request.Context(), orgID)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "internal", "Failed to load statuses")
		return
	}
	out := make([]dto.StatusCatalogResponse, 0, len(items))
	for _, s := range items {
		out = append(out, dto.StatusCatalogResponse{
			ID:     s.ID.String(),
			Code:   s.Code,
			NameHe: s.NameHe,
			NameEn: s.NameEn,
			Active: s.Active,
		})
	}
	response.OK(c, out, "")
}

func (h *CatalogHandler) ListAttributes(c *gin.Context) {
	claims := middleware.MustClaims(c)
	orgID, _ := uuid.Parse(claims.OrganizationID)
	items, err := h.repo.ListAttributes(c.Request.Context(), orgID)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "internal", "Failed to load attributes")
		return
	}
	out := make([]dto.AttributeCatalogResponse, 0, len(items))
	for _, a := range items {
		out = append(out, dto.AttributeCatalogResponse{
			ID:       a.ID.String(),
			Code:     a.Code,
			NameHe:   a.NameHe,
			NameEn:   a.NameEn,
			Category: a.Category,
			Active:   a.Active,
		})
	}
	response.OK(c, out, "")
}
