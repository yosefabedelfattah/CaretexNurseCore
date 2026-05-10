package handlers

import (
	"net/http"
	"strconv"
	"time"

	"github.com/caretex/caretexnursing.core/internal/middleware"
	"github.com/caretex/caretexnursing.core/internal/models"
	"github.com/caretex/caretexnursing.core/internal/repositories"
	"github.com/caretex/caretexnursing.core/pkg/response"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type TaskHandler struct {
	repo repositories.TaskRepository
}

func NewTaskHandler(repo repositories.TaskRepository) *TaskHandler {
	return &TaskHandler{repo: repo}
}

// List tasks with filters.
// GET /api/v1/tasks?scope=resident&status=assigned&shift=morning&caregiver_id=...
func (h *TaskHandler) List(c *gin.Context) {
	claims := middleware.MustClaims(c)
	orgID, _ := uuid.Parse(claims.OrganizationID)

	f := repositories.TaskFilter{
		Scope:        c.Query("scope"),
		DepartmentID: c.Query("department_id"),
		ResidentID:   c.Query("resident_id"),
		CaregiverID:  c.Query("caregiver_id"),
		Status:       c.Query("status"),
		Shift:        c.Query("shift"),
		Priority:     c.Query("priority"),
	}
	if v, _ := strconv.Atoi(c.Query("page")); v > 0 {
		f.Page = v
	}
	if v, _ := strconv.Atoi(c.Query("page_size")); v > 0 {
		f.PageSize = v
	}

	items, total, err := h.repo.List(c.Request.Context(), orgID, f)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "internal", "Failed to list tasks")
		return
	}

	page := f.Page
	if page < 1 {
		page = 1
	}
	pageSize := f.PageSize
	if pageSize < 1 {
		pageSize = 50
	}
	response.Paginated(c, items, page, pageSize, total)
}

// Get a single task.
// GET /api/v1/tasks/:id
func (h *TaskHandler) Get(c *gin.Context) {
	claims := middleware.MustClaims(c)
	orgID, _ := uuid.Parse(claims.OrganizationID)
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.Error(c, http.StatusBadRequest, "validation", "Invalid task ID")
		return
	}
	t, err := h.repo.FindByID(c.Request.Context(), orgID, id)
	if err != nil {
		response.Error(c, http.StatusNotFound, "not_found", "Task not found")
		return
	}
	response.OK(c, t, "")
}

// Create a new task.
// POST /api/v1/tasks
func (h *TaskHandler) Create(c *gin.Context) {
	claims := middleware.MustClaims(c)
	orgID, _ := uuid.Parse(claims.OrganizationID)

	var req models.AssignedTask
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "validation", err.Error())
		return
	}

	req.ID = uuid.New()
	req.OrganizationID = orgID
	if req.Status == "" {
		if req.AssignedCaregiverID != nil {
			req.Status = "assigned"
		} else {
			req.Status = "unassigned"
		}
	}
	if req.Priority == "" {
		req.Priority = "normal"
	}
	if req.Shift == "" {
		req.Shift = currentShift()
	}
	if req.ActionIcon == "" {
		req.ActionIcon = "task_alt"
	}
	if req.DueAt.IsZero() {
		req.DueAt = time.Now().Add(1 * time.Hour)
	}

	if err := h.repo.Create(c.Request.Context(), &req); err != nil {
		response.Error(c, http.StatusInternalServerError, "internal", "Failed to create task")
		return
	}
	response.Created(c, req, "Task created")
}

// Update a task (full update).
// PUT /api/v1/tasks/:id
func (h *TaskHandler) Update(c *gin.Context) {
	claims := middleware.MustClaims(c)
	orgID, _ := uuid.Parse(claims.OrganizationID)
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.Error(c, http.StatusBadRequest, "validation", "Invalid task ID")
		return
	}

	existing, err := h.repo.FindByID(c.Request.Context(), orgID, id)
	if err != nil {
		response.Error(c, http.StatusNotFound, "not_found", "Task not found")
		return
	}

	var req models.AssignedTask
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "validation", err.Error())
		return
	}

	req.ID = existing.ID
	req.OrganizationID = orgID
	req.CreatedAt = existing.CreatedAt
	req.CreatedBy = existing.CreatedBy

	// Auto-set timestamps based on status transitions
	if req.Status == "in_progress" && existing.Status != "in_progress" && req.StartedAt == nil {
		now := time.Now()
		req.StartedAt = &now
	}
	if req.Status == "done" && existing.Status != "done" && req.CompletedAt == nil {
		now := time.Now()
		req.CompletedAt = &now
	}
	if req.Status != "done" {
		req.CompletedAt = nil
	}

	if err := h.repo.Update(c.Request.Context(), &req); err != nil {
		response.Error(c, http.StatusInternalServerError, "internal", "Failed to update task")
		return
	}
	response.OK(c, req, "Task updated")
}

// Progress moves a task to the next status.
// PATCH /api/v1/tasks/:id/progress
// Body: {"status": "in_progress"} or {"status": "done"}
func (h *TaskHandler) Progress(c *gin.Context) {
	claims := middleware.MustClaims(c)
	orgID, _ := uuid.Parse(claims.OrganizationID)
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.Error(c, http.StatusBadRequest, "validation", "Invalid task ID")
		return
	}

	t, err := h.repo.FindByID(c.Request.Context(), orgID, id)
	if err != nil {
		response.Error(c, http.StatusNotFound, "not_found", "Task not found")
		return
	}

	var body struct {
		Status string `json:"status"`
		Notes  string `json:"notes"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		response.Error(c, http.StatusBadRequest, "validation", err.Error())
		return
	}

	now := time.Now()
	t.Status = body.Status
	if body.Notes != "" {
		t.Notes = body.Notes
	}

	switch body.Status {
	case "in_progress":
		if t.StartedAt == nil {
			t.StartedAt = &now
		}
	case "done":
		t.CompletedAt = &now
	case "unassigned":
		t.AssignedCaregiverID = nil
		t.AssignedCaregiverName = ""
		t.StartedAt = nil
		t.CompletedAt = nil
	}

	if err := h.repo.Update(c.Request.Context(), t); err != nil {
		response.Error(c, http.StatusInternalServerError, "internal", "Failed to update task")
		return
	}
	response.OK(c, t, "Task progressed")
}

// Delete a task.
// DELETE /api/v1/tasks/:id
func (h *TaskHandler) Delete(c *gin.Context) {
	claims := middleware.MustClaims(c)
	orgID, _ := uuid.Parse(claims.OrganizationID)
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.Error(c, http.StatusBadRequest, "validation", "Invalid task ID")
		return
	}
	if err := h.repo.Delete(c.Request.Context(), orgID, id); err != nil {
		response.Error(c, http.StatusInternalServerError, "internal", "Failed to delete task")
		return
	}
	response.OK(c, nil, "Task deleted")
}

func currentShift() string {
	h := time.Now().Hour()
	if h >= 7 && h < 15 {
		return "morning"
	}
	if h >= 15 && h < 23 {
		return "afternoon"
	}
	return "night"
}
