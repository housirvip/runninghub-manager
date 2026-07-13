package handlers

import (
	"log"
	"net/http"
	"strconv"

	"runninghub-manager/models"
	"runninghub-manager/pkg"
	"runninghub-manager/services"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type TaskHandler struct {
	DB       *gorm.DB
	RHClient *services.RHClient
}

func NewTaskHandler(db *gorm.DB, rhClient *services.RHClient) *TaskHandler {
	return &TaskHandler{DB: db, RHClient: rhClient}
}

func (h *TaskHandler) List(c *gin.Context) {
	userID, _ := c.Get("userID")
	isAdmin, _ := c.Get("isAdmin")

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "20"))
	status := c.Query("status")

	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	query := h.DB.Model(&models.Task{})

	// Non-admin only sees own tasks
	if !isAdmin.(bool) {
		query = query.Where("user_id = ?", userID)
	}

	if status != "" {
		query = query.Where("status = ?", status)
	}

	var total int64
	query.Count(&total)

	var tasks []models.Task
	query.Order("created_at DESC").
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		Find(&tasks)

	// Enrich with api key names
	if len(tasks) > 0 {
		keyIDs := make([]uint, 0)
		for _, t := range tasks {
			if t.ApiKeyID != nil {
				keyIDs = append(keyIDs, *t.ApiKeyID)
			}
		}
		if len(keyIDs) > 0 {
			var keys []models.ApiKey
			h.DB.Where("id IN ?", keyIDs).Find(&keys)
			keyMap := make(map[uint]string)
			for _, k := range keys {
				keyMap[k.ID] = k.Name
			}
			for i := range tasks {
				if tasks[i].ApiKeyID != nil {
					tasks[i].ApiKeyName = keyMap[*tasks[i].ApiKeyID]
				}
			}
		}
	}

	pkg.Success(c, gin.H{
		"items":    tasks,
		"total":    total,
		"page":     page,
		"pageSize": pageSize,
	})
}

func (h *TaskHandler) Get(c *gin.Context) {
	taskID := c.Param("id")
	userID, _ := c.Get("userID")
	isAdmin, _ := c.Get("isAdmin")

	var task models.Task
	if err := h.DB.First(&task, "id = ?", taskID).Error; err != nil {
		pkg.Error(c, http.StatusNotFound, "task not found")
		return
	}

	if !isAdmin.(bool) && task.UserID != userID.(uint) {
		pkg.Error(c, http.StatusForbidden, "access denied")
		return
	}

	// Get api key name
	if task.ApiKeyID != nil {
		var key models.ApiKey
		if err := h.DB.First(&key, task.ApiKeyID).Error; err == nil {
			task.ApiKeyName = key.Name
		}
	}

	pkg.Success(c, task)
}

func (h *TaskHandler) Cancel(c *gin.Context) {
	taskID := c.Param("id")
	userID, _ := c.Get("userID")
	isAdmin, _ := c.Get("isAdmin")

	var task models.Task
	if err := h.DB.First(&task, "id = ?", taskID).Error; err != nil {
		pkg.Error(c, http.StatusNotFound, "task not found")
		return
	}

	if !isAdmin.(bool) && task.UserID != userID.(uint) {
		pkg.Error(c, http.StatusForbidden, "access denied")
		return
	}

	// Can only cancel non-terminal tasks
	switch task.Status {
	case models.TaskStatusPending, models.TaskStatusDispatched, models.TaskStatusRunning, models.TaskStatusQueued:
		// If task has been submitted to RunningHub, call cancel API
		if task.RhTaskID != "" && task.ApiKeyID != nil {
			var apiKey models.ApiKey
			if err := h.DB.First(&apiKey, task.ApiKeyID).Error; err == nil {
				result, err := h.RHClient.CancelTask(apiKey.ApiKey, task.RhTaskID)
				if err != nil {
					log.Printf("[Cancel] Failed to cancel task %s on RunningHub: %v", task.RhTaskID, err)
				} else if result.Code != 0 {
					log.Printf("[Cancel] RunningHub cancel returned code %d: %s", result.Code, result.Msg)
				} else {
					log.Printf("[Cancel] Task %s cancelled on RunningHub", task.RhTaskID)
				}
			}
		}
		h.DB.Model(&task).Update("status", models.TaskStatusCancelled)
		pkg.Success(c, gin.H{"status": "CANCELLED"})
	default:
		pkg.Error(c, http.StatusBadRequest, "task cannot be cancelled in current state: "+string(task.Status))
	}
}
