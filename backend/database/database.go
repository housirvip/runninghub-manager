package database

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"runninghub-manager/config"
	"runninghub-manager/models"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var DB *gorm.DB

func Init(cfg *config.Config) *gorm.DB {
	var dialector gorm.Dialector

	switch cfg.DBDriver {
	case "sqlite":
		// Ensure directory exists
		dir := filepath.Dir(cfg.DBPath)
		if err := os.MkdirAll(dir, 0755); err != nil {
			log.Fatalf("Failed to create database directory: %v", err)
		}
		dsn := fmt.Sprintf("%s?_journal_mode=WAL&_busy_timeout=5000", cfg.DBPath)
		dialector = sqlite.Open(dsn)
	default:
		log.Fatalf("Unsupported database driver: %s", cfg.DBDriver)
	}

	logLevel := logger.Warn
	switch cfg.DBLogLevel {
	case "silent":
		logLevel = logger.Silent
	case "error":
		logLevel = logger.Error
	case "info":
		logLevel = logger.Info
	}

	db, err := gorm.Open(dialector, &gorm.Config{
		Logger: logger.Default.LogMode(logLevel),
	})
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}

	// Auto migrate
	if err := db.AutoMigrate(&models.User{}, &models.ApiKey{}, &models.Task{}, &models.PlatformKey{}); err != nil {
		log.Fatalf("Failed to migrate database: %v", err)
	}

	DB = db
	return db
}
