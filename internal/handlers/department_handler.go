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

type DepartmentHandler struct {
	repo repositories.DepartmentRepository
}

func NewDepartmentHandler(r repositories.DepartmentRepository) *DepartmentHandler {
	return &DepartmentHandler{repo: r}
}

func (h *DepartmentHandler) List(c *gin.Context) {
	claims := middleware.MustClaims(c)
	orgID, _ := uuid.Parse(claims.OrganizationID)
	items, err := h.repo.List(c.Request.Context(), orgID)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "internal", "Failed to load departments")
		return
	}
	out := make([]dto.DepartmentResponse, 0, len(items))
	for _, d := range items {
		out = append(out, dto.DepartmentResponse{
			ID:        d.ID.String(),
			Name:      d.Name,
			Code:      d.Code,
			SortOrder: d.SortOrder,
			CreatedAt: d.CreatedAt,
		})
	}
	response.OK(c, out, "")
}

func (h *DepartmentHandler) Rooms(c *gin.Context) {
	claims := middleware.MustClaims(c)
	orgID, _ := uuid.Parse(claims.OrganizationID)
	var depID *uuid.UUID
	if v := c.Query("department_id"); v != "" && v != "all" {
		if id, err := uuid.Parse(v); err == nil {
			depID = &id
		}
	}
	rooms, err := h.repo.Rooms(c.Request.Context(), orgID, depID)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "internal", "Failed to load rooms")
		return
	}
	if rooms == nil {
		rooms = []string{}
	}
	response.OK(c, rooms, "")
}
