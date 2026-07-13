package models

import "time"

type TaskStatus string

const (
	TaskStatusPending    TaskStatus = "PENDING"
	TaskStatusDispatched TaskStatus = "DISPATCHED"
	TaskStatusRunning    TaskStatus = "RUNNING"
	TaskStatusSuccess    TaskStatus = "SUCCESS"
	TaskStatusFailed     TaskStatus = "FAILED"
	TaskStatusCancelled  TaskStatus = "CANCELLED"
	TaskStatusQueued     TaskStatus = "QUEUED"
)

type Task struct {
	ID             uint       `gorm:"primarykey;autoIncrement" json:"id"`
	UserID         uint       `gorm:"index" json:"userId"`
	ApiKeyID       *uint      `gorm:"index" json:"apiKeyId"`
	ApiKeyName     string     `gorm:"-" json:"apiKeyName,omitempty"`
	WebappID       string     `gorm:"size:64;not null" json:"webappId"`
	NodeInfoList   string     `gorm:"type:text;not null" json:"nodeInfoList"`
	WebhookURL     string     `gorm:"size:512" json:"webhookUrl"`
	InstanceType   string     `gorm:"size:32;default:default" json:"instanceType"`
	AccessPassword string     `gorm:"size:128" json:"-"`
	Status         TaskStatus `gorm:"size:20;index;default:PENDING" json:"status"`
	IsLocal        bool       `gorm:"default:false" json:"isLocal"`
	RhTaskID       string     `gorm:"size:64;index" json:"rhTaskId"`
	RhClientID     string     `gorm:"size:128" json:"rhClientId"`
	RhWssURL       string     `gorm:"type:text" json:"rhWssUrl"`
	ErrorMessage   string     `gorm:"type:text" json:"errorMessage"`
	Results        string     `gorm:"type:text" json:"results"`
	PromptTips     string     `gorm:"type:text" json:"promptTips"`
	CreatedAt      time.Time  `json:"createdAt"`
	UpdatedAt      time.Time  `json:"updatedAt"`
	DispatchedAt   *time.Time `json:"dispatchedAt"`
	CompletedAt    *time.Time `json:"completedAt"`
}
