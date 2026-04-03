package repository

import (
	"errors"
	"time"

	"gorm.io/gorm"

	"has-smartlock-service/internal/user/model"
)

type Repository struct {
	db *gorm.DB
}

func New(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) CreateVerificationCode(code *model.VerificationCode) error {
	return r.db.Create(code).Error
}

func (r *Repository) FindLatestVerificationCode(username, codeType string) (*model.VerificationCode, error) {
	var record model.VerificationCode
	err := r.db.
		Where("username = ? AND type = ?", username, codeType).
		Order("id DESC").
		First(&record).Error
	if err != nil {
		return nil, err
	}
	return &record, nil
}

func (r *Repository) ConsumeVerificationCode(id uint, now time.Time) error {
	return r.db.Model(&model.VerificationCode{}).
		Where("id = ? AND used_at IS NULL", id).
		Update("used_at", now).Error
}

func (r *Repository) FindRefreshToken(token string) (*model.RefreshToken, error) {
	var refreshToken model.RefreshToken
	err := r.db.Where("token = ?", token).First(&refreshToken).Error
	if err != nil {
		return nil, err
	}

	return &refreshToken, nil
}

func (r *Repository) CreateUser(user *model.User) error {
	return r.db.Create(user).Error
}

func (r *Repository) FindUserByUsername(username string) (*model.User, error) {
	var user model.User
	err := r.db.Where("username = ? AND deleted_at IS NULL", username).First(&user).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *Repository) FindUserByUID(uid string) (*model.User, error) {
	var user model.User
	err := r.db.Where("uid = ? AND deleted_at IS NULL", uid).First(&user).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *Repository) FindUserByID(id uint) (*model.User, error) {
	var user model.User
	err := r.db.Where("id = ? AND deleted_at IS NULL", id).First(&user).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *Repository) CreateRefreshToken(token *model.RefreshToken) error {
	return r.db.Create(token).Error
}

func (r *Repository) UpdateUserPasswordByID(id uint, passwordHash string) error {
	return r.db.Model(&model.User{}).
		Where("id = ? AND deleted_at IS NULL", id).
		Update("password_hash", passwordHash).Error
}

func (r *Repository) UpdateUserNicknameByID(id uint, nickname string) error {
	return r.db.Model(&model.User{}).
		Where("id = ? AND deleted_at IS NULL", id).
		Update("nickname", nickname).Error
}

func (r *Repository) UpdateUserAvatarByID(id uint, avatar string) error {
	return r.db.Model(&model.User{}).
		Where("id = ? AND deleted_at IS NULL", id).
		Update("avatar", avatar).Error
}

func (r *Repository) FindUserClientByUserIDAndPushToken(userID uint, pushToken string) (*model.UserClient, error) {
	var client model.UserClient
	err := r.db.Where("user_id = ? AND push_token = ?", userID, pushToken).First(&client).Error
	if err != nil {
		return nil, err
	}

	return &client, nil
}

func (r *Repository) CreateUserClient(client *model.UserClient) error {
	return r.db.Create(client).Error
}

func (r *Repository) UpdateUserClientByID(id uint, attrs map[string]any) error {
	return r.db.Model(&model.UserClient{}).
		Where("id = ?", id).
		Updates(attrs).Error
}

func (r *Repository) SoftDeleteUserByID(id uint, now time.Time) error {
	return r.db.Model(&model.User{}).
		Where("id = ? AND deleted_at IS NULL", id).
		Update("deleted_at", now).Error
}

func (r *Repository) RevokeActiveRefreshTokens(userID uint, now time.Time) error {
	return r.db.Model(&model.RefreshToken{}).
		Where("user_id = ? AND revoked_at IS NULL", userID).
		Update("revoked_at", now).Error
}

func (r *Repository) WithTx(fn func(txRepo *Repository) error) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		return fn(&Repository{db: tx})
	})
}

func IsNotFound(err error) bool {
	return errors.Is(err, gorm.ErrRecordNotFound)
}
