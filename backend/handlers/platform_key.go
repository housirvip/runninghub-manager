package handlers

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"strconv"
	"time"

	"runninghub-manager/models"
	"runninghub-manager/pkg"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type PlatformKeyHandler struct {
	DB *gorm.DB
}

func NewPlatformKeyHandler(db *gorm.DB) *PlatformKeyHandler {
	return &PlatformKeyHandler{DB: db}
}

type CreatePlatformKeyRequest struct {
	Name      string  `json:"name" binding:"required"`
	ExpiresAt *string `json:"expiresAt"` // optional ISO 8601 datetime
}

func (h *PlatformKeyHandler) List(c *gin.Context) {
	var keys []models.PlatformKey
	if err := h.DB.Order("created_at DESC").Find(&keys).Error; err != nil {
		pkg.Error(c, http.StatusInternalServerError, "failed to list platform keys")
		return
	}

	// Mask keys: show first 7 chars + "...****"
	type MaskedKey struct {
		ID         uint       `json:"id"`
		UserID     uint       `json:"userId"`
		Name       string     `json:"name"`
		Key        string     `json:"key"`
		IsActive   bool       `json:"isActive"`
		ExpiresAt  *time.Time `json:"expiresAt"`
		LastUsedAt *time.Time `json:"lastUsedAt"`
		CreatedAt  time.Time  `json:"createdAt"`
	}

	result := make([]MaskedKey, 0, len(keys))
	for _, k := range keys {
		masked := k.Key
		if len(masked) > 7 {
			masked = masked[:7] + "...****"
		}
		result = append(result, MaskedKey{
			ID:         k.ID,
			UserID:     k.UserID,
			Name:       k.Name,
			Key:        masked,
			IsActive:   k.IsActive,
			ExpiresAt:  k.ExpiresAt,
			LastUsedAt: k.LastUsedAt,
			CreatedAt:  k.CreatedAt,
		})
	}

	pkg.Success(c, result)
}

func (h *PlatformKeyHandler) Create(c *gin.Context) {
	// Only admins can create platform keys
	isAdmin, _ := c.Get("isAdmin")
	if !isAdmin.(bool) {
		pkg.Error(c, http.StatusForbidden, "only admins can create platform keys")
		return
	}

	var req CreatePlatformKeyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		pkg.Error(c, http.StatusBadRequest, "invalid request: "+err.Error())
		return
	}

	// Parse optional expiration
	var expiresAt *time.Time
	if req.ExpiresAt != nil && *req.ExpiresAt != "" {
		t, err := time.Parse(time.RFC3339, *req.ExpiresAt)
		if err != nil {
			pkg.Error(c, http.StatusBadRequest, "invalid expiresAt format, use RFC3339 (e.g. 2025-12-31T00:00:00Z)")
			return
		}
		expiresAt = &t
	}

	// Generate key: sk- + 32 random hex chars
	keyBytes := make([]byte, 16)
	if _, err := rand.Read(keyBytes); err != nil {
		pkg.Error(c, http.StatusInternalServerError, "failed to generate key")
		return
	}
	key := "sk-" + hex.EncodeToString(keyBytes)

	userID, _ := c.Get("userID")
	platformKey := models.PlatformKey{
		UserID:    userID.(uint),
		Name:      req.Name,
		Key:       key,
		IsActive:  true,
		ExpiresAt: expiresAt,
	}

	if err := h.DB.Create(&platformKey).Error; err != nil {
		pkg.Error(c, http.StatusInternalServerError, "failed to create platform key")
		return
	}

	// Return full key (only shown once)
	pkg.Success(c, gin.H{
		"id":        platformKey.ID,
		"name":      platformKey.Name,
		"key":       key,
		"expiresAt": platformKey.ExpiresAt,
		"createdAt": platformKey.CreatedAt,
	})
}

func (h *PlatformKeyHandler) Delete(c *gin.Context) {
	// Only admins can delete platform keys
	isAdmin, _ := c.Get("isAdmin")
	if !isAdmin.(bool) {
		pkg.Error(c, http.StatusForbidden, "only admins can delete platform keys")
		return
	}

	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		pkg.Error(c, http.StatusBadRequest, "invalid id")
		return
	}

	if err := h.DB.Delete(&models.PlatformKey{}, id).Error; err != nil {
		pkg.Error(c, http.StatusInternalServerError, "failed to delete platform key")
		return
	}

	pkg.Success(c, nil)
}

func (h *PlatformKeyHandler) Reveal(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		pkg.Error(c, http.StatusBadRequest, "invalid id")
		return
	}

	var key models.PlatformKey
	if err := h.DB.First(&key, id).Error; err != nil {
		pkg.Error(c, http.StatusNotFound, "platform key not found")
		return
	}

	// Admin or owner can reveal
	userID, _ := c.Get("userID")
	isAdmin, _ := c.Get("isAdmin")
	if !isAdmin.(bool) && key.UserID != userID.(uint) {
		pkg.Error(c, http.StatusForbidden, "无权查看此密钥")
		return
	}

	pkg.Success(c, gin.H{"key": key.Key})
}
