package handlers

import (
	"fmt"
	"log"
	"net/http"
	"strconv"

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
	if err := config.AppConfig.SaveSetting(h.DB, config.SettingKeyStrategy, string(strategy)); err != nil {
		log.Printf("warn: persist setting %s: %v", config.SettingKeyStrategy, err)
	}
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
	if err := config.AppConfig.SaveSetting(h.DB, config.SettingKeyTick, strconv.Itoa(req.TickMs)); err != nil {
		log.Printf("warn: persist setting %s: %v", config.SettingKeyTick, err)
	}
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
		if err := config.AppConfig.SaveSetting(h.DB, config.SettingKeyPollInterval, strconv.Itoa(*req.PollInterval)); err != nil {
			log.Printf("warn: persist setting %s: %v", config.SettingKeyPollInterval, err)
		}
	}

	if req.PollMaxAttempts != nil {
		if *req.PollMaxAttempts < 1 || *req.PollMaxAttempts > 10000 {
			pkg.Error(c, http.StatusBadRequest, "pollMaxAttempts must be between 1 and 10000")
			return
		}
		config.AppConfig.SetPollMaxAttempts(*req.PollMaxAttempts)
		if err := config.AppConfig.SaveSetting(h.DB, config.SettingKeyPollMaxAttempts, strconv.Itoa(*req.PollMaxAttempts)); err != nil {
			log.Printf("warn: persist setting %s: %v", config.SettingKeyPollMaxAttempts, err)
		}
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

func (h *DashboardHandler) GetChartData(c *gin.Context) {
	daysStr := c.DefaultQuery("days", "7")
	days := 7
	if d, err := strconv.Atoi(daysStr); err == nil && d > 0 && d <= 30 {
		days = d
	}

	// Task trend: daily counts for last N days
	type taskTrendRow struct {
		Date    string `json:"date"`
		Total   int64  `json:"total"`
		Success int64  `json:"success"`
		Failed  int64  `json:"failed"`
	}
	var taskTrend []taskTrendRow
	h.DB.Raw(`
		SELECT date(created_at) as date,
			COUNT(*) as total,
			SUM(CASE WHEN status = 'SUCCESS' THEN 1 ELSE 0 END) as success,
			SUM(CASE WHEN status = 'FAILED' THEN 1 ELSE 0 END) as failed
		FROM tasks
		WHERE created_at >= datetime('now', ?)
		GROUP BY date(created_at)
		ORDER BY date(created_at)
	`, fmt.Sprintf("-%d days", days)).Scan(&taskTrend)

	// API call trend: daily counts from request_logs
	type apiCallTrendRow struct {
		Date       string `json:"date"`
		Total      int64  `json:"total"`
		Proxy      int64  `json:"proxy"`
		Management int64  `json:"management"`
	}
	var apiCallTrend []apiCallTrendRow
	h.DB.Raw(`
		SELECT date(created_at) as date,
			COUNT(*) as total,
			SUM(CASE WHEN path LIKE '/task/openapi/%' OR path LIKE '/openapi/%' THEN 1 ELSE 0 END) as proxy,
			SUM(CASE WHEN path LIKE '/api/%' THEN 1 ELSE 0 END) as management
		FROM request_logs
		WHERE created_at >= datetime('now', ?)
		GROUP BY date(created_at)
		ORDER BY date(created_at)
	`, fmt.Sprintf("-%d days", days)).Scan(&apiCallTrend)

	// Hourly breakdown for today
	type hourlyRow struct {
		Hour     string `json:"hour"`
		Tasks    int64  `json:"tasks"`
		ApiCalls int64  `json:"apiCalls"`
	}

	var hourlyTasks []struct {
		Hour  string
		Count int64
	}
	h.DB.Raw(`
		SELECT strftime('%H:00', created_at) as hour, COUNT(*) as count
		FROM tasks
		WHERE date(created_at) = date('now')
		GROUP BY strftime('%H', created_at)
	`).Scan(&hourlyTasks)

	var hourlyAPICalls []struct {
		Hour  string
		Count int64
	}
	h.DB.Raw(`
		SELECT strftime('%H:00', created_at) as hour, COUNT(*) as count
		FROM request_logs
		WHERE date(created_at) = date('now')
		GROUP BY strftime('%H', created_at)
	`).Scan(&hourlyAPICalls)

	// Merge hourly data into 24-hour slots
	taskMap := make(map[string]int64)
	for _, t := range hourlyTasks {
		taskMap[t.Hour] = t.Count
	}
	apiMap := make(map[string]int64)
	for _, a := range hourlyAPICalls {
		apiMap[a.Hour] = a.Count
	}

	hourlyToday := make([]hourlyRow, 24)
	for i := 0; i < 24; i++ {
		h := fmt.Sprintf("%02d:00", i)
		hourlyToday[i] = hourlyRow{
			Hour:     h,
			Tasks:    taskMap[h],
			ApiCalls: apiMap[h],
		}
	}

	pkg.Success(c, gin.H{
		"taskTrend":    taskTrend,
		"apiCallTrend": apiCallTrend,
		"hourlyToday":  hourlyToday,
	})
}

func (h *DashboardHandler) GetRequestLogs(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "50"))
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 200 {
		pageSize = 50
	}

	method := c.Query("method")
	path := c.Query("path")

	query := h.DB.Model(&models.RequestLog{})

	if method != "" {
		query = query.Where("method = ?", method)
	}
	if path != "" {
		query = query.Where("path LIKE ?", path+"%")
	}

	var total int64
	query.Count(&total)

	var logs []models.RequestLog
	query.Order("created_at DESC").
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		Find(&logs)

	pkg.Success(c, gin.H{
		"logs":     logs,
		"total":    total,
		"page":     page,
		"pageSize": pageSize,
	})
}
