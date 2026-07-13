package handlers

import (
	"net/http"

	"runninghub-manager/apps"
	"runninghub-manager/config"
	"runninghub-manager/models"
	"runninghub-manager/pkg"
	"runninghub-manager/services"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type DashboardHandler struct {
	DB        *gorm.DB
	Scheduler *services.Scheduler
}

func NewDashboardHandler(db *gorm.DB, scheduler *services.Scheduler) *DashboardHandler {
	return &DashboardHandler{DB: db, Scheduler: scheduler}
}

func (h *DashboardHandler) GetStats(c *gin.Context) {
	var totalTasks, pendingTasks, runningTasks, successTasks, failedTasks int64

	h.DB.Model(&models.Task{}).Count(&totalTasks)
	h.DB.Model(&models.Task{}).Where("status = ?", models.TaskStatusPending).Count(&pendingTasks)
	h.DB.Model(&models.Task{}).Where("status IN ?", []models.TaskStatus{models.TaskStatusDispatched, models.TaskStatusRunning}).Count(&runningTasks)
	h.DB.Model(&models.Task{}).Where("status = ?", models.TaskStatusSuccess).Count(&successTasks)
	h.DB.Model(&models.Task{}).Where("status = ?", models.TaskStatusFailed).Count(&failedTasks)

	var totalKeys, activeKeys int64
	h.DB.Model(&models.ApiKey{}).Count(&totalKeys)
	h.DB.Model(&models.ApiKey{}).Where("is_active = ?", true).Count(&activeKeys)

	workerStatus := h.Scheduler.GetWorkerStatus()

	pkg.Success(c, gin.H{
		"totalTasks":       totalTasks,
		"pendingTasks":     pendingTasks,
		"runningTasks":     runningTasks,
		"successTasks":     successTasks,
		"failedTasks":      failedTasks,
		"totalKeys":        totalKeys,
		"activeKeys":       activeKeys,
		"workerStatus":     workerStatus,
		"scheduleStrategy": config.AppConfig.GetStrategy(),
		"schedulerTick":    config.AppConfig.GetSchedulerTick(),
	})
}

func (h *DashboardHandler) GetStrategy(c *gin.Context) {
	pkg.Success(c, gin.H{
		"strategy": config.AppConfig.GetStrategy(),
	})
}

func (h *DashboardHandler) SetStrategy(c *gin.Context) {
	var req struct {
		Strategy string `json:"strategy" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		pkg.Error(c, http.StatusBadRequest, "invalid request")
		return
	}

	strategy := config.ScheduleStrategy(req.Strategy)
	if strategy != config.StrategyLeastLoaded && strategy != config.StrategyFillFirst {
		pkg.Error(c, http.StatusBadRequest, "invalid strategy, must be 'least-loaded' or 'fill-first'")
		return
	}

	config.AppConfig.SetStrategy(strategy)
	pkg.Success(c, gin.H{
		"strategy": strategy,
	})
}

func (h *DashboardHandler) GetTick(c *gin.Context) {
	pkg.Success(c, gin.H{
		"tickMs": config.AppConfig.GetSchedulerTick(),
	})
}

func (h *DashboardHandler) SetTick(c *gin.Context) {
	var req struct {
		TickMs int `json:"tickMs" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		pkg.Error(c, http.StatusBadRequest, "invalid request")
		return
	}

	if req.TickMs < 100 || req.TickMs > 60000 {
		pkg.Error(c, http.StatusBadRequest, "tickMs must be between 100 and 60000")
		return
	}

	config.AppConfig.SetSchedulerTick(req.TickMs)
	h.Scheduler.SetTick(req.TickMs)
	pkg.Success(c, gin.H{
		"tickMs": req.TickMs,
	})
}

func (h *DashboardHandler) GetPollConfig(c *gin.Context) {
	pkg.Success(c, gin.H{
		"pollInterval":    config.AppConfig.GetPollInterval(),
		"pollMaxAttempts": config.AppConfig.GetPollMaxAttempts(),
	})
}

func (h *DashboardHandler) SetPollConfig(c *gin.Context) {
	var req struct {
		PollInterval    *int `json:"pollInterval"`
		PollMaxAttempts *int `json:"pollMaxAttempts"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		pkg.Error(c, http.StatusBadRequest, "invalid request")
		return
	}

	if req.PollInterval != nil {
		if *req.PollInterval < 1 || *req.PollInterval > 60 {
			pkg.Error(c, http.StatusBadRequest, "pollInterval must be between 1 and 60 seconds")
			return
		}
		config.AppConfig.SetPollInterval(*req.PollInterval)
	}

	if req.PollMaxAttempts != nil {
		if *req.PollMaxAttempts < 1 || *req.PollMaxAttempts > 10000 {
			pkg.Error(c, http.StatusBadRequest, "pollMaxAttempts must be between 1 and 10000")
			return
		}
		config.AppConfig.SetPollMaxAttempts(*req.PollMaxAttempts)
	}

	pkg.Success(c, gin.H{
		"pollInterval":    config.AppConfig.GetPollInterval(),
		"pollMaxAttempts": config.AppConfig.GetPollMaxAttempts(),
	})
}


func (h *DashboardHandler) GetApps(c *gin.Context) {
	appList := apps.List()
	result := make([]gin.H, 0, len(appList))
	for _, app := range appList {
		result = append(result, gin.H{
			"id":   app.ID(),
			"name": app.Name(),
		})
	}
	pkg.Success(c, result)
}