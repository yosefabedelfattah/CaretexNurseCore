package models

import "github.com/google/uuid"

type UserRole struct {
	Base
	UserID uuid.UUID `gorm:"type:uuid;index;not null" json:"user_id"`
	RoleID uuid.UUID `gorm:"type:uuid;index;not null" json:"role_id"`
}

func (UserRole) TableName() string { return "ctxnurse_user_roles" }
