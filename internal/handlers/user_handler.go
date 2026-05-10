package handlers

import (
	"net/http"
	"strconv"

	"github.com/caretex/caretexnursing.core/internal/dto"
	"github.com/caretex/caretexnursing.core/internal/middleware"
	"github.com/caretex/caretexnursing.core/internal/repositories"
	"github.com/caretex/caretexnursing.core/pkg/response"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// UserHandler exposes the staff list for caregiver pickers in the UI.
//
// Scope note: this is intentionally read-only for now. User creation and
// editing happen via the Caretx sync (and an admin password-reset flow that
// lives outside this handler). If we later need self-service profile edits,
// add them here behind a `users:write` permission.
type UserHandler struct {
	repo repositories.UserRepository
}

func NewUserHandler(repo repositories.UserRepository) *UserHandler {
	return &UserHandler{repo: repo}
}

// List returns staff users matching optional filters.
//
// GET /api/v1/users
//   ?department_id=<uuid>|all   default = "all" (cross-department visible)
//   ?role=nurse|aide|physio|doctor|admin
//   ?status=active|inactive     default = active (only active staff)
//   ?q=<substring>              matches full_name, email, phone (case-insensitive)
//   ?page=1&page_size=100       page_size capped to 500 server-side
//
// Returns the trimmed dto.UserResponse — never the password hash, never the
// full role/permission expansion. The picker doesn't need them and exposing
// them widens the auth blast radius for nothing.
func (h *UserHandler) List(c *gin.Context) {
	claims := middleware.MustClaims(c)
	orgID, _ := uuid.Parse(claims.OrganizationID)

	f := repositories.UserFilter{
		DepartmentID: c.Query("department_id"),
		Role:         c.Query("role"),
		Status:       c.Query("status"),
		Q:            c.Query("q"),
	}
	// Default: only show active staff. Callers who want everyone pass status=all.
	if f.Status == "" {
		f.Status = "active"
	} else if f.Status == "all" {
		f.Status = ""
	}
	if v, _ := strconv.Atoi(c.Query("page")); v > 0 {
		f.Page = v
	}
	if v, _ := strconv.Atoi(c.Query("page_size")); v > 0 {
		f.PageSize = v
	}

	items, total, err := h.repo.List(c.Request.Context(), orgID, f)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "internal", "Failed to list users")
		return
	}

	out := make([]dto.UserResponse, 0, len(items))
	for _, u := range items {
		ur := dto.UserResponse{
			ID:        u.ID.String(),
			Email:     u.Email,
			FullName:  u.FullName,
			Role:      u.Role,
			Phone:     u.Phone,
			PhotoURL:  u.PhotoURL,
			Status:    u.Status,
			CaretxUID: u.CaretxUID,
		}
		if u.PreferredDepartmentID != nil {
			s := u.PreferredDepartmentID.String()
			ur.DepartmentID = &s
		}
		out = append(out, ur)
	}

	page := f.Page
	if page < 1 {
		page = 1
	}
	pageSize := f.PageSize
	if pageSize < 1 {
		pageSize = 100
	}
	response.Paginated(c, out, page, pageSize, total)
}
