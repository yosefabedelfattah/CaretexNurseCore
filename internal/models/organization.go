package models

type Organization struct {
	Base
	Name   string `gorm:"size:200;not null" json:"name"`
	Code   string `gorm:"size:64;uniqueIndex;not null" json:"code"`
	Status string `gorm:"size:32;not null;default:'active'" json:"status"`
}

func (Organization) TableName() string { return "ctxnurse_organizations" }
