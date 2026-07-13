package config

import (
	"os"
	"strconv"
	"sync"
)

type ScheduleStrategy string

const (
	StrategyLeastLoaded ScheduleStrategy = "least-loaded"
	StrategyFillFirst   ScheduleStrategy = "fill-first"
)

type Config struct {
	Port             string
	DBDriver         string
	DBPath           string
	JWTSecret        string
	RHBaseURL        string
	BaseURL          string
	UploadDir        string
	OutputDir        string
	LocalMaxConc     int
	SchedulerTick    int
	PollInterval     int // seconds between each poll (default: 3)
	PollMaxAttempts  int // max poll attempts before timeout (default: 200 = 10min at 3s)
	AllowRegister    bool
	ScheduleStrategy ScheduleStrategy
	mu               sync.RWMutex
}

var AppConfig *Config

func Load() *Config {
	strategy := ScheduleStrategy(getEnv("SCHEDULE_STRATEGY", "least-loaded"))
	if strategy != StrategyLeastLoaded && strategy != StrategyFillFirst {
		strategy = StrategyLeastLoaded
	}

	localMaxConc := 4
	if v := os.Getenv("LOCAL_MAX_CONC"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			localMaxConc = n
		}
	}

	pollInterval := 3
	if v := os.Getenv("POLL_INTERVAL"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			pollInterval = n
		}
	}
	pollMaxAttempts := 200
	if v := os.Getenv("POLL_MAX_ATTEMPTS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			pollMaxAttempts = n
		}
	}

	cfg := &Config{
		Port:             getEnv("PORT", ":3060"),
		DBDriver:         getEnv("DB_DRIVER", "sqlite"),
		DBPath:           getEnv("DB_PATH", "./data/runninghub.db"),
		JWTSecret:        getEnv("JWT_SECRET", "change-me-in-production"),
		RHBaseURL:        getEnv("RH_BASE_URL", "https://www.runninghub.cn"),
		BaseURL:          getEnv("BASE_URL", "http://localhost:3060"),
		UploadDir:        getEnv("UPLOAD_DIR", "./uploads"),
		OutputDir:        getEnv("OUTPUT_DIR", "./output"),
		LocalMaxConc:     localMaxConc,
		SchedulerTick:    1000,
		PollInterval:     pollInterval,
		PollMaxAttempts:  pollMaxAttempts,
		AllowRegister:    getEnv("ALLOW_REGISTER", "false") == "true",
		ScheduleStrategy: strategy,
	}
	AppConfig = cfg
	return cfg
}

func (c *Config) GetStrategy() ScheduleStrategy {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.ScheduleStrategy
}

func (c *Config) SetStrategy(s ScheduleStrategy) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.ScheduleStrategy = s
}

func (c *Config) GetSchedulerTick() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.SchedulerTick
}

func (c *Config) SetSchedulerTick(ms int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.SchedulerTick = ms
}

func (c *Config) GetPollInterval() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.PollInterval
}

func (c *Config) SetPollInterval(seconds int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.PollInterval = seconds
}

func (c *Config) GetPollMaxAttempts() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.PollMaxAttempts
}

func (c *Config) SetPollMaxAttempts(n int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.PollMaxAttempts = n
}

func getEnv(key, fallback string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return fallback
}
