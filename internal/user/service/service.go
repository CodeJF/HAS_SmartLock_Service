package service

import (
	"errors"
	"fmt"
	"time"

	"has-smartlock-service/internal/pkg/auth"
	"has-smartlock-service/internal/pkg/config"
	"has-smartlock-service/internal/user/model"
	"has-smartlock-service/internal/user/repository"
)

var (
	ErrInvalidInput        = errors.New("invalid input")
	ErrUserExists          = errors.New("user already exists")
	ErrUserNotFound        = errors.New("user not found")
	ErrInvalidCredentials  = errors.New("invalid credentials")
	ErrInvalidCode         = errors.New("invalid verification code")
	ErrExpiredCode         = errors.New("verification code expired")
	ErrRefreshTokenInvalid = errors.New("refresh token invalid")
)

type Clock func() time.Time
type CodeGenerator func() string
type UIDGenerator func() string

type Service struct {
	repo          *repository.Repository
	tokenManager  *auth.TokenManager
	clock         Clock
	codeGenerator CodeGenerator
	uidGenerator  UIDGenerator
	cfg           config.Config
}

type TokenPair struct {
	UID          string `json:"uid"`
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	IsDebug      int    `json:"is_debug"`
	Expiration   int64  `json:"expiration"`
}

type RegisterSendInput struct {
	Username string
	Country  string
	Type     string
}

type RegisterInput struct {
	Username string
	Country  string
	Code     string
	Password string
}

type LoginInput struct {
	Username   string
	Type       string
	Password   string
	Code       string
	PhoneBrand string
}

type UserInfo struct {
	Username     string `json:"username"`
	Nickname     string `json:"nickname"`
	Avatar       string `json:"avatar"`
	IsDebug      int    `json:"is_debug"`
	RegisterTime int64  `json:"register_time"`
}

type ValidateCodeInput struct {
	Username string
	Code     string
	Type     string
}

type RefreshInput struct {
	RefreshToken string
}

type ResetInput struct {
	Username string
	Code     string
	Password string
}

type UpdatePasswordInput struct {
	UID         string
	NewPassword string
}

type UpdateInfoInput struct {
	UID      string
	Nickname string
}

type PutClientInput struct {
	UID       string
	PushType  int
	PushToken string
	Brand     string
	Version   string
	Language  string
	Zone      string
}

type DeleteInput struct {
	UID      string
	Username string
	Code     string
}

func New(repo *repository.Repository, tokenManager *auth.TokenManager, cfg config.Config) *Service {
	return &Service{
		repo:         repo,
		tokenManager: tokenManager,
		cfg:          cfg,
		clock:        time.Now,
		codeGenerator: func() string {
			return "123456"
		},
		uidGenerator: func() string {
			return fmt.Sprintf("u_%d", time.Now().UnixNano())
		},
	}
}

func (s *Service) WithClock(clock Clock) *Service {
	s.clock = clock
	return s
}

func (s *Service) WithCodeGenerator(generator CodeGenerator) *Service {
	s.codeGenerator = generator
	return s
}

func (s *Service) WithUIDGenerator(generator UIDGenerator) *Service {
	s.uidGenerator = generator
	return s
}

func (s *Service) SendVerificationCode(input RegisterSendInput) error {
	if input.Username == "" || input.Country == "" || input.Type == "" {
		return ErrInvalidInput
	}

	record := &model.VerificationCode{
		Username:  input.Username,
		Country:   input.Country,
		Type:      input.Type,
		Code:      s.codeGenerator(),
		ExpiresAt: s.clock().Add(time.Duration(s.cfg.VerificationTTL) * time.Second),
		SendCount: 1,
	}

	return s.repo.CreateVerificationCode(record)
}

func (s *Service) Register(input RegisterInput) (*TokenPair, error) {
	if input.Username == "" || input.Country == "" || input.Code == "" {
		return nil, ErrInvalidInput
	}

	if _, err := s.repo.FindUserByUsername(input.Username); err == nil {
		return nil, ErrUserExists
	} else if !repository.IsNotFound(err) {
		return nil, err
	}

	record, now, err := s.validateVerificationCode(input.Username, input.Code, "register")
	if err != nil {
		return nil, err
	}

	var result *TokenPair
	err = s.repo.WithTx(func(txRepo *repository.Repository) error {
		if err := txRepo.ConsumeVerificationCode(record.ID, now); err != nil {
			return err
		}

		user := &model.User{
			UID:          s.uidGenerator(),
			Username:     input.Username,
			Country:      input.Country,
			PasswordHash: input.Password,
			Nickname:     input.Username,
			IsDebug:      0,
			RegisterTime: now.Unix(),
		}
		if err := txRepo.CreateUser(user); err != nil {
			return err
		}

		tokens, err := s.issueTokens(txRepo, user, now)
		if err != nil {
			return err
		}
		result = tokens
		return nil
	})
	if err != nil {
		return nil, err
	}

	return result, nil
}

func (s *Service) Login(input LoginInput) (*TokenPair, error) {
	if input.Username == "" || input.Type == "" || input.PhoneBrand == "" {
		return nil, ErrInvalidInput
	}

	user, err := s.repo.FindUserByUsername(input.Username)
	if err != nil {
		if repository.IsNotFound(err) {
			return nil, ErrUserNotFound
		}
		return nil, err
	}

	now := s.clock()
	switch input.Type {
	case "password":
		if input.Password == "" || user.PasswordHash == "" || user.PasswordHash != input.Password {
			return nil, ErrInvalidCredentials
		}
	case "code":
		record, _, err := s.validateVerificationCode(input.Username, input.Code, "login")
		if err != nil {
			return nil, err
		}
		if err := s.repo.ConsumeVerificationCode(record.ID, now); err != nil {
			return nil, err
		}
	default:
		return nil, ErrInvalidInput
	}

	var result *TokenPair
	err = s.repo.WithTx(func(txRepo *repository.Repository) error {
		tokens, err := s.issueTokens(txRepo, user, now)
		if err != nil {
			return err
		}
		result = tokens
		return nil
	})
	if err != nil {
		return nil, err
	}

	return result, nil
}

func (s *Service) GetUserInfo(uid string) (*UserInfo, error) {
	if uid == "" {
		return nil, ErrInvalidInput
	}

	user, err := s.repo.FindUserByUID(uid)
	if err != nil {
		if repository.IsNotFound(err) {
			return nil, ErrUserNotFound
		}
		return nil, err
	}

	return &UserInfo{
		Username:     user.Username,
		Nickname:     user.Nickname,
		Avatar:       user.Avatar,
		IsDebug:      user.IsDebug,
		RegisterTime: user.RegisterTime,
	}, nil
}

func (s *Service) ValidateCode(input ValidateCodeInput) error {
	if input.Username == "" || input.Code == "" || input.Type == "" {
		return ErrInvalidInput
	}

	_, _, err := s.validateVerificationCode(input.Username, input.Code, input.Type)
	return err
}

func (s *Service) ResetPassword(input ResetInput) error {
	if input.Username == "" || input.Code == "" || input.Password == "" {
		return ErrInvalidInput
	}

	record, now, err := s.validateVerificationCode(input.Username, input.Code, "reset")
	if err != nil {
		return err
	}

	user, err := s.repo.FindUserByUsername(input.Username)
	if err != nil {
		if repository.IsNotFound(err) {
			return ErrUserNotFound
		}
		return err
	}

	return s.repo.WithTx(func(txRepo *repository.Repository) error {
		if err := txRepo.ConsumeVerificationCode(record.ID, now); err != nil {
			return err
		}

		if err := txRepo.UpdateUserPasswordByID(user.ID, input.Password); err != nil {
			return err
		}

		return nil
	})
}

func (s *Service) Refresh(input RefreshInput) (*TokenPair, error) {
	if input.RefreshToken == "" {
		return nil, ErrInvalidInput
	}

	record, err := s.repo.FindRefreshToken(input.RefreshToken)
	if err != nil {
		if repository.IsNotFound(err) {
			return nil, ErrRefreshTokenInvalid
		}
		return nil, err
	}

	now := s.clock()
	if record.RevokedAt != nil || now.After(record.ExpiresAt) {
		return nil, ErrRefreshTokenInvalid
	}

	user, err := s.repo.FindUserByID(record.UserID)
	if err != nil {
		if repository.IsNotFound(err) {
			return nil, ErrUserNotFound
		}
		return nil, err
	}

	var result *TokenPair
	err = s.repo.WithTx(func(txRepo *repository.Repository) error {
		if err := txRepo.RevokeActiveRefreshTokens(user.ID, now); err != nil {
			return err
		}

		tokens, err := s.issueTokens(txRepo, user, now)
		if err != nil {
			return err
		}
		result = tokens
		return nil
	})
	if err != nil {
		return nil, err
	}

	return result, nil
}

func (s *Service) Logout(uid string) error {
	if uid == "" {
		return ErrInvalidInput
	}

	user, err := s.repo.FindUserByUID(uid)
	if err != nil {
		if repository.IsNotFound(err) {
			return ErrUserNotFound
		}
		return err
	}

	return s.repo.RevokeActiveRefreshTokens(user.ID, s.clock())
}

func (s *Service) UpdatePassword(input UpdatePasswordInput) error {
	if input.UID == "" || input.NewPassword == "" {
		return ErrInvalidInput
	}

	user, err := s.repo.FindUserByUID(input.UID)
	if err != nil {
		if repository.IsNotFound(err) {
			return ErrUserNotFound
		}
		return err
	}

	return s.repo.UpdateUserPasswordByID(user.ID, input.NewPassword)
}

func (s *Service) UpdateInfo(input UpdateInfoInput) error {
	if input.UID == "" || input.Nickname == "" {
		return ErrInvalidInput
	}

	user, err := s.repo.FindUserByUID(input.UID)
	if err != nil {
		if repository.IsNotFound(err) {
			return ErrUserNotFound
		}
		return err
	}

	return s.repo.UpdateUserNicknameByID(user.ID, input.Nickname)
}

func (s *Service) PutClient(input PutClientInput) error {
	if input.UID == "" || input.PushType == 0 || input.PushToken == "" {
		return ErrInvalidInput
	}

	user, err := s.repo.FindUserByUID(input.UID)
	if err != nil {
		if repository.IsNotFound(err) {
			return ErrUserNotFound
		}
		return err
	}

	now := s.clock()
	client, err := s.repo.FindUserClientByUserIDAndPushToken(user.ID, input.PushToken)
	if err != nil && !repository.IsNotFound(err) {
		return err
	}

	if client == nil {
		return s.repo.CreateUserClient(&model.UserClient{
			UserID:     user.ID,
			PushType:   input.PushType,
			PushToken:  input.PushToken,
			Brand:      input.Brand,
			Version:    input.Version,
			Language:   input.Language,
			Zone:       input.Zone,
			LastSeenAt: now,
		})
	}

	return s.repo.UpdateUserClientByID(client.ID, map[string]any{
		"push_type":    input.PushType,
		"brand":        input.Brand,
		"version":      input.Version,
		"language":     input.Language,
		"zone":         input.Zone,
		"last_seen_at": now,
	})
}

func (s *Service) DeleteSend(uid, username string) error {
	if uid == "" || username == "" {
		return ErrInvalidInput
	}

	user, err := s.repo.FindUserByUID(uid)
	if err != nil {
		if repository.IsNotFound(err) {
			return ErrUserNotFound
		}
		return err
	}

	if user.Username != username {
		return ErrInvalidInput
	}

	return s.SendVerificationCode(RegisterSendInput{
		Username: username,
		Country:  user.Country,
		Type:     "delete",
	})
}

func (s *Service) DeleteAccount(input DeleteInput) error {
	if input.UID == "" || input.Username == "" || input.Code == "" {
		return ErrInvalidInput
	}

	user, err := s.repo.FindUserByUID(input.UID)
	if err != nil {
		if repository.IsNotFound(err) {
			return ErrUserNotFound
		}
		return err
	}

	if user.Username != input.Username {
		return ErrInvalidInput
	}

	record, now, err := s.validateVerificationCode(input.Username, input.Code, "delete")
	if err != nil {
		return err
	}

	return s.repo.WithTx(func(txRepo *repository.Repository) error {
		if err := txRepo.ConsumeVerificationCode(record.ID, now); err != nil {
			return err
		}

		if err := txRepo.SoftDeleteUserByID(user.ID, now); err != nil {
			return err
		}

		if err := txRepo.RevokeActiveRefreshTokens(user.ID, now); err != nil {
			return err
		}

		return nil
	})
}

func (s *Service) issueTokens(repo *repository.Repository, user *model.User, now time.Time) (*TokenPair, error) {
	accessToken, accessExpiresAt, err := s.tokenManager.GenerateAccessToken(user.UID, now)
	if err != nil {
		return nil, err
	}

	refreshToken, err := auth.GenerateRefreshToken()
	if err != nil {
		return nil, err
	}

	if err := repo.RevokeActiveRefreshTokens(user.ID, now); err != nil {
		return nil, err
	}
	if err := repo.CreateRefreshToken(&model.RefreshToken{
		UserID:    user.ID,
		Token:     refreshToken,
		ExpiresAt: s.tokenManager.RefreshTokenExpiresAt(now),
	}); err != nil {
		return nil, err
	}

	return &TokenPair{
		UID:          user.UID,
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		IsDebug:      user.IsDebug,
		Expiration:   accessExpiresAt.Unix(),
	}, nil
}

func (s *Service) validateVerificationCode(username, code, codeType string) (*model.VerificationCode, time.Time, error) {
	record, err := s.repo.FindLatestVerificationCode(username, codeType)
	if err != nil {
		if repository.IsNotFound(err) {
			return nil, time.Time{}, ErrInvalidCode
		}
		return nil, time.Time{}, err
	}

	now := s.clock()
	if record.UsedAt != nil || record.Code != code {
		return nil, time.Time{}, ErrInvalidCode
	}
	if now.After(record.ExpiresAt) {
		return nil, time.Time{}, ErrExpiredCode
	}

	return record, now, nil
}
