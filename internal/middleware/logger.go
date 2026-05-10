package middleware

import (
	"time"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"
)

func Logger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		latency := time.Since(start)
		reqID, _ := c.Get("request_id")

		entry := log.Info()
		if c.Writer.Status() >= 500 {
			entry = log.Error()
		} else if c.Writer.Status() >= 400 {
			entry = log.Warn()
		}
		entry.
			Str("request_id", toString(reqID)).
			Str("method", c.Request.Method).
			Str("path", c.Request.URL.Path).
			Int("status", c.Writer.Status()).
			Dur("latency", latency).
			Str("ip", c.ClientIP()).
			Msg("request")
	}
}

func toString(v any) string {
	if v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}
