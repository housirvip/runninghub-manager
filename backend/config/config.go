package config

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log"
	"os"
	"strconv"
	"sync"

	"github.com/joho/godotenv"
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
	DBLogLevel       string // "silent", "error", "warn", "info"
	JWTSecret        string
	RHBaseURL        string
	BaseURL          string
	UploadDir        string
	OutputDir        string
	MaxUploadSize    int64 // bytes, default 50MB
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
	// Load .env file if present (does not override existing env vars)
	// Try current dir first, then parent dir (for running from backend/ subdirectory)
	if err := godotenv.Load(); err != nil {
		_ = godotenv.Load("../.env")
	}

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

	// Max upload size (default 50MB)
	maxUploadSize := int64(50 << 20)
	if v := os.Getenv("MAX_UPLOAD_SIZE_MB"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			maxUploadSize = int64(n) << 20
		}
	}

	// JWT Secret: generate random if not set (warn in dev, fatal hint for prod)
	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" || jwtSecret == "change-me-in-production" {
		jwtSecret = generateRandomSecret()
		log.Printf("⚠️  JWT_SECRET not set, generated random secret (tokens invalidate on restart). Set JWT_SECRET env for persistence.")
	}

	cfg := &Config{
		Port:             getEnv("PORT", ":3060"),
		DBDriver:         getEnv("DB_DRIVER", "sqlite"),
		DBPath:           getEnv("DB_PATH", "./data/runninghub.db"),
		DBLogLevel:       getEnv("DB_LOG_LEVEL", "warn"),
		JWTSecret:        jwtSecret,
		RHBaseURL:        getEnv("RH_BASE_URL", "https://www.runninghub.cn"),
		BaseURL:          getEnv("BASE_URL", "http://localhost:3060"),
		UploadDir:        getEnv("UPLOAD_DIR", "./uploads"),
		OutputDir:        getEnv("OUTPUT_DIR", "./output"),
		MaxUploadSize:    maxUploadSize,
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

func generateRandomSecret() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("fallback-%d", os.Getpid())
	}
	return hex.EncodeToString(b)
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
