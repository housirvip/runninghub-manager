package middleware

import (
	"strings"
	"time"

	"runninghub-manager/models"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// RequestLogger records API requests to the database for analytics.
func RequestLogger(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		path := c.Request.URL.Path

		// Skip static file and health-check paths
		if strings.HasPrefix(path, "/files/") ||
			strings.HasPrefix(path, "/uploads/") ||
			strings.HasPrefix(path, "/assets/") ||
			path == "/favicon.ico" ||
			path == "/favicon.svg" {
			c.Next()
			return
		}

		start := time.Now()
		c.Next()

		// After handler completes
		latency := time.Since(start).Milliseconds()

		entry := models.RequestLog{
			Method:     c.Request.Method,
			Path:       path,
			StatusCode: c.Writer.Status(),
			Latency:    latency,
			ClientIP:   c.ClientIP(),
			CreatedAt:  time.Now(),
		}

		// Extract user info from context (set by auth middleware)
		if uid, exists := c.Get("userID"); exists {
			if id, ok := uid.(uint); ok {
				entry.UserID = &id
			}
		}
		if uname, exists := c.Get("username"); exists {
			if name, ok := uname.(string); ok {
				entry.Username = name
			}
		}

		// Write async to avoid blocking the response
		go db.Create(&entry)
	}
}
