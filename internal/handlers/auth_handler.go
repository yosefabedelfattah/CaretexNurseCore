package handlers

import (
	"net/http"

	"github.com/caretex/caretexnursing.core/internal/dto"
	"github.com/caretex/caretexnursing.core/internal/middleware"
	"github.com/caretex/caretexnursing.core/internal/services"
	apperr "github.com/caretex/caretexnursing.core/pkg/errors"
	"github.com/caretex/caretexnursing.core/pkg/response"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type AuthHandler struct{ svc services.AuthService }

func NewAuthHandler(s services.AuthService) *AuthHandler { return &AuthHandler{svc: s} }

func (h *AuthHandler) Login(c *gin.Context) {
	var req dto.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "validation/bind", err.Error())
		return
	}
	pair, err := h.svc.Login(c.Request.Context(), req.Email, req.Password, c.Request.UserAgent(), c.ClientIP())
	if err != nil {
		if apperr.Is(err, apperr.ErrUnauthorized) {
			response.Error(c, http.StatusUnauthorized, "auth/invalid_credentials", "Invalid email or password")
			return
		}
		response.Error(c, http.StatusInternalServerError, "internal", "Login failed")
		return
	}
	response.OK(c, pair, "Login successful")
}

func (h *AuthHandler) Refresh(c *gin.Context) {
	var req dto.RefreshRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "validation/bind", err.Error())
		return
	}
	pair, err := h.svc.Refresh(c.Request.Context(), req.RefreshToken, c.Request.UserAgent(), c.ClientIP())
	if err != nil {
		response.Error(c, http.StatusUnauthorized, "auth/refresh_failed", "Refresh failed")
		return
	}
	response.OK(c, pair, "")
}

func (h *AuthHandler) Logout(c *gin.Context) {
	var req dto.LogoutRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "validation/bind", err.Error())
		return
	}
	if err := h.svc.Logout(c.Request.Context(), req.RefreshToken); err != nil {
		response.Error(c, http.StatusInternalServerError, "internal", "Logout failed")
		return
	}
	response.NoContent(c)
}

func (h *AuthHandler) Me(c *gin.Context) {
	claims := middleware.MustClaims(c)
	if claims == nil {
		response.Error(c, http.StatusUnauthorized, "auth/no_claims", "Authentication required")
		return
	}
	uid, err := uuid.Parse(claims.UserID)
	if err != nil {
		response.Error(c, http.StatusUnauthorized, "auth/no_claims", "Authentication required")
		return
	}
	me, err := h.svc.Me(c.Request.Context(), uid)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "internal", "Could not load profile")
		return
	}
	response.OK(c, me, "")
}
