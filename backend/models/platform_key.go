package models

import "time"

type PlatformKey struct {
	ID         uint       `gorm:"primarykey" json:"id"`
	UserID     uint       `gorm:"index" json:"userId"`
	Name       string     `gorm:"size:128;not null" json:"name"`
	Key        string     `gorm:"size:64;uniqueIndex;not null" json:"key"`
	IsActive   bool       `gorm:"default:true" json:"isActive"`
	ExpiresAt  *time.Time `json:"expiresAt"`
	LastUsedAt *time.Time `json:"lastUsedAt"`
	CreatedAt  time.Time  `json:"createdAt"`
	UpdatedAt  time.Time  `json:"updatedAt"`
}
