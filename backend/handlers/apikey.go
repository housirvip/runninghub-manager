package handlers

import (
	"net/http"
	"strconv"

	"runninghub-manager/models"
	"runninghub-manager/pkg"
	"runninghub-manager/services"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type ApiKeyHandler struct {
	DB        *gorm.DB
	Scheduler *services.Scheduler
}

func NewApiKeyHandler(db *gorm.DB, scheduler *services.Scheduler) *ApiKeyHandler {
	return &ApiKeyHandler{DB: db, Scheduler: scheduler}
}

type CreateApiKeyRequest struct {
	Name           string `json:"name" binding:"required"`
	ApiKey         string `json:"apiKey" binding:"required"`
	BaseURL        string `json:"baseUrl"`
	MaxConcurrency int    `json:"maxConcurrency"`
}

type UpdateApiKeyRequest struct {
	Name           string `json:"name"`
	BaseURL        string `json:"baseUrl"`
	MaxConcurrency int    `json:"maxConcurrency"`
	IsActive       *bool  `json:"isActive"`
}

func (h *ApiKeyHandler) List(c *gin.Context) {
	var keys []models.ApiKey
	if err := h.DB.Order("created_at DESC").Find(&keys).Error; err != nil {
		pkg.Error(c, http.StatusInternalServerError, "failed to list keys")
		return
	}

	// Add current task counts
	type result struct {
		ApiKeyID     uint
		CurrentTasks int64
	}
	var counts []result
	h.DB.Model(&models.Task{}).
		Select("api_key_id, count(*) as current_tasks").
		Where("status IN ?", []models.TaskStatus{models.TaskStatusDispatched, models.TaskStatusRunning}).
		Group("api_key_id").
		Find(&counts)

	countMap := make(map[uint]int64)
	for _, r := range counts {
		countMap[r.ApiKeyID] = r.CurrentTasks
	}

	type KeyWithLoad struct {
		models.ApiKey
		CurrentTasks int64 `json:"currentTasks"`
	}

	response := make([]KeyWithLoad, len(keys))
	for i, k := range keys {
		response[i] = KeyWithLoad{
			ApiKey:       k,
			CurrentTasks: countMap[k.ID],
		}
	}

	pkg.Success(c, response)
}

func (h *ApiKeyHandler) Create(c *gin.Context) {
	var req CreateApiKeyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		pkg.Error(c, http.StatusBadRequest, "invalid request: "+err.Error())
		return
	}

	if req.MaxConcurrency <= 0 {
		req.MaxConcurrency = 3
	}

	baseURL := req.BaseURL
	if baseURL == "" {
		baseURL = "https://www.runninghub.cn"
	}

	key := models.ApiKey{
		Name:           req.Name,
		ApiKey:         req.ApiKey,
		BaseURL:        baseURL,
		MaxConcurrency: req.MaxConcurrency,
		IsActive:       true,
	}

	if err := h.DB.Create(&key).Error; err != nil {
		pkg.Error(c, http.StatusInternalServerError, "failed to create key")
		return
	}

	// Start worker for new key
	h.Scheduler.AddWorker(&key)

	pkg.Success(c, key)
}

func (h *ApiKeyHandler) Update(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		pkg.Error(c, http.StatusBadRequest, "invalid id")
		return
	}

	var key models.ApiKey
	if err := h.DB.First(&key, id).Error; err != nil {
		pkg.Error(c, http.StatusNotFound, "key not found")
		return
	}

	var req UpdateApiKeyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		pkg.Error(c, http.StatusBadRequest, "invalid request: "+err.Error())
		return
	}

	if req.Name != "" {
		key.Name = req.Name
	}
	if req.BaseURL != "" {
		key.BaseURL = req.BaseURL
	}
	if req.MaxConcurrency > 0 {
		key.MaxConcurrency = req.MaxConcurrency
	}
	if req.IsActive != nil {
		key.IsActive = *req.IsActive
	}

	if err := h.DB.Save(&key).Error; err != nil {
		pkg.Error(c, http.StatusInternalServerError, "failed to update key")
		return
	}

	// Update worker
	h.Scheduler.UpdateWorker(&key)

	pkg.Success(c, key)
}

func (h *ApiKeyHandler) Delete(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		pkg.Error(c, http.StatusBadRequest, "invalid id")
		return
	}

	var key models.ApiKey
	if err := h.DB.First(&key, id).Error; err != nil {
		pkg.Error(c, http.StatusNotFound, "key not found")
		return
	}

	// Check if any tasks reference this key
	var count int64
	h.DB.Model(&models.Task{}).Where("api_key_id = ? AND status IN ?",
		id, []models.TaskStatus{models.TaskStatusDispatched, models.TaskStatusRunning}).Count(&count)
	if count > 0 {
		pkg.Error(c, http.StatusConflict, "key has active tasks, deactivate it instead")
		return
	}

	// Stop worker
	h.Scheduler.RemoveWorker(uint(id))

	if err := h.DB.Delete(&key).Error; err != nil {
		pkg.Error(c, http.StatusInternalServerError, "failed to delete key")
		return
	}

	pkg.Success(c, nil)
}
