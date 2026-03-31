package model

import "time"

type User struct {
	ID           uint   `gorm:"primaryKey"`
	UID          string `gorm:"size:64;uniqueIndex;not null"`
	Username     string `gorm:"size:191;uniqueIndex;not null"`
	Country      string `gorm:"size:16;not null"`
	PasswordHash string `gorm:"size:255"`
	Nickname     string `gorm:"size:191"`
	Avatar       string `gorm:"size:1024"`
	IsDebug      int    `gorm:"not null;default:0"`
	RegisterTime int64  `gorm:"not null"`
	CreatedAt    time.Time
	UpdatedAt    time.Time
	DeletedAt    *time.Time `gorm:"index"`
}

type VerificationCode struct {
	ID           uint      `gorm:"primaryKey"`
	Username     string    `gorm:"size:191;index:idx_verification_lookup,priority:1;not null"`
	Country      string    `gorm:"size:16;not null"`
	Type         string    `gorm:"size:32;index:idx_verification_lookup,priority:2;not null"`
	Code         string    `gorm:"size:32;not null"`
	ExpiresAt    time.Time `gorm:"not null"`
	UsedAt       *time.Time
	SendCount    int `gorm:"not null;default:1"`
	AttemptCount int `gorm:"not null;default:0"`
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

type RefreshToken struct {
	ID        uint      `gorm:"primaryKey"`
	UserID    uint      `gorm:"index;not null"`
	Token     string    `gorm:"size:128;uniqueIndex;not null"`
	ClientID  string    `gorm:"size:191"`
	ExpiresAt time.Time `gorm:"not null"`
	RevokedAt *time.Time
	CreatedAt time.Time
	UpdatedAt time.Time
}

type UserClient struct {
	ID         uint      `gorm:"primaryKey"`
	UserID     uint      `gorm:"index;not null"`
	PushType   int       `gorm:"not null"`
	PushToken  string    `gorm:"size:512;not null"`
	Brand      string    `gorm:"size:191"`
	Version    string    `gorm:"size:64"`
	Language   string    `gorm:"size:32"`
	Zone       string    `gorm:"size:64"`
	LastSeenAt time.Time `gorm:"not null"`
	CreatedAt  time.Time
	UpdatedAt  time.Time
}
