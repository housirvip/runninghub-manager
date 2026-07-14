package models

import "time"

type Upload struct {
	ID           uint      `gorm:"primarykey" json:"id"`
	UserID       uint      `gorm:"index" json:"userId"`
	OriginalName string    `gorm:"size:512;not null;index" json:"originalName"`
	FileName     string    `gorm:"size:512;not null;uniqueIndex" json:"fileName"`
	FileType     string    `gorm:"size:32;not null" json:"fileType"`
	FileSize     int64     `json:"fileSize"`
	URL          string    `gorm:"size:1024;not null" json:"url"`
	IsLocal      bool      `json:"isLocal"`
	CreatedAt    time.Time `json:"createdAt"`
}
