package models

import "time"

type ApiKey struct {
	ID             uint      `gorm:"primarykey" json:"id"`
	Name           string    `gorm:"size:128;not null" json:"name"`
	ApiKey         string    `gorm:"size:255;not null" json:"apiKey"`
	BaseURL        string    `gorm:"size:255;not null;default:https://www.runninghub.cn" json:"baseUrl"`
	MaxConcurrency int       `gorm:"default:3" json:"maxConcurrency"`
	IsActive       bool      `gorm:"default:true" json:"isActive"`
	CreatedAt      time.Time `json:"createdAt"`
	UpdatedAt      time.Time `json:"updatedAt"`
}
