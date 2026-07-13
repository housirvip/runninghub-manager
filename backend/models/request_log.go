package models

import "time"

type RequestLog struct {
	ID         uint      `gorm:"primarykey;autoIncrement" json:"id"`
	Method     string    `gorm:"size:10;not null;index" json:"method"`
	Path       string    `gorm:"size:512;not null" json:"path"`
	StatusCode int       `gorm:"not null" json:"statusCode"`
	Latency    int64     `json:"latency"` // milliseconds
	ClientIP   string    `gorm:"size:45" json:"clientIP"`
	UserID     *uint     `gorm:"index" json:"userId"`
	Username   string    `gorm:"size:64" json:"username"`
	ApiKeyID   *uint     `gorm:"index" json:"apiKeyId"`
	Error      string    `gorm:"size:512" json:"error"`
	CreatedAt  time.Time `gorm:"index" json:"createdAt"`
}
