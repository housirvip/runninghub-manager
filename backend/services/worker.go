package services

import (
	"encoding/json"
	"fmt"
	"log"
	"sync/atomic"
	"time"

	"runninghub-manager/config"
	"runninghub-manager/models"

	"gorm.io/gorm"
)

type Worker struct {
	apiKeyID uint
	apiKey   string
	name     string
	maxConc  int
	inflight atomic.Int32    // tracks total tasks owned by this worker (queued + running)
	sem      chan struct{}   // limits actual concurrent RunningHub calls
	taskChan chan uint       // receives task IDs from scheduler
	quit     chan struct{}
	db       *gorm.DB
	rhClient *RHClient
}

func NewWorker(apiKeyID uint, apiKey string, name string, maxConc int, db *gorm.DB, rhClient *RHClient) *Worker {
	return &Worker{
		apiKeyID: apiKeyID,
		apiKey:   apiKey,
		name:     name,
		maxConc:  maxConc,
		sem:      make(chan struct{}, maxConc),
		taskChan: make(chan uint, maxConc),
		quit:     make(chan struct{}),
		db:       db,
		rhClient: rhClient,
	}
}

func (w *Worker) Start() {
	go w.run()
	log.Printf("[Worker:%s] Started with concurrency=%d", w.name, w.maxConc)
}

func (w *Worker) Stop() {
	close(w.quit)
	log.Printf("[Worker:%s] Stopped", w.name)
}

// Available returns true if the worker can accept more tasks.
// It checks the inflight counter which tracks tasks from dispatch to completion.
func (w *Worker) Available() bool {
	return int(w.inflight.Load()) < w.maxConc
}

// CurrentLoad returns the number of tasks currently owned by this worker.
func (w *Worker) CurrentLoad() int {
	return int(w.inflight.Load())
}

// Dispatch sends a task to this worker. Must only be called when Available() is true.
// Increments inflight immediately so subsequent Available() checks reflect accurately.
func (w *Worker) Dispatch(taskID uint) bool {
	w.inflight.Add(1)
	select {
	case w.taskChan <- taskID:
		return true
	default:
		// Channel full (shouldn't happen if Available() was checked)
		w.inflight.Add(-1)
		return false
	}
}

func (w *Worker) run() {
	for {
		select {
		case <-w.quit:
			return
		case taskID := <-w.taskChan:
			// Acquire semaphore
			select {
			case <-w.quit:
				return
			case w.sem <- struct{}{}:
				go w.processTask(taskID)
			}
		}
	}
}

func (w *Worker) processTask(taskID uint) {
	defer func() { <-w.sem; w.inflight.Add(-1) }()

	// Load task from DB
	var task models.Task
	if err := w.db.First(&task, "id = ?", taskID).Error; err != nil {
		log.Printf("[Worker:%s] Failed to load task %d: %v", w.name, taskID, err)
		return
	}

	// Check if task was cancelled while in queue
	if task.Status == models.TaskStatusCancelled {
		log.Printf("[Worker:%s] Task %d was cancelled, skipping", w.name, taskID)
		return
	}

	// Parse nodeInfoList
	var nodeInfoList json.RawMessage
	if err := json.Unmarshal([]byte(task.NodeInfoList), &nodeInfoList); err != nil {
		w.failTask(taskID, 0, "invalid nodeInfoList JSON: "+err.Error())
		return
	}

	// Create task on RunningHub with retry for transient errors
	req := CreateTaskRequest{
		WebappID:       task.WebappID,
		NodeInfoList:   nodeInfoList,
		WebhookURL:     task.WebhookURL,
		InstanceType:   task.InstanceType,
		AccessPassword: task.AccessPassword,
	}

	maxRetries := 3
	var resp *CreateTaskResponse
	var lastErr error

	for attempt := 0; attempt <= maxRetries; attempt++ {
		// Check cancellation before each attempt
		if attempt > 0 {
			var check models.Task
			if err := w.db.Select("status").First(&check, "id = ?", taskID).Error; err == nil {
				if check.Status == models.TaskStatusCancelled {
					log.Printf("[Worker:%s] Task %d cancelled during retry", w.name, taskID)
					return
				}
			}
		}

		var err error
		resp, err = w.rhClient.CreateTask(w.apiKey, req)
		if err != nil {
			lastErr = err
			// Network error — retry
			if attempt < maxRetries {
				log.Printf("[Worker:%s] Task %d network error (attempt %d/%d): %v", w.name, taskID, attempt+1, maxRetries+1, err)
				time.Sleep(time.Duration(5*(attempt+1)) * time.Second)
				continue
			}
			break
		}

		if resp.Code == 0 && resp.Data != nil {
			// Success
			break
		}

		// Check if this error code is retryable
		if isRetryableCreateError(resp.Code) {
			lastErr = fmt.Errorf("[%d] %s", resp.Code, resp.Msg)
			delay := retryDelay(resp.Code, attempt)
			if attempt < maxRetries {
				log.Printf("[Worker:%s] Task %d got retryable error %d (attempt %d/%d), retrying in %v: %s",
					w.name, taskID, resp.Code, attempt+1, maxRetries+1, delay, resp.Msg)
				time.Sleep(delay)
				continue
			}
		} else {
			// Non-retryable error — fail immediately with Chinese translation
			w.failTask(taskID, resp.Code, translateError(resp.Code, resp.Msg))
			return
		}
	}

	// All retries exhausted
	if resp == nil || resp.Code != 0 || resp.Data == nil {
		errMsg := "重试耗尽仍然失败"
		if resp != nil && resp.Code > 0 {
			errMsg = translateError(resp.Code, resp.Msg)
			w.failTask(taskID, resp.Code, errMsg)
		} else {
			if lastErr != nil {
				errMsg = "网络请求失败: " + lastErr.Error()
			}
			w.failTask(taskID, 0, errMsg)
		}
		return
	}

	// Update task with RunningHub response
	now := time.Now()
	w.db.Model(&models.Task{}).Where("id = ?", taskID).Updates(map[string]interface{}{
		"status":        models.TaskStatusRunning,
		"rh_task_id":    resp.Data.TaskID,
		"rh_client_id":  resp.Data.ClientID,
		"rh_wss_url":    resp.Data.NetWssURL,
		"prompt_tips":   resp.Data.PromptTips,
		"dispatched_at": now,
	})

	log.Printf("[Worker:%s] Task %d dispatched to RunningHub (rh_task_id=%s)", w.name, taskID, resp.Data.TaskID)

	// Poll for results
	w.pollTaskResult(taskID, resp.Data.TaskID)
}

// isRetryableCreateError returns true for error codes that are transient and should be retried.
func isRetryableCreateError(code int) bool {
	switch code {
	case 415: // TASK_INSTANCE_MAXED — 机器数不足
		return true
	case 421: // TASK_QUEUE_MAXED — 并发上限
		return true
	case 500: // UNKNOWN_ERROR
		return true
	case 1003: // Rate limit exceeded
		return true
	case 1005: // Internal server error
		return true
	case 1006: // Task execution timed out
		return true
	case 1010: // Service unavailable
		return true
	case 1011: // System busy
		return true
	case 1012: // Service response exception
		return true
	}
	return false
}

// retryDelay returns the appropriate wait time based on error code and attempt number.
func retryDelay(code int, attempt int) time.Duration {
	base := 5 * time.Second
	switch code {
	case 415: // 机器不足，等久一点
		base = 30 * time.Second
	case 421: // 并发上限，等一会
		base = 15 * time.Second
	case 1003: // 频率超限
		base = 10 * time.Second
	case 1011: // 系统繁忙
		base = 20 * time.Second
	}
	// Exponential backoff: base * (attempt+1)
	return base * time.Duration(attempt+1)
}

func (w *Worker) pollTaskResult(platformTaskID uint, rhTaskID string) {
	interval := time.Duration(config.AppConfig.GetPollInterval()) * time.Second
	maxAttempts := config.AppConfig.GetPollMaxAttempts()

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	attempts := 0
	for {
		select {
		case <-w.quit:
			return
		case <-ticker.C:
			attempts++

			// Check max attempts
			if attempts > maxAttempts {
				w.failTask(platformTaskID, 0, fmt.Sprintf("任务轮询超时（已查询 %d 次，间隔 %ds）", maxAttempts, int(interval.Seconds())))
				return
			}

			// Check if cancelled locally
			var task models.Task
			if err := w.db.Select("status").First(&task, "id = ?", platformTaskID).Error; err == nil {
				if task.Status == models.TaskStatusCancelled {
					log.Printf("[Worker:%s] Task %d was cancelled, stopping poll", w.name, platformTaskID)
					return
				}
			}

			result, err := w.rhClient.QueryTask(w.apiKey, rhTaskID)
			if err != nil {
				log.Printf("[Worker:%s] Failed to query task %d (attempt %d/%d): %v", w.name, platformTaskID, attempts, maxAttempts, err)
				continue
			}

			switch result.Status {
			case "SUCCESS":
				now := time.Now()
				updates := map[string]interface{}{
					"status":       models.TaskStatusSuccess,
					"completed_at": now,
				}
				if result.Results != nil {
					updates["results"] = string(result.Results)
				}
				w.db.Model(&models.Task{}).Where("id = ?", platformTaskID).Updates(updates)
				log.Printf("[Worker:%s] Task %d completed successfully (polled %d times)", w.name, platformTaskID, attempts)
				return

			case "FAILED":
				now := time.Now()
				errMsg := result.ErrorMessage
				errCode := result.ErrorCode
				if errCode != "" {
					var codeInt int
					fmt.Sscanf(errCode, "%d", &codeInt)
					if codeInt > 0 {
						errMsg = fmt.Sprintf("[%d] %s", codeInt, translateError(codeInt, errMsg))
					} else {
						errMsg = fmt.Sprintf("[%s] %s", errCode, errMsg)
					}
				} else if errMsg == "" {
					errMsg = "任务在 RunningHub 上执行失败"
				}
				w.db.Model(&models.Task{}).Where("id = ?", platformTaskID).Updates(map[string]interface{}{
					"status":        models.TaskStatusFailed,
					"error_message": errMsg,
					"completed_at":  now,
				})
				log.Printf("[Worker:%s] Task %d failed: %s", w.name, platformTaskID, errMsg)
				return

			case "RUNNING", "QUEUED", "CREATE":
				continue

			default:
				log.Printf("[Worker:%s] Task %d unknown status: %s", w.name, platformTaskID, result.Status)
				continue
			}
		}
	}
}

// errorCodeMessages maps RunningHub error codes to Chinese descriptions.
var errorCodeMessages = map[int]string{
	301:  "参数错误：必填参数缺失或类型不符",
	380:  "工作流不存在：指定的工作流 ID 无效",
	412:  "API 路径错误：请检查接口 URL 是否正确",
	415:  "机器数不足：资源紧张，请稍后重试",
	416:  "钱包余额不足：请充值后重试",
	421:  "并发上限：当前 API Key 并发已达上限",
	423:  "未找到指定任务：任务 ID 错误或已被清理",
	433:  "工作流校验未通过：节点参数或连接逻辑错误",
	435:  "未找到 API 实例：48G显存机器需添加 instanceType=plus",
	436:  "独占会员已到期：独占资源服务已到期",
	500:  "未知错误：服务端异常",
	801:  "免费用户不支持 API Key：请升级账户",
	802:  "API Key 未授权或已失效：请检查密钥",
	803:  "nodeInfoList 不匹配：节点 ID 或字段名与工作流不一致",
	804:  "任务正在运行中",
	805:  "任务状态异常：任务可能已被中断或取消",
	806:  "未找到对应用户：Key 关联的用户信息不存在",
	807:  "未找到对应任务：无法查询到该任务记录",
	808:  "文件上传失败：存储服务异常或网络中断",
	809:  "文件大小超出限制",
	810:  "工作流未保存或未运行：请先在平台保存并手动运行一次",
	811:  "企业版 API Key 无效",
	812:  "企业版余额不足",
	813:  "任务已排队：任务已受理，无需重试",
	901:  "WebApp 不存在：应用 ID 错误",
	1000: "未知错误：请重试或联系支持",
	1001: "请求链接无效：请检查调用链接",
	1002: "API Key 无效：请检查凭据",
	1003: "请求频率超限：请降低请求速度",
	1004: "任务不存在或已过期：请检查任务 ID",
	1005: "系统内部错误：请稍后重试",
	1006: "任务执行超时：请重试",
	1007: "参数校验失败：请检查输入参数",
	1008: "文件大小超出限制：请压缩后重试",
	1009: "请求方法不支持：请查阅文档确认",
	1010: "服务暂不可用：请稍后重试",
	1011: "系统繁忙：请稍后重试",
	1012: "上游服务响应异常：请联系支持或稍后重试",
	1013: "文件处理失败：请检查链接或重新上传",
	1014: "访问被拒绝：标准模型 API 仅限企业级共享 Key 调用",
	1015: "生成失败：请重试",
	1101: "节点信息异常：工作流节点数据解析错误",
	1501: "内容审核未通过：请修改提示词或图片",
	1504: "模型响应超时：请稍后重试",
	1505: "禁止生成真人照片：请修改提示词或参考图",
}

// translateError converts a RunningHub error code + original msg to a Chinese message.
func translateError(code int, originalMsg string) string {
	if msg, ok := errorCodeMessages[code]; ok {
		return msg
	}
	// Fallback: return original
	if originalMsg != "" {
		return originalMsg
	}
	return "未知错误"
}

func (w *Worker) failTask(taskID uint, code int, errMsg string) {
	now := time.Now()
	if code > 0 {
		translated := translateError(code, errMsg)
		errMsg = fmt.Sprintf("[%d] %s", code, translated)
	}
	w.db.Model(&models.Task{}).Where("id = ?", taskID).Updates(map[string]interface{}{
		"status":        models.TaskStatusFailed,
		"error_message": errMsg,
		"completed_at":  now,
	})
	log.Printf("[Worker:%s] Task %d failed: %s", w.name, taskID, errMsg)
}
