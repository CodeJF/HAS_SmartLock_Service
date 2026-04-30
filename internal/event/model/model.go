package model

import "time"

type DeviceEvent struct {
	ID         uint   `gorm:"primaryKey"`
	DeviceID   uint   `gorm:"index;not null"`
	HomeID     *uint  `gorm:"index"`
	EventType  int    `gorm:"not null"`
	EventTime  int64  `gorm:"not null"`
	DeviceTime int64  `gorm:"not null"`
	Thumbnail  string `gorm:"size:1024"`
	Payload    string `gorm:"type:json"`
	CreatedAt  time.Time
	UpdatedAt  time.Time
	DeletedAt  *time.Time `gorm:"index"`
}

type DeviceEventUserState struct {
	ID        uint `gorm:"primaryKey"`
	EventID   uint `gorm:"uniqueIndex:idx_event_user_states_event_user;not null"`
	UserID    uint `gorm:"uniqueIndex:idx_event_user_states_event_user;not null"`
	IsRead    int  `gorm:"not null;default:0"`
	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt *time.Time `gorm:"index"`
}
