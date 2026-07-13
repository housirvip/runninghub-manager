package handlers

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"

	"runninghub-manager/apps"
	"runninghub-manager/config"
	"runninghub-manager/models"
	"runninghub-manager/pkg"
	"runninghub-manager/services"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type ProxyHandler struct {
	DB        *gorm.DB
	RHClient  *services.RHClient
	Scheduler *services.Scheduler
}

func NewProxyHandler(db *gorm.DB, rhClient *services.RHClient, scheduler *services.Scheduler) *ProxyHandler {
	return &ProxyHandler{DB: db, RHClient: rhClient, Scheduler: scheduler}
}

type ProxyCreateTaskRequest struct {
	ApiKey         string          `json:"apiKey"`
	WebappID       json.RawMessage `json:"webappId"`
	NodeInfoList   json.RawMessage `json:"nodeInfoList" binding:"required"`
	WebhookURL     string          `json:"webhookUrl"`
	InstanceType   string          `json:"instanceType"`
	AccessPassword string          `json:"accessPassword"`
}

func (h *ProxyHandler) CreateTask(c *gin.Context) {
	var req ProxyCreateTaskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		pkg.RHError(c, -1, "invalid request: "+err.Error())
		return
	}

	userID, _ := c.Get("userID")

	// Parse webappId which could be string or number
	webappID := ""
	if req.WebappID != nil {
		// Try as string first
		var s string
		if err := json.Unmarshal(req.WebappID, &s); err == nil {
			webappID = s
		} else {
			// Try as number
			var n json.Number
			if err := json.Unmarshal(req.WebappID, &n); err == nil {
				webappID = n.String()
			} else {
				webappID = string(req.WebappID)
			}
		}
	}

	if webappID == "" {
		pkg.RHError(c, -1, "webappId is required")
		return
	}

	// Store nodeInfoList as JSON string
	nodeInfoJSON := string(req.NodeInfoList)
	task := models.Task{
		UserID:         userID.(uint),
		WebappID:       webappID,
		NodeInfoList:   nodeInfoJSON,
		WebhookURL:     req.WebhookURL,
		InstanceType:   req.InstanceType,
		AccessPassword: req.AccessPassword,
		Status:         models.TaskStatusPending,
		IsLocal:        apps.IsCustomApp(webappID),
	}

	if err := h.DB.Create(&task).Error; err != nil {
		pkg.RHError(c, -1, "failed to create task")
		return
	}

	// Return RunningHub-compatible response
	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"msg":  "success",
		"data": gin.H{
			"taskId":     task.ID,
			"taskStatus": "QUEUED",
			"clientId":   "",
			"netWssUrl":  "",
			"promptTips": "",
		},
	})
}

func (h *ProxyHandler) QueryTask(c *gin.Context) {
	var req struct {
		TaskID string `json:"taskId" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		pkg.RHError(c, -1, "invalid request: "+err.Error())
		return
	}

	userID, _ := c.Get("userID")
	isAdmin, _ := c.Get("isAdmin")

	// Look up by platform ID (numeric) or RunningHub task ID (string)
	var task models.Task
	var findErr error
	if numID, parseErr := strconv.ParseUint(req.TaskID, 10, 64); parseErr == nil {
		findErr = h.DB.Where("id = ? OR rh_task_id = ?", numID, req.TaskID).First(&task).Error
	} else {
		findErr = h.DB.Where("rh_task_id = ?", req.TaskID).First(&task).Error
	}
	if findErr != nil {
		c.JSON(http.StatusOK, gin.H{
			"taskId":       req.TaskID,
			"status":       "FAILED",
			"errorCode":    "404",
			"errorMessage": "task not found",
			"results":      nil,
			"clientId":     "",
			"promptTips":   "",
		})
		return
	}

	// Check ownership
	if !isAdmin.(bool) && task.UserID != userID.(uint) {
		c.JSON(http.StatusOK, gin.H{
			"taskId":       req.TaskID,
			"status":       "FAILED",
			"errorCode":    "403",
			"errorMessage": "access denied",
			"results":      nil,
			"clientId":     "",
			"promptTips":   "",
		})
		return
	}

	// All tasks — return local DB state directly.
	// Worker handles RunningHub polling and updates the DB.
	var results interface{}
	if task.Results != "" {
		json.Unmarshal([]byte(task.Results), &results)
	}

	status := string(task.Status)
	switch task.Status {
	case models.TaskStatusCancelled:
		status = "FAILED"
	case models.TaskStatusPending, models.TaskStatusDispatched:
		status = "QUEUED"
	}

	c.JSON(http.StatusOK, gin.H{
		"taskId":       task.ID,
		"status":       status,
		"errorCode":    "",
		"errorMessage": task.ErrorMessage,
		"results":      results,
		"clientId":     task.RhClientID,
		"promptTips":   task.PromptTips,
	})
}

func (h *ProxyHandler) Upload(c *gin.Context) {
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		pkg.RHError(c, -1, "file is required")
		return
	}
	defer file.Close()

	fileType := c.PostForm("fileType")
	if fileType == "" {
		fileType = "input"
	}

	// Check if this is a local upload
	isLocal := c.Query("local") == "true" || c.PostForm("local") == "true"
	if isLocal {
		// Store file locally
		ext := filepath.Ext(header.Filename)
		fileName := uuid.New().String() + ext
		uploadDir := config.AppConfig.UploadDir
		if err := os.MkdirAll(uploadDir, 0755); err != nil {
			pkg.RHError(c, -1, "failed to create upload dir: "+err.Error())
			return
		}
		destPath := filepath.Join(uploadDir, fileName)
		dest, err := os.Create(destPath)
		if err != nil {
			pkg.RHError(c, -1, "failed to create file: "+err.Error())
			return
		}
		defer dest.Close()
		if _, err := io.Copy(dest, file); err != nil {
			pkg.RHError(c, -1, "failed to save file: "+err.Error())
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"code": 0,
			"msg":  "success",
			"data": gin.H{
				"fileName": fileName,
				"fileType": fileType,
			},
		})
		return
	}

	// Proxy upload to RunningHub
	var apiKey models.ApiKey
	if err := h.DB.Where("is_active = ?", true).First(&apiKey).Error; err != nil {
		pkg.RHError(c, -1, "no active API keys available")
		return
	}

	result, err := h.RHClient.ForBaseURL(apiKey.BaseURL).UploadFile(apiKey.ApiKey, fileType, file, header.Filename)
	if err != nil {
		pkg.RHError(c, -1, "upload failed: "+err.Error())
		return
	}

	c.JSON(http.StatusOK, result)
}

func (h *ProxyHandler) GetWebappInfo(c *gin.Context) {
	webappID := c.Query("webappId")
	if webappID == "" {
		pkg.RHError(c, -1, "webappId is required")
		return
	}

	// Check if it's a local custom app
	if app, ok := apps.Get(webappID); ok {
		c.JSON(http.StatusOK, gin.H{
			"code": 0,
			"msg":  "success",
			"data": gin.H{
				"webappId":     app.ID(),
				"webappName":   app.Name(),
				"nodeInfoList": app.NodeInfoList(),
			},
		})
		return
	}

	// Proxy to RunningHub
	var apiKey models.ApiKey
	if err := h.DB.Where("is_active = ?", true).First(&apiKey).Error; err != nil {
		pkg.RHError(c, -1, "no active API keys available")
		return
	}

	result, err := h.RHClient.ForBaseURL(apiKey.BaseURL).GetWebappInfo(apiKey.ApiKey, webappID)
	if err != nil {
		pkg.RHError(c, -1, "failed to get webapp info: "+err.Error())
		return
	}

	c.Data(http.StatusOK, "application/json", result)
}

func (h *ProxyHandler) CancelTask(c *gin.Context) {
	var req struct {
		ApiKey string `json:"apiKey"`
		TaskID string `json:"taskId" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		pkg.RHError(c, -1, "invalid request: "+err.Error())
		return
	}

	userID, _ := c.Get("userID")
	isAdmin, _ := c.Get("isAdmin")

	// Look up by platform ID or RunningHub task ID
	var task models.Task
	var findErr error
	if numID, parseErr := strconv.ParseUint(req.TaskID, 10, 64); parseErr == nil {
		findErr = h.DB.Where("id = ? OR rh_task_id = ?", numID, req.TaskID).First(&task).Error
	} else {
		findErr = h.DB.Where("rh_task_id = ?", req.TaskID).First(&task).Error
	}
	if findErr != nil {
		c.JSON(http.StatusOK, gin.H{"code": 807, "msg": "APIKEY_TASK_NOT_FOUND", "data": nil})
		return
	}

	// Access check
	if !isAdmin.(bool) && task.UserID != userID.(uint) {
		c.JSON(http.StatusOK, gin.H{"code": 403, "msg": "access denied", "data": nil})
		return
	}

	// Only cancel non-terminal tasks
	switch task.Status {
	case models.TaskStatusPending, models.TaskStatusDispatched, models.TaskStatusRunning, models.TaskStatusQueued:
		// Call RunningHub cancel if task was submitted
		if task.RhTaskID != "" && task.ApiKeyID != nil {
			var apiKey models.ApiKey
			if err := h.DB.First(&apiKey, task.ApiKeyID).Error; err == nil {
				h.RHClient.ForBaseURL(apiKey.BaseURL).CancelTask(apiKey.ApiKey, task.RhTaskID)
			}
		}
		h.DB.Model(&task).Update("status", models.TaskStatusCancelled)
		c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "success", "data": nil})
	default:
		c.JSON(http.StatusOK, gin.H{"code": -1, "msg": "task cannot be cancelled in current state: " + string(task.Status), "data": nil})
	}
}

// QueryTaskOutputs handles the legacy POST /task/openapi/outputs endpoint.
// Response format differs from V2: uses code-based status indication.
func (h *ProxyHandler) QueryTaskOutputs(c *gin.Context) {
	var req struct {
		ApiKey string `json:"apiKey"`
		TaskID string `json:"taskId" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": -1, "msg": "invalid request: " + err.Error(), "data": nil})
		return
	}

	userID, _ := c.Get("userID")
	isAdmin, _ := c.Get("isAdmin")

	var task models.Task
	var findErr error
	if numID, parseErr := strconv.ParseUint(req.TaskID, 10, 64); parseErr == nil {
		findErr = h.DB.Where("id = ? OR rh_task_id = ?", numID, req.TaskID).First(&task).Error
	} else {
		findErr = h.DB.Where("rh_task_id = ?", req.TaskID).First(&task).Error
	}
	if findErr != nil {
		c.JSON(http.StatusOK, gin.H{"code": 807, "msg": "APIKEY_TASK_NOT_FOUND", "data": nil})
		return
	}

	if !isAdmin.(bool) && task.UserID != userID.(uint) {
		c.JSON(http.StatusOK, gin.H{"code": 403, "msg": "access denied", "data": nil})
		return
	}

	switch task.Status {
	case models.TaskStatusSuccess:
		// Parse results into legacy format
		var results []interface{}
		if task.Results != "" {
			json.Unmarshal([]byte(task.Results), &results)
		}
		c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "success", "data": results})

	case models.TaskStatusFailed, models.TaskStatusCancelled:
		failedReason := map[string]interface{}{
			"exception_message": task.ErrorMessage,
		}
		c.JSON(http.StatusOK, gin.H{"code": 805, "msg": "APIKEY_TASK_STATUS_ERROR", "data": gin.H{"failedReason": failedReason}})

	case models.TaskStatusRunning:
		data := gin.H{}
		if task.RhWssURL != "" {
			data["netWssUrl"] = task.RhWssURL
		}
		c.JSON(http.StatusOK, gin.H{"code": 804, "msg": "APIKEY_TASK_IS_RUNNING", "data": data})

	case models.TaskStatusPending, models.TaskStatusDispatched, models.TaskStatusQueued:
		c.JSON(http.StatusOK, gin.H{"code": 813, "msg": "APIKEY_TASK_IS_QUEUED", "data": nil})

	default:
		c.JSON(http.StatusOK, gin.H{"code": 804, "msg": "APIKEY_TASK_IS_RUNNING", "data": nil})
	}
}

// QueryTaskStatus handles the legacy POST /task/openapi/status endpoint.
// Returns simple status string: "QUEUED", "RUNNING", "FAILED", "SUCCESS".
func (h *ProxyHandler) QueryTaskStatus(c *gin.Context) {
	var req struct {
		ApiKey string `json:"apiKey"`
		TaskID string `json:"taskId" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": -1, "msg": "invalid request: " + err.Error(), "data": ""})
		return
	}

	userID, _ := c.Get("userID")
	isAdmin, _ := c.Get("isAdmin")

	var task models.Task
	var findErr error
	if numID, parseErr := strconv.ParseUint(req.TaskID, 10, 64); parseErr == nil {
		findErr = h.DB.Where("id = ? OR rh_task_id = ?", numID, req.TaskID).First(&task).Error
	} else {
		findErr = h.DB.Where("rh_task_id = ?", req.TaskID).First(&task).Error
	}
	if findErr != nil {
		c.JSON(http.StatusOK, gin.H{"code": 807, "msg": "APIKEY_TASK_NOT_FOUND", "data": ""})
		return
	}

	if !isAdmin.(bool) && task.UserID != userID.(uint) {
		c.JSON(http.StatusOK, gin.H{"code": 403, "msg": "access denied", "data": ""})
		return
	}

	// Map internal status to RunningHub status
	var status string
	switch task.Status {
	case models.TaskStatusSuccess:
		status = "SUCCESS"
	case models.TaskStatusFailed, models.TaskStatusCancelled:
		status = "FAILED"
	case models.TaskStatusRunning:
		status = "RUNNING"
	default:
		status = "QUEUED"
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "", "data": status})
}
