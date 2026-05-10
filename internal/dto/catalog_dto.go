package dto

import "time"

type DepartmentResponse struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Code      string    `json:"code"`
	SortOrder int       `json:"sort_order"`
	CreatedAt time.Time `json:"created_at"`
}

type StatusCatalogResponse struct {
	ID     string `json:"id"`
	Code   string `json:"code"`
	NameHe string `json:"name_he"`
	NameEn string `json:"name_en"`
	Active bool   `json:"active"`
}

type AttributeCatalogResponse struct {
	ID       string `json:"id"`
	Code     string `json:"code"`
	NameHe   string `json:"name_he"`
	NameEn   string `json:"name_en"`
	Category string `json:"category"`
	Active   bool   `json:"active"`
}
