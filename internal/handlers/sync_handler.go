package handlers

import (
	"net/http"

	"github.com/caretex/caretexnursing.core/internal/integrations/caretx"
	"github.com/caretex/caretexnursing.core/internal/middleware"
	"github.com/caretex/caretexnursing.core/internal/services"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
)

type SyncHandler struct {
	syncSvc     services.CaretxSyncService
	caretxClient caretx.Client
}

func NewSyncHandler(syncSvc services.CaretxSyncService, client caretx.Client) *SyncHandler {
	return &SyncHandler{syncSvc: syncSvc, caretxClient: client}
}

// WhoAmI verifies the Caretex credential.
// GET /api/v1/sync/caretx/whoami
func (h *SyncHandler) WhoAmI(c *gin.Context) {
	resp, err := h.caretxClient.WhoAmI(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{
			"success": false,
			"message": "Caretex credential check failed: " + err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    resp,
	})
}

// SyncAll triggers a full pull from Caretex: departments first, then residents.
// POST /api/v1/sync/caretx
func (h *SyncHandler) SyncAll(c *gin.Context) {
	claims := middleware.MustClaims(c)
	orgID, _ := uuid.Parse(claims.OrganizationID)

	log.Info().Str("org_id", orgID.String()).Msg("sync: starting full Caretex sync")

	result, err := h.syncSvc.SyncAll(c.Request.Context(), orgID)
	if err != nil {
		log.Error().Err(err).Msg("sync: Caretex sync failed")
		c.JSON(http.StatusBadGateway, gin.H{
			"success": false,
			"message": "Caretex sync failed: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Caretex sync completed",
		"data":    result,
	})
}

// SyncDepartments triggers a department-only pull from Caretex.
// POST /api/v1/sync/caretx/departments
func (h *SyncHandler) SyncDepartments(c *gin.Context) {
	claims := middleware.MustClaims(c)
	orgID, _ := uuid.Parse(claims.OrganizationID)

	result, err := h.syncSvc.SyncDepartments(c.Request.Context(), orgID)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{
			"success": false,
			"message": "Department sync failed: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Department sync completed",
		"data":    result,
	})
}

// SyncResidents triggers a residents-only pull from Caretex.
// POST /api/v1/sync/caretx/residents
func (h *SyncHandler) SyncResidents(c *gin.Context) {
	claims := middleware.MustClaims(c)
	orgID, _ := uuid.Parse(claims.OrganizationID)

	result, err := h.syncSvc.SyncResidents(c.Request.Context(), orgID)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{
			"success": false,
			"message": "Resident sync failed: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Resident sync completed",
		"data":    result,
	})
}

// SyncUsers triggers a users-only pull from Caretex. Imports staff into
// ctxnurse_users so they're selectable in caregiver pickers with real UUIDs.
// POST /api/v1/sync/caretx/users
func (h *SyncHandler) SyncUsers(c *gin.Context) {
	claims := middleware.MustClaims(c)
	orgID, _ := uuid.Parse(claims.OrganizationID)

	result, err := h.syncSvc.SyncUsers(c.Request.Context(), orgID)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{
			"success": false,
			"message": "User sync failed: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "User sync completed",
		"data":    result,
	})
}
