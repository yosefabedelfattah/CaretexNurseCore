package middleware

import (
	"net/http"
	"strings"

	"github.com/caretex/caretexnursing.core/internal/config"
	"github.com/caretex/caretexnursing.core/pkg/response"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

type Claims struct {
	UserID         string   `json:"sub"`
	OrganizationID string   `json:"org"`
	Roles          []string `json:"roles"`
	Permissions    []string `json:"perms"`
	jwt.RegisteredClaims
}

const ClaimsContextKey = "auth_claims"

// Auth validates the bearer JWT and attaches Claims to the context.
func Auth(cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		header := c.GetHeader("Authorization")
		if !strings.HasPrefix(header, "Bearer ") {
			response.Error(c, http.StatusUnauthorized, "auth/missing_token", "Missing or invalid Authorization header")
			return
		}
		tokenStr := strings.TrimPrefix(header, "Bearer ")

		claims := &Claims{}
		token, err := jwt.ParseWithClaims(tokenStr, claims, func(t *jwt.Token) (any, error) {
			if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, jwt.ErrSignatureInvalid
			}
			return []byte(cfg.JWTAccessSecret), nil
		})
		if err != nil || !token.Valid {
			response.Error(c, http.StatusUnauthorized, "auth/invalid_token", "Invalid or expired token")
			return
		}

		c.Set(ClaimsContextKey, claims)
		c.Next()
	}
}

// RequirePermission ensures the authenticated user has the named permission code.
func RequirePermission(code string) gin.HandlerFunc {
	return func(c *gin.Context) {
		v, ok := c.Get(ClaimsContextKey)
		if !ok {
			response.Error(c, http.StatusUnauthorized, "auth/no_claims", "Authentication required")
			return
		}
		claims, ok := v.(*Claims)
		if !ok {
			response.Error(c, http.StatusUnauthorized, "auth/no_claims", "Authentication required")
			return
		}
		for _, p := range claims.Permissions {
			if p == code {
				c.Next()
				return
			}
		}
		response.Error(c, http.StatusForbidden, "auth/forbidden", "Insufficient permissions")
	}
}

// MustClaims fetches the validated claims from the request context.
func MustClaims(c *gin.Context) *Claims {
	v, _ := c.Get(ClaimsContextKey)
	claims, _ := v.(*Claims)
	return claims
}
