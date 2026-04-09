package model

import "time"

const (
	RoleOwner = 1
)

type Home struct {
	ID          uint   `gorm:"primaryKey"`
	HomeID      string `gorm:"size:64;uniqueIndex;not null"`
	OwnerUserID uint   `gorm:"index;not null"`
	Name        string `gorm:"size:191;not null"`
	Location    string `gorm:"size:255"`
	CreateTime  int64  `gorm:"not null"`
	CreatedAt   time.Time
	UpdatedAt   time.Time
	DeletedAt   *time.Time `gorm:"index"`
}

type HomeMember struct {
	ID        uint `gorm:"primaryKey"`
	HomeID    uint `gorm:"index;not null"`
	UserID    uint `gorm:"index;not null"`
	Role      int  `gorm:"not null;default:1"`
	Accept    int  `gorm:"not null;default:1"`
	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt *time.Time `gorm:"index"`
}

type HomeShareInvite struct {
	ID         uint   `gorm:"primaryKey"`
	MsgID      string `gorm:"size:64;uniqueIndex;not null"`
	HomeID     uint   `gorm:"index;not null"`
	FromUserID uint   `gorm:"index;not null"`
	ToUserID   uint   `gorm:"index;not null"`
	Accept     int    `gorm:"not null;default:0"`
	CreatedAt  time.Time
	UpdatedAt  time.Time
	DeletedAt  *time.Time `gorm:"index"`
}
