package service

import (
	"errors"
	"strings"

	"has-smartlock-service/internal/pkg/stsclient"
	userrepo "has-smartlock-service/internal/user/repository"
)

var (
	ErrInvalidInput = errors.New("invalid input")
	ErrUserNotFound = errors.New("user not found")
)

type TokenProvider interface {
	GetAvatarToken(uid string) (*stsclient.TokenResult, error)
}

type Service struct {
	userRepo      *userrepo.Repository
	tokenProvider TokenProvider
}

type GetTokenResult struct {
	AccessTokenID   string `json:"access_token_id"`
	AccessKeySecret string `json:"access_key_secret"`
	SecurityToken   string `json:"security_token"`
	Expiration      int64  `json:"expiration"`
	RegionID        string `json:"region_id"`
	Endpoint        string `json:"endpoint"`
	Bucket          string `json:"bucket"`
}

func New(userRepo *userrepo.Repository, tokenProvider TokenProvider) *Service {
	return &Service{userRepo: userRepo, tokenProvider: tokenProvider}
}

func (s *Service) GetToken(uid, uuid string) (*GetTokenResult, error) {
	if strings.TrimSpace(uid) == "" || strings.TrimSpace(uuid) == "" {
		return nil, ErrInvalidInput
	}

	if _, err := s.userRepo.FindUserByUID(uid); err != nil {
		if userrepo.IsNotFound(err) {
			return nil, ErrUserNotFound
		}
		return nil, err
	}

	result, err := s.tokenProvider.GetAvatarToken(uid)
	if err != nil {
		return nil, err
	}

	return &GetTokenResult{
		AccessTokenID:   result.AccessKeyID,
		AccessKeySecret: result.AccessKeySecret,
		SecurityToken:   result.SecurityToken,
		Expiration:      result.Expiration,
		RegionID:        result.RegionID,
		Endpoint:        result.Endpoint,
		Bucket:          result.Bucket,
	}, nil
}
