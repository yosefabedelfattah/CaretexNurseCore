package models

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
)

// PermissionList is a JSONB-backed slice of permission codes.
type PermissionList []string

func (p PermissionList) Value() (driver.Value, error) {
	if p == nil {
		return []byte("[]"), nil
	}
	return json.Marshal(p)
}

func (p *PermissionList) Scan(src any) error {
	if src == nil {
		*p = PermissionList{}
		return nil
	}
	switch v := src.(type) {
	case []byte:
		return json.Unmarshal(v, p)
	case string:
		return json.Unmarshal([]byte(v), p)
	default:
		return errors.New("unsupported scan type for PermissionList")
	}
}

func (p PermissionList) Has(code string) bool {
	for _, x := range p {
		if x == code {
			return true
		}
	}
	return false
}

type Role struct {
	Base
	Name        string         `gorm:"size:64;uniqueIndex;not null" json:"name"`
	Description string         `gorm:"size:255" json:"description"`
	Permissions PermissionList `gorm:"type:jsonb;not null;default:'[]'" json:"permissions"`
}

func (Role) TableName() string { return "ctxnurse_roles" }
