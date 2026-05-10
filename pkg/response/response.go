package response

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

type FieldError struct {
	Field   string `json:"field,omitempty"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

type Pagination struct {
	Page       int   `json:"page"`
	PageSize   int   `json:"page_size"`
	Total      int64 `json:"total"`
	TotalPages int   `json:"total_pages"`
}

type Meta struct {
	RequestID  string      `json:"request_id,omitempty"`
	Pagination *Pagination `json:"pagination,omitempty"`
}

type Envelope struct {
	Success bool         `json:"success"`
	Message string       `json:"message,omitempty"`
	Data    any          `json:"data,omitempty"`
	Errors  []FieldError `json:"errors,omitempty"`
	Meta    *Meta        `json:"meta,omitempty"`
}

func reqID(c *gin.Context) string {
	if id, ok := c.Get("request_id"); ok {
		if s, ok2 := id.(string); ok2 {
			return s
		}
	}
	return ""
}

func OK(c *gin.Context, data any, message string) {
	c.JSON(http.StatusOK, Envelope{
		Success: true,
		Message: message,
		Data:    data,
		Meta:    &Meta{RequestID: reqID(c)},
	})
}

func Created(c *gin.Context, data any, message string) {
	c.JSON(http.StatusCreated, Envelope{
		Success: true,
		Message: message,
		Data:    data,
		Meta:    &Meta{RequestID: reqID(c)},
	})
}

func NoContent(c *gin.Context) {
	c.Status(http.StatusNoContent)
}

func Paginated(c *gin.Context, data any, page, pageSize int, total int64) {
	totalPages := 0
	if pageSize > 0 {
		totalPages = int((total + int64(pageSize) - 1) / int64(pageSize))
	}
	c.JSON(http.StatusOK, Envelope{
		Success: true,
		Data:    data,
		Meta: &Meta{
			RequestID: reqID(c),
			Pagination: &Pagination{
				Page: page, PageSize: pageSize, Total: total, TotalPages: totalPages,
			},
		},
	})
}

func Error(c *gin.Context, status int, code, message string) {
	c.AbortWithStatusJSON(status, Envelope{
		Success: false,
		Message: message,
		Errors:  []FieldError{{Code: code, Message: message}},
		Meta:    &Meta{RequestID: reqID(c)},
	})
}

func ValidationError(c *gin.Context, errs []FieldError) {
	c.AbortWithStatusJSON(http.StatusUnprocessableEntity, Envelope{
		Success: false,
		Message: "Validation failed",
		Errors:  errs,
		Meta:    &Meta{RequestID: reqID(c)},
	})
}
