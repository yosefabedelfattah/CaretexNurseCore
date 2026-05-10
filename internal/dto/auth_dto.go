package dto

import "time"

type LoginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required,min=6"`
}

type RefreshRequest struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
}

type LogoutRequest struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
}

type TokenPair struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	ExpiresAt    time.Time `json:"expires_at"`
	TokenType    string    `json:"token_type"`
}

type MeResponse struct {
	ID             string   `json:"id"`
	Email          string   `json:"email"`
	FullName       string   `json:"full_name"`
	OrganizationID string   `json:"organization_id"`
	Roles          []string `json:"roles"`
	Permissions    []string `json:"permissions"`
}
