package config

import (
	"log"
	"strconv"

	"runninghub-manager/models"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Persisted runtime setting keys. Stored as rows in the settings table so the
// four tunable runtime values survive restart; env/.env remains the fallback.
const (
	SettingKeyStrategy         = "schedule_strategy"
	SettingKeyTick             = "scheduler_tick"
	SettingKeyPollInterval     = "poll_interval"
	SettingKeyPollMaxAttempts  = "poll_max_attempts"
	SettingKeyLocalTaskTimeout = "local_task_timeout"
)

// LoadFromDB loads all persisted runtime settings from the database and applies
// them to the in-memory config, overriding the env defaults loaded by Load().
// Unknown keys are ignored (forward-compatible). Invalid values are skipped
// with a warning rather than failing the whole load; only a DB query error is
// returned as an error.
func (c *Config) LoadFromDB(db *gorm.DB) error {
	var rows []models.Setting
	if err := db.Find(&rows).Error; err != nil {
		return err
	}

	for _, row := range rows {
		switch row.Key {
		case SettingKeyStrategy:
			s := ScheduleStrategy(row.Value)
			if s != StrategyLeastLoaded && s != StrategyFillFirst {
				log.Printf("warn: setting %s: invalid value %q, skipping", row.Key, row.Value)
				continue
			}
			c.SetStrategy(s)
		case SettingKeyTick:
			ms, err := strconv.Atoi(row.Value)
			if err != nil {
				log.Printf("warn: setting %s: invalid integer %q, skipping: %v", row.Key, row.Value, err)
				continue
			}
			if ms < 100 || ms > 60000 {
				log.Printf("warn: setting %s: value %d out of range [100,60000], skipping", row.Key, ms)
				continue
			}
			c.SetSchedulerTick(ms)
		case SettingKeyPollInterval:
			n, err := strconv.Atoi(row.Value)
			if err != nil {
				log.Printf("warn: setting %s: invalid integer %q, skipping: %v", row.Key, row.Value, err)
				continue
			}
			if n < 1 || n > 60 {
				log.Printf("warn: setting %s: value %d out of range [1,60], skipping", row.Key, n)
				continue
			}
			c.SetPollInterval(n)
		case SettingKeyPollMaxAttempts:
			n, err := strconv.Atoi(row.Value)
			if err != nil {
				log.Printf("warn: setting %s: invalid integer %q, skipping: %v", row.Key, row.Value, err)
				continue
			}
			if n < 1 || n > 10000 {
				log.Printf("warn: setting %s: value %d out of range [1,10000], skipping", row.Key, n)
				continue
			}
			c.SetPollMaxAttempts(n)
		case SettingKeyLocalTaskTimeout:
			n, err := strconv.Atoi(row.Value)
			if err != nil {
				log.Printf("warn: setting %s: invalid integer %q, skipping: %v", row.Key, row.Value, err)
				continue
			}
			if n < 1 || n > 1440 {
				log.Printf("warn: setting %s: value %d out of range [1,1440], skipping", row.Key, n)
				continue
			}
			c.SetLocalTaskTimeout(n)
		default:
			// Unknown key — skip to stay forward-compatible.
		}
	}
	return nil
}

// SaveSetting upserts a single key/value setting row. Used by the dashboard
// Set* handlers to persist runtime changes so they survive restart.
func (c *Config) SaveSetting(db *gorm.DB, key, value string) error {
	return db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "key"}},
		DoUpdates: clause.AssignmentColumns([]string{"value"}),
	}).Create(&models.Setting{Key: key, Value: value}).Error
}
