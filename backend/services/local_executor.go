package services

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"runninghub-manager/apps"
	"runninghub-manager/config"
	"runninghub-manager/models"

	"gorm.io/gorm"
)

// LocalExecutor processes tasks for self-built (local) custom apps.
type LocalExecutor struct {
	db        *gorm.DB
	quit      chan struct{}
	baseURL   string
	uploadDir string
	outputDir string
	maxConc   int
	sem       chan struct{}
}

// NewLocalExecutor creates a new local app executor.
func NewLocalExecutor(db *gorm.DB, baseURL, uploadDir, outputDir string, maxConc int) *LocalExecutor {
	if maxConc < 1 {
		maxConc = 4
	}
	return &LocalExecutor{
		db:        db,
		quit:      make(chan struct{}),
		baseURL:   baseURL,
		uploadDir: uploadDir,
		outputDir: outputDir,
		maxConc:   maxConc,
		sem:       make(chan struct{}, maxConc),
	}
}

// Start launches the local executor dispatch loop.
func (e *LocalExecutor) Start() {
	go e.dispatchLoop()
	log.Printf("[LocalExecutor] Started with maxConcurrency=%d, outputDir=%s", e.maxConc, e.outputDir)
}

// Stop signals the local executor to shut down.
func (e *LocalExecutor) Stop() {
	close(e.quit)
	log.Println("[LocalExecutor] Stopped")
}

func (e *LocalExecutor) dispatchLoop() {
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-e.quit:
			return
		case <-ticker.C:
			e.dispatch()
		}
	}
}

func (e *LocalExecutor) dispatch() {
	// Get pending local tasks
	var tasks []models.Task
	result := e.db.Where("status = ? AND is_local = ?", models.TaskStatusPending, true).
		Order("created_at ASC").
		Limit(20).
		Find(&tasks)

	if result.Error != nil || len(tasks) == 0 {
		return
	}

	for _, task := range tasks {
		// Check if we can acquire a semaphore slot (non-blocking)
		select {
		case e.sem <- struct{}{}:
			// Got a slot — dispatch this task
			now := time.Now()
			e.db.Model(&models.Task{}).Where("id = ?", task.ID).Updates(map[string]interface{}{
				"status":        models.TaskStatusDispatched,
				"dispatched_at": now,
			})
			go e.processTask(task.ID)
		default:
			// All slots full, stop dispatching
			return
		}
	}
}

func (e *LocalExecutor) processTask(taskID uint) {
	defer func() { <-e.sem }() // release semaphore

	// Load fresh task from DB
	var task models.Task
	if err := e.db.First(&task, "id = ?", taskID).Error; err != nil {
		log.Printf("[LocalExecutor] Failed to load task %d: %v", taskID, err)
		return
	}

	// Check if cancelled
	if task.Status == models.TaskStatusCancelled {
		log.Printf("[LocalExecutor] Task %d was cancelled, skipping", taskID)
		return
	}

	// Update status to RUNNING
	e.db.Model(&task).Update("status", models.TaskStatusRunning)

	// Look up the custom app
	app, ok := apps.Get(task.WebappID)
	if !ok {
		e.failTask(taskID, fmt.Sprintf("unknown custom app: %s", task.WebappID))
		return
	}

	// Parse nodeInfoList
	var nodeInfoList []apps.NodeInfo
	if err := json.Unmarshal([]byte(task.NodeInfoList), &nodeInfoList); err != nil {
		e.failTask(taskID, fmt.Sprintf("invalid nodeInfoList JSON: %v", err))
		return
	}

	// Resolve remote files: download from URL if file is not local
	for _, node := range nodeInfoList {
		fileName := string(node.FieldValue)
		if fileName == "" {
			continue
		}
		// Check if this fileName exists in uploads table as a remote file
		var upload models.Upload
		if err := e.db.Where("file_name = ? AND is_local = ?", fileName, false).First(&upload).Error; err != nil {
			continue // not a remote upload record, skip
		}
		// Download if not already cached locally
		localName := filepath.Base(fileName)
		localPath := filepath.Join(e.uploadDir, localName)
		if _, err := os.Stat(localPath); err == nil {
			continue // already exists locally
		}
		if upload.URL == "" {
			e.failTask(taskID, fmt.Sprintf("remote file %s has no download URL", fileName))
			return
		}
		log.Printf("[LocalExecutor] Downloading remote file: %s", fileName)
		if err := e.downloadFile(upload.URL, localPath); err != nil {
			e.failTask(taskID, fmt.Sprintf("failed to download remote file %s: %v", fileName, err))
			return
		}
		log.Printf("[LocalExecutor] Downloaded: %s -> %s", fileName, localPath)
	}

	// Create task-specific output directory
	taskOutputDir := filepath.Join(e.outputDir, fmt.Sprintf("task-%d", taskID))
	if err := os.MkdirAll(taskOutputDir, 0755); err != nil {
		e.failTask(taskID, fmt.Sprintf("failed to create output dir: %v", err))
		return
	}

	// Build input
	input := apps.AppInput{
		NodeInfoList: nodeInfoList,
		UploadDir:    e.uploadDir,
		OutputDir:    taskOutputDir,
		BaseURL:      e.baseURL,
	}

	// Execute with configurable timeout (default 60 minutes)
	timeout := time.Duration(config.AppConfig.GetLocalTaskTimeout()) * time.Minute
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	appResult, err := app.Execute(ctx, input)
	if err != nil {
		os.RemoveAll(taskOutputDir) // clean up partial output files
		e.failTask(taskID, err.Error())
		return
	}

	// Store results
	resultsJSON, _ := json.Marshal(appResult.Files)
	now := time.Now()
	e.db.Model(&models.Task{}).Where("id = ?", taskID).Updates(map[string]interface{}{
		"status":       models.TaskStatusSuccess,
		"results":      string(resultsJSON),
		"completed_at": now,
	})

	log.Printf("[LocalExecutor] Task %d completed successfully (%s)", taskID, app.ID())
}

func (e *LocalExecutor) failTask(taskID uint, errMsg string) {
	now := time.Now()
	e.db.Model(&models.Task{}).Where("id = ?", taskID).Updates(map[string]interface{}{
		"status":        models.TaskStatusFailed,
		"error_message": errMsg,
		"completed_at":  now,
	})
	log.Printf("[LocalExecutor] Task %d failed: %s", taskID, errMsg)
}

func (e *LocalExecutor) downloadFile(url, destPath string) error {
	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("HTTP GET: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
		return fmt.Errorf("create dir: %w", err)
	}

	f, err := os.Create(destPath)
	if err != nil {
		return fmt.Errorf("create file: %w", err)
	}
	defer f.Close()

	if _, err := io.Copy(f, resp.Body); err != nil {
		os.Remove(destPath)
		return fmt.Errorf("write file: %w", err)
	}
	return nil
}
