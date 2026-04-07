package model

import "time"

type Device struct {
	ID            uint   `gorm:"primaryKey"`
	UUID          string `gorm:"size:64;uniqueIndex;not null"`
	DeviceID      string `gorm:"size:64;uniqueIndex;not null"`
	UID           string `gorm:"size:64;not null"`
	BindType      int    `gorm:"not null;default:1"`
	Secret        string `gorm:"size:191;not null"`
	Name          string `gorm:"size:191;not null"`
	FirstBindTime int64  `gorm:"not null"`
	BindTime      int64  `gorm:"not null"`
	CreatedAt     time.Time
	UpdatedAt     time.Time
	DeletedAt     *time.Time `gorm:"index"`
}

type HomeDevice struct {
	ID        uint `gorm:"primaryKey"`
	HomeID    uint `gorm:"index;not null"`
	DeviceID  uint `gorm:"index;not null"`
	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt *time.Time `gorm:"index"`
}
