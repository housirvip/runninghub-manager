package services

import (
	"log"
	"sync"
	"sync/atomic"
	"time"

	"runninghub-manager/config"
	"runninghub-manager/models"

	"gorm.io/gorm"
)

type Scheduler struct {
	workers    map[uint]*Worker
	db         *gorm.DB
	rhClient   *RHClient
	mu         sync.RWMutex
	quit       chan struct{}
	tick       time.Duration
	tickUpdate chan time.Duration
	counter    atomic.Uint64
}

func NewScheduler(db *gorm.DB, rhClient *RHClient, tickMs int) *Scheduler {
	return &Scheduler{
		workers:    make(map[uint]*Worker),
		db:         db,
		rhClient:   rhClient,
		quit:       make(chan struct{}),
		tick:       time.Duration(tickMs) * time.Millisecond,
		tickUpdate: make(chan time.Duration, 1),
	}
}

func (s *Scheduler) Start() {
	// Recover orphaned tasks from previous crash/restart
	s.recoverOrphanedTasks()

	// Load all active API keys and start workers
	var keys []models.ApiKey
	s.db.Where("is_active = ?", true).Find(&keys)

	for i := range keys {
		s.addWorkerInternal(&keys[i])
	}

	// Start dispatch loop
	go s.dispatchLoop()

	// Resume polling for RUNNING tasks that have rh_task_id
	go s.resumeRunningTasks()

	log.Printf("[Scheduler] Started with %d workers", len(s.workers))
}

func (s *Scheduler) Stop() {
	close(s.quit)
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, w := range s.workers {
		w.Stop()
	}
	log.Println("[Scheduler] Stopped")
}

func (s *Scheduler) AddWorker(apiKey *models.ApiKey) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.addWorkerInternal(apiKey)
}

func (s *Scheduler) RemoveWorker(apiKeyID uint) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if w, ok := s.workers[apiKeyID]; ok {
		w.Stop()
		delete(s.workers, apiKeyID)
	}
}

func (s *Scheduler) UpdateWorker(apiKey *models.ApiKey) {
	s.RemoveWorker(apiKey.ID)
	if apiKey.IsActive {
		s.AddWorker(apiKey)
	}
}

func (s *Scheduler) GetWorkerStatus() []map[string]interface{} {
	s.mu.RLock()
	defer s.mu.RUnlock()

	status := make([]map[string]interface{}, 0, len(s.workers))
	for _, w := range s.workers {
		status = append(status, map[string]interface{}{
			"apiKeyId":       w.apiKeyID,
			"name":           w.name,
			"maxConcurrency": w.maxConc,
			"currentTasks":   w.CurrentLoad(),
			"available":      w.Available(),
		})
	}
	return status
}

func (s *Scheduler) addWorkerInternal(apiKey *models.ApiKey) {
	// Each worker gets an RHClient with the key's specific base URL
	keyClient := s.rhClient.ForBaseURL(apiKey.BaseURL)
	w := NewWorker(apiKey.ID, apiKey.ApiKey, apiKey.Name, apiKey.MaxConcurrency, s.db, keyClient)
	s.workers[apiKey.ID] = w
	w.Start()
}

func (s *Scheduler) dispatchLoop() {
	ticker := time.NewTicker(s.tick)
	defer ticker.Stop()

	for {
		select {
		case <-s.quit:
			return
		case newTick := <-s.tickUpdate:
			ticker.Stop()
			s.tick = newTick
			ticker = time.NewTicker(newTick)
			log.Printf("[Scheduler] Tick updated to %v", newTick)
		case <-ticker.C:
			s.dispatch()
		}
	}
}

// SetTick dynamically updates the scheduler's dispatch interval.
func (s *Scheduler) SetTick(ms int) {
	if ms < 100 {
		ms = 100 // minimum 100ms
	}
	if ms > 60000 {
		ms = 60000 // maximum 60s
	}
	newTick := time.Duration(ms) * time.Millisecond
	select {
	case s.tickUpdate <- newTick:
	default:
		// Channel already has a pending update, drain and resend
		select {
		case <-s.tickUpdate:
		default:
		}
		s.tickUpdate <- newTick
	}
}

// GetTick returns current tick in milliseconds.
func (s *Scheduler) GetTick() int {
	return int(s.tick / time.Millisecond)
}

func (s *Scheduler) dispatch() {
	// Get pending tasks
	var tasks []models.Task
	result := s.db.Where("status = ? AND is_local = ?", models.TaskStatusPending, false).
		Order("created_at ASC").
		Limit(50).
		Find(&tasks)

	if result.Error != nil || len(tasks) == 0 {
		return
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	if len(s.workers) == 0 {
		return
	}

	// Build list of available workers
	availableWorkers := make([]*Worker, 0)
	for _, w := range s.workers {
		if w.Available() {
			availableWorkers = append(availableWorkers, w)
		}
	}

	if len(availableWorkers) == 0 {
		return
	}

	for i := range tasks {
		if len(availableWorkers) == 0 {
			break
		}

		// Select worker based on strategy
		idx := s.selectWorker(availableWorkers)
		worker := availableWorkers[idx]

		// Dispatch to worker (increments inflight counter atomically)
		if worker.Dispatch(tasks[i].ID) {
			s.db.Model(&tasks[i]).Updates(map[string]interface{}{
				"status":     models.TaskStatusDispatched,
				"api_key_id": worker.apiKeyID,
			})
			log.Printf("[Scheduler] Dispatched task %d to worker %s (strategy=%s)",
				tasks[i].ID, worker.name, config.AppConfig.GetStrategy())
		}

		// Re-check: remove full workers from available list
		if !worker.Available() {
			availableWorkers = append(availableWorkers[:idx], availableWorkers[idx+1:]...)
		}
	}
}

// selectWorker picks a worker index from availableWorkers based on current strategy.
func (s *Scheduler) selectWorker(workers []*Worker) int {
	strategy := config.AppConfig.GetStrategy()

	switch strategy {
	case config.StrategyFillFirst:
		// Pick the worker with the MOST load (fill it up before moving to next)
		maxLoad := -1
		maxIdx := 0
		for i, w := range workers {
			load := w.CurrentLoad()
			if load > maxLoad {
				maxLoad = load
				maxIdx = i
			}
		}
		return maxIdx

	case config.StrategyLeastLoaded:
		// Pick the worker with the LEAST load
		minLoad := int(^uint(0) >> 1) // max int
		minIdx := 0
		for i, w := range workers {
			load := w.CurrentLoad()
			if load < minLoad {
				minLoad = load
				minIdx = i
			}
		}
		return minIdx

	default:
		// Fallback: round-robin
		return int(s.counter.Add(1)-1) % len(workers)
	}
}

// recoverOrphanedTasks resets DISPATCHED tasks back to PENDING on startup.
// DISPATCHED means "sent to worker channel but not yet submitted to RunningHub" —
// after a restart, the worker channel is gone so these need re-dispatch.
func (s *Scheduler) recoverOrphanedTasks() {
	result := s.db.Model(&models.Task{}).
		Where("status = ?", models.TaskStatusDispatched).
		Update("status", models.TaskStatusPending)
	if result.RowsAffected > 0 {
		log.Printf("[Scheduler] Recovered %d orphaned DISPATCHED tasks → PENDING", result.RowsAffected)
	}
}

// resumeRunningTasks resumes polling for tasks that were RUNNING before the restart.
// These tasks have already been submitted to RunningHub (have rh_task_id) and just
// need their status polled to completion.
func (s *Scheduler) resumeRunningTasks() {
	var tasks []models.Task
	s.db.Where("status = ? AND rh_task_id != '' AND is_local = ?", models.TaskStatusRunning, false).Find(&tasks)
	if len(tasks) == 0 {
		return
	}

	log.Printf("[Scheduler] Resuming poll for %d RUNNING tasks from previous session", len(tasks))

	for _, task := range tasks {
		// Find the worker for this task's API key
		if task.ApiKeyID == nil {
			continue
		}

		s.mu.RLock()
		w, exists := s.workers[*task.ApiKeyID]
		s.mu.RUnlock()

		if !exists {
			// No worker for this key (key might have been deactivated)
			// Try any available worker with the same API key from DB
			var apiKey models.ApiKey
			if err := s.db.First(&apiKey, *task.ApiKeyID).Error; err != nil {
				log.Printf("[Scheduler] Cannot resume task %d: API key %d not found", task.ID, *task.ApiKeyID)
				continue
			}
			// Mark as failed since we can't poll without the right key's worker
			s.db.Model(&task).Updates(map[string]interface{}{
				"status":        models.TaskStatusFailed,
				"error_message": "服务重启后无法恢复：关联的 API Key 已不可用",
			})
			continue
		}

		// Resume polling in a goroutine using the worker's client and API key
		taskID := task.ID
		rhTaskID := task.RhTaskID
		w.wg.Add(1)
		w.inflight.Add(1)
		go func() {
			w.sem <- struct{}{} // acquire semaphore slot
			w.pollTaskResult(taskID, rhTaskID)
			<-w.sem
			w.inflight.Add(-1)
			w.wg.Done()
		}()
	}
}
