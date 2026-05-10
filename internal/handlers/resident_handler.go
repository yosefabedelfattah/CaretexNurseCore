package handlers

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/caretex/caretexnursing.core/internal/dto"
	"github.com/caretex/caretexnursing.core/internal/middleware"
	"github.com/caretex/caretexnursing.core/internal/models"
	"github.com/caretex/caretexnursing.core/internal/repositories"
	"github.com/caretex/caretexnursing.core/internal/services"
	apperr "github.com/caretex/caretexnursing.core/pkg/errors"
	"github.com/caretex/caretexnursing.core/pkg/response"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type ResidentHandler struct {
	svc      services.ResidentService
	users    repositories.UserRepository
	depts    repositories.DepartmentRepository
}

func NewResidentHandler(s services.ResidentService, u repositories.UserRepository, d repositories.DepartmentRepository) *ResidentHandler {
	return &ResidentHandler{svc: s, users: u, depts: d}
}

func (h *ResidentHandler) List(c *gin.Context) {
	claims := middleware.MustClaims(c)
	orgID, err := uuid.Parse(claims.OrganizationID)
	if err != nil {
		response.Error(c, http.StatusUnauthorized, "auth/bad_org", "Invalid organization claim")
		return
	}
	actorID, _ := uuid.Parse(claims.UserID)

	filter := h.parseFilter(c)

	// If caller didn't specify a department and didn't say "all", fall back to
	// the current user's preferred department. This is the rule in the
	// requirements: "the default is the user's preferred department".
	if filter.DepartmentID == "" {
		if u, err := h.users.FindByID(c.Request.Context(), actorID); err == nil && u.PreferredDepartmentID != nil {
			filter.DepartmentID = u.PreferredDepartmentID.String()
		} else {
			filter.DepartmentID = "all"
		}
	}

	items, total, err := h.svc.List(c.Request.Context(), orgID, filter)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "internal", "Failed to list residents")
		return
	}
	out := make([]dto.ResidentResponse, 0, len(items))
	for i := range items {
		out = append(out, toResidentResponse(&items[i]))
	}
	page := filter.Page
	if page < 1 {
		page = 1
	}
	pageSize := filter.PageSize
	if pageSize < 1 {
		pageSize = 25
	}
	response.Paginated(c, out, page, pageSize, total)
}

func (h *ResidentHandler) Create(c *gin.Context) {
	claims := middleware.MustClaims(c)
	orgID, _ := uuid.Parse(claims.OrganizationID)
	actorID, _ := uuid.Parse(claims.UserID)

	var req dto.CreateResidentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "validation/bind", err.Error())
		return
	}
	r, err := h.svc.Create(c.Request.Context(), orgID, actorID, req)
	if err != nil {
		mapResidentError(c, err)
		return
	}
	response.Created(c, toResidentResponse(r), "Resident created")
}

func (h *ResidentHandler) Get(c *gin.Context) {
	claims := middleware.MustClaims(c)
	orgID, _ := uuid.Parse(claims.OrganizationID)
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.Error(c, http.StatusBadRequest, "validation/id", "Invalid id")
		return
	}
	r, err := h.svc.Get(c.Request.Context(), orgID, id)
	if err != nil {
		mapResidentError(c, err)
		return
	}
	response.OK(c, toResidentResponse(r), "")
}

func (h *ResidentHandler) Update(c *gin.Context) {
	claims := middleware.MustClaims(c)
	orgID, _ := uuid.Parse(claims.OrganizationID)
	actorID, _ := uuid.Parse(claims.UserID)
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.Error(c, http.StatusBadRequest, "validation/id", "Invalid id")
		return
	}
	var req dto.UpdateResidentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "validation/bind", err.Error())
		return
	}
	r, err := h.svc.Update(c.Request.Context(), orgID, actorID, id, req)
	if err != nil {
		mapResidentError(c, err)
		return
	}
	response.OK(c, toResidentResponse(r), "Resident updated")
}

func (h *ResidentHandler) Delete(c *gin.Context) {
	claims := middleware.MustClaims(c)
	orgID, _ := uuid.Parse(claims.OrganizationID)
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.Error(c, http.StatusBadRequest, "validation/id", "Invalid id")
		return
	}
	if err := h.svc.Delete(c.Request.Context(), orgID, id); err != nil {
		mapResidentError(c, err)
		return
	}
	response.NoContent(c)
}

// parseFilter pulls every filter knob from the query string. It accepts
// repeated keys (?status_codes=a&status_codes=b) and CSV (?status_codes=a,b).
func (h *ResidentHandler) parseFilter(c *gin.Context) dto.ResidentListFilter {
	f := dto.ResidentListFilter{
		DepartmentID: c.Query("department_id"),
		Q:            c.Query("q"),
		Sort:         c.DefaultQuery("sort", "last_name"),
		StatusCodes:  multiQuery(c, "status_codes"),
		Rooms:        multiQuery(c, "rooms"),
	}
	if v := c.Query("has_attributes"); v != "" {
		b := parseTriBool(v)
		f.HasAttributes = b
	}
	if v := c.Query("has_treatment_plan"); v != "" {
		b := parseTriBool(v)
		f.HasTreatmentPlan = b
	}
	if v, _ := strconv.Atoi(c.Query("page")); v > 0 {
		f.Page = v
	}
	if v, _ := strconv.Atoi(c.Query("page_size")); v > 0 {
		f.PageSize = v
	}
	return f
}

func multiQuery(c *gin.Context, key string) []string {
	out := []string{}
	for _, v := range c.QueryArray(key) {
		for _, p := range strings.Split(v, ",") {
			p = strings.TrimSpace(p)
			if p != "" {
				out = append(out, p)
			}
		}
	}
	return out
}

func parseTriBool(s string) *bool {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "true", "1", "yes":
		t := true
		return &t
	case "false", "0", "no":
		f := false
		return &f
	}
	return nil
}

func toResidentResponse(r *models.Resident) dto.ResidentResponse {
	out := dto.ResidentResponse{
		ID:               r.ID.String(),
		OrganizationID:   r.OrganizationID.String(),
		MRN:              r.MRN,
		FirstName:        r.FirstName,
		LastName:         r.LastName,
		DOB:              r.DOB,
		Gender:           r.Gender,
		RoomNumber:       r.RoomNumber,
		Phone:            r.Phone,
		Email:            r.Email,
		PhotoURL:         r.PhotoURL,
		Notes:            r.Notes,
		HasTreatmentPlan: r.HasTreatmentPlan,
		CaretxID:         r.CaretxID,
		Statuses:         []dto.StatusRef{},
		Attributes:       []dto.AttributeRef{},
		CreatedAt:        r.CreatedAt,
		UpdatedAt:        r.UpdatedAt,
	}
	if r.Department != nil {
		out.Department = &dto.DepartmentRef{
			ID:   r.Department.ID.String(),
			Name: r.Department.Name,
			Code: r.Department.Code,
		}
	}
	for _, s := range r.Statuses {
		out.Statuses = append(out.Statuses, dto.StatusRef{
			ID:     s.ID.String(),
			Code:   s.Code,
			NameHe: s.NameHe,
			NameEn: s.NameEn,
		})
	}
	for _, a := range r.Attributes {
		out.Attributes = append(out.Attributes, dto.AttributeRef{
			ID:       a.ID.String(),
			Code:     a.Code,
			NameHe:   a.NameHe,
			NameEn:   a.NameEn,
			Category: a.Category,
		})
	}
	return out
}

func mapResidentError(c *gin.Context, err error) {
	switch {
	case apperr.Is(err, apperr.ErrNotFound):
		response.Error(c, http.StatusNotFound, "residents/not_found", "Resident not found")
	case apperr.Is(err, apperr.ErrConflict):
		response.Error(c, http.StatusConflict, "residents/mrn_conflict", "A resident with this MRN already exists")
	case apperr.Is(err, apperr.ErrInvalidInput):
		response.Error(c, http.StatusBadRequest, "residents/invalid_input", err.Error())
	default:
		response.Error(c, http.StatusInternalServerError, "internal", "Operation failed")
	}
}
