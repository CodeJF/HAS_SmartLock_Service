package model

import "time"

type Device struct {
	ID             uint   `gorm:"primaryKey"`
	UUID           string `gorm:"size:64;uniqueIndex;not null"`
	DeviceID       string `gorm:"size:64;uniqueIndex;not null"`
	UID            string `gorm:"size:64;not null"`
	BindType       int    `gorm:"not null;default:1"`
	Secret         string `gorm:"size:191;not null"`
	ModelCode      string `gorm:"size:64"`
	CurrentVersion string `gorm:"size:128"`
	Name           string `gorm:"size:191;not null"`
	FirstBindTime  int64  `gorm:"not null"`
	BindTime       int64  `gorm:"not null"`
	CreatedAt      time.Time
	UpdatedAt      time.Time
	DeletedAt      *time.Time `gorm:"index"`
}

type HomeDevice struct {
	ID        uint `gorm:"primaryKey"`
	HomeID    uint `gorm:"index;not null"`
	DeviceID  uint `gorm:"index;not null"`
	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt *time.Time `gorm:"index"`
}

type DeviceShareInvite struct {
	ID         uint   `gorm:"primaryKey"`
	MsgID      string `gorm:"size:64;uniqueIndex;not null"`
	DeviceID   uint   `gorm:"index;not null"`
	FromUserID uint   `gorm:"index;not null"`
	ToUserID   uint   `gorm:"index;not null"`
	Status     int    `gorm:"not null;default:0"`
	IsRead     int    `gorm:"not null;default:0"`
	CreatedAt  time.Time
	UpdatedAt  time.Time
	DeletedAt  *time.Time `gorm:"index"`
}

type DeviceShareMember struct {
	ID              uint `gorm:"primaryKey"`
	DeviceID        uint `gorm:"index;not null"`
	UserID          uint `gorm:"index;not null"`
	Role            int  `gorm:"not null;default:2"`
	GrantedByUserID uint `gorm:"index;not null"`
	CreatedAt       time.Time
	UpdatedAt       time.Time
	DeletedAt       *time.Time `gorm:"index"`
}

type DeviceShareFeedbackMessage struct {
	ID         uint   `gorm:"primaryKey"`
	MsgID      string `gorm:"size:64;uniqueIndex;not null"`
	DeviceID   uint   `gorm:"index;not null"`
	FromUserID uint   `gorm:"index;not null"`
	ToUserID   uint   `gorm:"index;not null"`
	Status     int    `gorm:"not null"`
	IsRead     int    `gorm:"not null;default:0"`
	CreatedAt  time.Time
	UpdatedAt  time.Time
	DeletedAt  *time.Time `gorm:"index"`
}
