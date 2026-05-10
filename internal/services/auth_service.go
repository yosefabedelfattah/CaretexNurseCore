package services

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"strings"
	"time"

	"github.com/caretex/caretexnursing.core/internal/config"
	"github.com/caretex/caretexnursing.core/internal/dto"
	"github.com/caretex/caretexnursing.core/internal/middleware"
	"github.com/caretex/caretexnursing.core/internal/models"
	"github.com/caretex/caretexnursing.core/internal/repositories"
	apperr "github.com/caretex/caretexnursing.core/pkg/errors"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

type AuthService interface {
	Login(ctx context.Context, email, password, ua, ip string) (*dto.TokenPair, error)
	Refresh(ctx context.Context, refreshToken, ua, ip string) (*dto.TokenPair, error)
	Logout(ctx context.Context, refreshToken string) error
	Me(ctx context.Context, userID uuid.UUID) (*dto.MeResponse, error)
}

type authService struct {
	users    repositories.UserRepository
	refresh  repositories.RefreshTokenRepository
	cfg      *config.Config
}

func NewAuthService(u repositories.UserRepository, r repositories.RefreshTokenRepository, cfg *config.Config) AuthService {
	return &authService{users: u, refresh: r, cfg: cfg}
}

func (s *authService) Login(ctx context.Context, email, password, ua, ip string) (*dto.TokenPair, error) {
	user, err := s.users.FindByEmail(ctx, email)
	if err != nil {
		if apperr.Is(err, apperr.ErrNotFound) {
			return nil, apperr.ErrUnauthorized
		}
		return nil, err
	}
	if user.Status != "active" {
		return nil, apperr.ErrUnauthorized
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return nil, apperr.ErrUnauthorized
	}
	roles, err := s.users.GetRoles(ctx, user.ID)
	if err != nil {
		return nil, err
	}
	pair, err := s.issueTokens(ctx, user, roles, ua, ip)
	if err != nil {
		return nil, err
	}
	_ = s.users.UpdateLastLogin(ctx, user.ID)
	return pair, nil
}

func (s *authService) Refresh(ctx context.Context, refreshToken, ua, ip string) (*dto.TokenPair, error) {
	hash := hashToken(refreshToken)
	rt, err := s.refresh.FindByHash(ctx, hash)
	if err != nil {
		if apperr.Is(err, apperr.ErrNotFound) {
			return nil, apperr.ErrTokenInvalid
		}
		return nil, err
	}
	now := time.Now()
	if !rt.IsActive(now) {
		// reuse of revoked token => revoke all sessions for that user
		if rt.RevokedAt != nil {
			_ = s.refresh.RevokeAllForUser(ctx, rt.UserID)
		}
		return nil, apperr.ErrTokenRevoked
	}
	user, err := s.users.FindByID(ctx, rt.UserID)
	if err != nil {
		return nil, err
	}
	roles, err := s.users.GetRoles(ctx, user.ID)
	if err != nil {
		return nil, err
	}
	// rotate: revoke old, issue new
	if err := s.refresh.Revoke(ctx, rt.ID); err != nil {
		return nil, err
	}
	return s.issueTokens(ctx, user, roles, ua, ip)
}

func (s *authService) Logout(ctx context.Context, refreshToken string) error {
	hash := hashToken(refreshToken)
	rt, err := s.refresh.FindByHash(ctx, hash)
	if err != nil {
		if apperr.Is(err, apperr.ErrNotFound) {
			return nil // idempotent
		}
		return err
	}
	return s.refresh.Revoke(ctx, rt.ID)
}

func (s *authService) Me(ctx context.Context, userID uuid.UUID) (*dto.MeResponse, error) {
	user, err := s.users.FindByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	roles, err := s.users.GetRoles(ctx, userID)
	if err != nil {
		return nil, err
	}
	roleNames, perms := flattenRoles(roles)
	return &dto.MeResponse{
		ID:             user.ID.String(),
		Email:          user.Email,
		FullName:       user.FullName,
		OrganizationID: user.OrganizationID.String(),
		Roles:          roleNames,
		Permissions:    perms,
	}, nil
}

// --- helpers ---

func (s *authService) issueTokens(ctx context.Context, user *models.User, roles []models.Role, ua, ip string) (*dto.TokenPair, error) {
	roleNames, perms := flattenRoles(roles)
	now := time.Now()
	exp := now.Add(s.cfg.JWTAccessTTL)

	claims := middleware.Claims{
		UserID:         user.ID.String(),
		OrganizationID: user.OrganizationID.String(),
		Roles:          roleNames,
		Permissions:    perms,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   user.ID.String(),
			ExpiresAt: jwt.NewNumericDate(exp),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			ID:        uuid.NewString(),
		},
	}
	access, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).
		SignedString([]byte(s.cfg.JWTAccessSecret))
	if err != nil {
		return nil, err
	}

	refreshRaw, err := generateRefreshToken()
	if err != nil {
		return nil, err
	}
	rt := &models.RefreshToken{
		Base:      models.Base{ID: uuid.New()},
		UserID:    user.ID,
		TokenHash: hashToken(refreshRaw),
		ExpiresAt: now.Add(s.cfg.JWTRefreshTTL),
		UserAgent: trunc(ua, 255),
		IPAddress: trunc(ip, 64),
	}
	if err := s.refresh.Create(ctx, rt); err != nil {
		return nil, err
	}
	return &dto.TokenPair{
		AccessToken:  access,
		RefreshToken: refreshRaw,
		ExpiresAt:    exp,
		TokenType:    "Bearer",
	}, nil
}

func flattenRoles(roles []models.Role) ([]string, []string) {
	names := make([]string, 0, len(roles))
	permSet := map[string]struct{}{}
	for _, r := range roles {
		names = append(names, r.Name)
		for _, p := range r.Permissions {
			permSet[p] = struct{}{}
		}
	}
	perms := make([]string, 0, len(permSet))
	for p := range permSet {
		perms = append(perms, p)
	}
	return names, perms
}

func generateRefreshToken() (string, error) {
	b := make([]byte, 48)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func hashToken(raw string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(raw)))
	return hex.EncodeToString(sum[:])
}

func trunc(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}
