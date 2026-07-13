package middleware

import (
	"net/http"
	"strings"
	"time"

	"runninghub-manager/config"
	"runninghub-manager/models"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"gorm.io/gorm"
)

func JWTAuth(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"code": -1, "message": "missing authorization header"})
			c.Abort()
			return
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || parts[0] != "Bearer" {
			c.JSON(http.StatusUnauthorized, gin.H{"code": -1, "message": "invalid authorization format"})
			c.Abort()
			return
		}

		tokenStr := parts[1]

		// Platform API Key path (starts with "sk-")
		if strings.HasPrefix(tokenStr, "sk-") {
			authByPlatformKey(c, db, tokenStr)
			return
		}

		// JWT path
		authByJWT(c, tokenStr)
	}
}

func authByPlatformKey(c *gin.Context, db *gorm.DB, key string) {
	var platformKey models.PlatformKey
	if err := db.Where("key = ? AND is_active = ?", key, true).First(&platformKey).Error; err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"code": -1, "message": "invalid platform key"})
		c.Abort()
		return
	}

	// Check expiration
	if platformKey.ExpiresAt != nil && platformKey.ExpiresAt.Before(time.Now()) {
		c.JSON(http.StatusUnauthorized, gin.H{"code": -1, "message": "platform key expired"})
		c.Abort()
		return
	}

	// Update last_used_at (async to not block request)
	go func() {
		now := time.Now()
		db.Model(&models.PlatformKey{}).Where("id = ?", platformKey.ID).Update("last_used_at", now)
	}()

	// Platform keys have full (admin) access
	c.Set("userID", platformKey.UserID)
	c.Set("username", "api")
	c.Set("isAdmin", true)

	c.Next()
}

func authByJWT(c *gin.Context, tokenStr string) {
	token, err := jwt.Parse(tokenStr, func(token *jwt.Token) (interface{}, error) {
		return []byte(config.AppConfig.JWTSecret), nil
	})

	if err != nil || !token.Valid {
		c.JSON(http.StatusUnauthorized, gin.H{"code": -1, "message": "invalid or expired token"})
		c.Abort()
		return
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"code": -1, "message": "invalid token claims"})
		c.Abort()
		return
	}

	// Extract user info from claims
	userIDFloat, _ := claims["sub"].(float64)
	userID := uint(userIDFloat)
	username, _ := claims["username"].(string)
	isAdmin, _ := claims["isAdmin"].(bool)

	c.Set("userID", userID)
	c.Set("username", username)
	c.Set("isAdmin", isAdmin)

	c.Next()
}
