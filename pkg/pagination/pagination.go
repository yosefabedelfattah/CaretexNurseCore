package pagination

import (
	"strconv"

	"github.com/gin-gonic/gin"
)

const (
	DefaultPageSize = 25
	MaxPageSize     = 200
)

type Params struct {
	Page     int
	PageSize int
	Sort     string
	Q        string
}

func (p Params) Offset() int { return (p.Page - 1) * p.PageSize }
func (p Params) Limit() int  { return p.PageSize }

// FromQuery parses ?page=&page_size=&sort=&q=
func FromQuery(c *gin.Context) Params {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	if page < 1 {
		page = 1
	}
	size, _ := strconv.Atoi(c.DefaultQuery("page_size", strconv.Itoa(DefaultPageSize)))
	if size < 1 {
		size = DefaultPageSize
	}
	if size > MaxPageSize {
		size = MaxPageSize
	}
	return Params{
		Page:     page,
		PageSize: size,
		Sort:     c.Query("sort"),
		Q:        c.Query("q"),
	}
}
