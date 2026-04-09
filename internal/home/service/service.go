package service

import (
	"errors"
	"fmt"
	"strings"
	"time"

	devicemodel "has-smartlock-service/internal/device/model"
	devicerepo "has-smartlock-service/internal/device/repository"
	homemodel "has-smartlock-service/internal/home/model"
	homerepo "has-smartlock-service/internal/home/repository"
	"has-smartlock-service/internal/pkg/config"
	userrepo "has-smartlock-service/internal/user/repository"
)

var (
	ErrInvalidInput     = errors.New("invalid input")
	ErrUserNotFound     = errors.New("user not found")
	ErrHomeNotFound     = errors.New("home not found")
	ErrHomeForbidden    = errors.New("home forbidden")
	ErrHomeShareInvalid = errors.New("home share invalid")
	ErrDeviceNotFound   = errors.New("device not found")
	ErrDeviceForbidden  = errors.New("device forbidden")
	ErrHomeCreateFailed = errors.New("home create failed")
)

type Clock func() time.Time
type HomeIDGenerator func() string
type ShareMessageIDGenerator func() string

type Service struct {
	homeRepo        *homerepo.Repository
	deviceRepo      *devicerepo.Repository
	userRepo        *userrepo.Repository
	cfg             config.Config
	clock           Clock
	homeIDGenerator HomeIDGenerator
	shareMsgIDGen   ShareMessageIDGenerator
}

type HomeItem struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Location   string `json:"location"`
	Count      int    `json:"count"`
	Role       int    `json:"role"`
	CreateTime int64  `json:"create_time"`
}

type HomeUserItem struct {
	UID      string `json:"uid"`
	Username string `json:"username"`
	Avatar   string `json:"avatar"`
	Role     int    `json:"role"`
	Accept   int    `json:"accept"`
}

type DeviceState struct {
	Desired  map[string]any `json:"desired"`
	Reported map[string]any `json:"reported"`
}

type HomeDeviceItem struct {
	ID            int         `json:"id"`
	UUID          string      `json:"uuid"`
	DeviceID      string      `json:"device_id"`
	UID           string      `json:"uid"`
	BindType      int         `json:"bind_type"`
	Secret        string      `json:"secret"`
	Name          string      `json:"name"`
	FirstBindTime int64       `json:"first_bind_time"`
	BindTime      int64       `json:"bind_time"`
	State         DeviceState `json:"State"`
}

func New(homeRepo *homerepo.Repository, deviceRepo *devicerepo.Repository, userRepo *userrepo.Repository, cfg config.Config) *Service {
	return &Service{
		homeRepo:   homeRepo,
		deviceRepo: deviceRepo,
		userRepo:   userRepo,
		cfg:        cfg,
		clock:      time.Now,
		homeIDGenerator: func() string {
			return fmt.Sprintf("h_%d", time.Now().UnixNano())
		},
		shareMsgIDGen: func() string {
			return fmt.Sprintf("msg_%d", time.Now().UnixNano())
		},
	}
}

func (s *Service) WithClock(clock Clock) *Service {
	s.clock = clock
	return s
}

func (s *Service) WithHomeIDGenerator(generator HomeIDGenerator) *Service {
	s.homeIDGenerator = generator
	return s
}

func (s *Service) WithShareMessageIDGenerator(generator ShareMessageIDGenerator) *Service {
	s.shareMsgIDGen = generator
	return s
}

func (s *Service) CreateHome(uid, name string) error {
	if strings.TrimSpace(uid) == "" || strings.TrimSpace(name) == "" {
		return ErrInvalidInput
	}

	user, err := s.userRepo.FindUserByUID(uid)
	if err != nil {
		if userrepo.IsNotFound(err) {
			return ErrUserNotFound
		}
		return err
	}

	now := s.clock()
	return s.homeRepo.WithTx(func(txRepo *homerepo.Repository) error {
		home := &homemodel.Home{
			HomeID:      s.homeIDGenerator(),
			OwnerUserID: user.ID,
			Name:        strings.TrimSpace(name),
			Location:    "",
			CreateTime:  now.Unix(),
		}
		if err := txRepo.CreateHome(home); err != nil {
			return err
		}
		member := &homemodel.HomeMember{
			HomeID: home.ID,
			UserID: user.ID,
			Role:   homemodel.RoleOwner,
			Accept: 1,
		}
		return txRepo.CreateHomeMember(member)
	})
}

func (s *Service) ListHomes(uid string) ([]HomeItem, error) {
	if strings.TrimSpace(uid) == "" {
		return nil, ErrInvalidInput
	}

	user, err := s.userRepo.FindUserByUID(uid)
	if err != nil {
		if userrepo.IsNotFound(err) {
			return nil, ErrUserNotFound
		}
		return nil, err
	}

	homes, err := s.homeRepo.ListHomesByUserID(user.ID)
	if err != nil {
		return nil, err
	}

	result := make([]HomeItem, 0, len(homes))
	for _, item := range homes {
		result = append(result, HomeItem{
			ID:         item.Home.HomeID,
			Name:       item.Home.Name,
			Location:   item.Home.Location,
			Count:      0,
			Role:       item.Role,
			CreateTime: item.Home.CreateTime,
		})
	}
	return result, nil
}

func (s *Service) ListHomeUsers(uid, homeID string) ([]HomeUserItem, error) {
	if strings.TrimSpace(uid) == "" || strings.TrimSpace(homeID) == "" {
		return nil, ErrInvalidInput
	}

	user, err := s.userRepo.FindUserByUID(uid)
	if err != nil {
		if userrepo.IsNotFound(err) {
			return nil, ErrUserNotFound
		}
		return nil, err
	}

	homeWithMember, err := s.homeRepo.FindHomeMembershipByHomeIDAndUserID(strings.TrimSpace(homeID), user.ID)
	if err != nil {
		if homerepo.IsNotFound(err) {
			return nil, ErrHomeNotFound
		}
		return nil, err
	}

	members, err := s.homeRepo.ListHomeMembersByInternalHomeID(homeWithMember.Home.ID)
	if err != nil {
		return nil, err
	}

	result := make([]HomeUserItem, 0, len(members))
	for _, member := range members {
		result = append(result, HomeUserItem{
			UID:      member.User.UID,
			Username: member.User.Username,
			Avatar:   s.avatarObjectKey(member.User.UID),
			Role:     member.Member.Role,
			Accept:   member.Member.Accept,
		})
	}
	return result, nil
}

func (s *Service) ListHomeDevices(uid, homeID string) ([]HomeDeviceItem, error) {
	if strings.TrimSpace(uid) == "" || strings.TrimSpace(homeID) == "" {
		return nil, ErrInvalidInput
	}

	user, err := s.userRepo.FindUserByUID(uid)
	if err != nil {
		if userrepo.IsNotFound(err) {
			return nil, ErrUserNotFound
		}
		return nil, err
	}

	homeWithMember, err := s.homeRepo.FindHomeMembershipByHomeIDAndUserID(strings.TrimSpace(homeID), user.ID)
	if err != nil {
		if homerepo.IsNotFound(err) {
			return nil, ErrHomeNotFound
		}
		return nil, err
	}

	devices, err := s.homeRepo.ListHomeDevicesByInternalHomeID(homeWithMember.Home.ID)
	if err != nil {
		return nil, err
	}

	result := make([]HomeDeviceItem, 0, len(devices))
	for _, item := range devices {
		result = append(result, HomeDeviceItem{
			ID:            int(item.Link.ID),
			UUID:          item.Device.UUID,
			DeviceID:      item.Device.DeviceID,
			UID:           item.Device.UID,
			BindType:      item.Device.BindType,
			Secret:        item.Device.Secret,
			Name:          item.Device.Name,
			FirstBindTime: item.Device.FirstBindTime,
			BindTime:      item.Device.BindTime,
			State: DeviceState{
				Desired:  map[string]any{},
				Reported: map[string]any{},
			},
		})
	}
	return result, nil
}

func (s *Service) UpdateHome(uid, homeID, name, location string) error {
	if strings.TrimSpace(uid) == "" || strings.TrimSpace(homeID) == "" || strings.TrimSpace(name) == "" || strings.TrimSpace(location) == "" {
		return ErrInvalidInput
	}

	user, err := s.userRepo.FindUserByUID(uid)
	if err != nil {
		if userrepo.IsNotFound(err) {
			return ErrUserNotFound
		}
		return err
	}

	homeWithMember, err := s.homeRepo.FindHomeMembershipByHomeIDAndUserID(strings.TrimSpace(homeID), user.ID)
	if err != nil {
		if homerepo.IsNotFound(err) {
			return ErrHomeNotFound
		}
		return err
	}
	if homeWithMember.Role != homemodel.RoleOwner {
		return ErrHomeForbidden
	}

	return s.homeRepo.UpdateHomeByInternalID(homeWithMember.Home.ID, map[string]any{
		"name":     strings.TrimSpace(name),
		"location": strings.TrimSpace(location),
	})
}

func (s *Service) AddDeviceToHome(uid, homeID, uuid string) error {
	if strings.TrimSpace(uid) == "" || strings.TrimSpace(homeID) == "" || strings.TrimSpace(uuid) == "" {
		return ErrInvalidInput
	}

	user, err := s.userRepo.FindUserByUID(strings.TrimSpace(uid))
	if err != nil {
		if userrepo.IsNotFound(err) {
			return ErrUserNotFound
		}
		return err
	}

	homeWithMember, err := s.homeRepo.FindHomeMembershipByHomeIDAndUserID(strings.TrimSpace(homeID), user.ID)
	if err != nil {
		if homerepo.IsNotFound(err) {
			return ErrHomeNotFound
		}
		return err
	}
	if homeWithMember.Role != homemodel.RoleOwner {
		return ErrHomeForbidden
	}

	device, err := s.deviceRepo.FindDeviceByUUID(strings.TrimSpace(uuid))
	if err != nil {
		if devicerepo.IsNotFound(err) {
			return ErrDeviceNotFound
		}
		return err
	}
	if device.UID != user.UID {
		return ErrDeviceForbidden
	}

	if _, err := s.homeRepo.FindActiveHomeDeviceByInternalHomeIDAndDeviceID(homeWithMember.Home.ID, device.ID); err == nil {
		return nil
	} else if !homerepo.IsNotFound(err) {
		return err
	}

	activeLink, err := s.homeRepo.FindActiveHomeDeviceByInternalDeviceID(device.ID)
	if err == nil {
		if activeLink.HomeID == homeWithMember.Home.ID {
			return nil
		}
		return ErrDeviceForbidden
	}
	if !homerepo.IsNotFound(err) {
		return err
	}

	return s.homeRepo.CreateHomeDevice(&devicemodel.HomeDevice{
		HomeID:   homeWithMember.Home.ID,
		DeviceID: device.ID,
	})
}

func (s *Service) ChangeDeviceHome(uid, targetHomeID, uuid string) error {
	if strings.TrimSpace(uid) == "" || strings.TrimSpace(targetHomeID) == "" || strings.TrimSpace(uuid) == "" {
		return ErrInvalidInput
	}

	user, err := s.userRepo.FindUserByUID(strings.TrimSpace(uid))
	if err != nil {
		if userrepo.IsNotFound(err) {
			return ErrUserNotFound
		}
		return err
	}

	device, err := s.deviceRepo.FindDeviceByUUID(strings.TrimSpace(uuid))
	if err != nil {
		if devicerepo.IsNotFound(err) {
			return ErrDeviceNotFound
		}
		return err
	}
	if device.UID != user.UID {
		return ErrDeviceForbidden
	}

	activeLink, err := s.homeRepo.FindActiveHomeDeviceByInternalDeviceID(device.ID)
	if err != nil {
		if homerepo.IsNotFound(err) {
			return ErrDeviceForbidden
		}
		return err
	}

	if _, err := s.homeRepo.FindHomeMembershipByInternalHomeIDAndUserID(activeLink.HomeID, user.ID); err != nil {
		if homerepo.IsNotFound(err) {
			return ErrHomeForbidden
		}
		return err
	}

	targetHomeWithMember, err := s.homeRepo.FindHomeMembershipByHomeIDAndUserID(strings.TrimSpace(targetHomeID), user.ID)
	if err != nil {
		if homerepo.IsNotFound(err) {
			return ErrHomeNotFound
		}
		return err
	}
	if targetHomeWithMember.Role != homemodel.RoleOwner {
		return ErrHomeForbidden
	}

	if activeLink.HomeID == targetHomeWithMember.Home.ID {
		return nil
	}

	now := s.clock()
	return s.homeRepo.WithTx(func(txRepo *homerepo.Repository) error {
		if err := txRepo.SoftDeleteHomeDeviceByID(activeLink.ID, now); err != nil {
			return err
		}
		return txRepo.CreateHomeDevice(&devicemodel.HomeDevice{
			HomeID:   targetHomeWithMember.Home.ID,
			DeviceID: device.ID,
		})
	})
}

func (s *Service) ShareHome(uid, homeID, username string) error {
	if strings.TrimSpace(uid) == "" || strings.TrimSpace(homeID) == "" || strings.TrimSpace(username) == "" {
		return ErrInvalidInput
	}

	user, err := s.userRepo.FindUserByUID(strings.TrimSpace(uid))
	if err != nil {
		if userrepo.IsNotFound(err) {
			return ErrUserNotFound
		}
		return err
	}

	homeWithMember, err := s.homeRepo.FindHomeMembershipByHomeIDAndUserID(strings.TrimSpace(homeID), user.ID)
	if err != nil {
		if homerepo.IsNotFound(err) {
			return ErrHomeNotFound
		}
		return err
	}
	if homeWithMember.Role != homemodel.RoleOwner {
		return ErrHomeForbidden
	}

	targetUser, err := s.userRepo.FindUserByUsername(strings.TrimSpace(username))
	if err != nil {
		if userrepo.IsNotFound(err) {
			return ErrUserNotFound
		}
		return err
	}
	if targetUser.ID == user.ID {
		return ErrHomeShareInvalid
	}

	if _, err := s.homeRepo.FindHomeMembershipByInternalHomeIDAndUserID(homeWithMember.Home.ID, targetUser.ID); err == nil {
		return ErrHomeShareInvalid
	} else if !homerepo.IsNotFound(err) {
		return err
	}

	if _, err := s.homeRepo.FindActiveHomeShareInviteByInternalHomeIDAndToUserID(homeWithMember.Home.ID, targetUser.ID); err == nil {
		return ErrHomeShareInvalid
	} else if !homerepo.IsNotFound(err) {
		return err
	}

	return s.homeRepo.CreateHomeShareInvite(&homemodel.HomeShareInvite{
		MsgID:      s.shareMsgIDGen(),
		HomeID:     homeWithMember.Home.ID,
		FromUserID: user.ID,
		ToUserID:   targetUser.ID,
		Accept:     0,
	})
}

func (s *Service) DeleteHome(uid, homeID string) error {
	if strings.TrimSpace(uid) == "" || strings.TrimSpace(homeID) == "" {
		return ErrInvalidInput
	}

	user, err := s.userRepo.FindUserByUID(uid)
	if err != nil {
		if userrepo.IsNotFound(err) {
			return ErrUserNotFound
		}
		return err
	}

	homeWithMember, err := s.homeRepo.FindHomeMembershipByHomeIDAndUserID(strings.TrimSpace(homeID), user.ID)
	if err != nil {
		if homerepo.IsNotFound(err) {
			return ErrHomeNotFound
		}
		return err
	}
	if homeWithMember.Role != homemodel.RoleOwner {
		return ErrHomeForbidden
	}

	now := s.clock()
	return s.homeRepo.WithTx(func(txRepo *homerepo.Repository) error {
		if err := txRepo.SoftDeleteHomeMembersByInternalHomeID(homeWithMember.Home.ID, now); err != nil {
			return err
		}
		return txRepo.SoftDeleteHomeByInternalID(homeWithMember.Home.ID, now)
	})
}

func (s *Service) avatarObjectKey(uid string) string {
	prefix := strings.Trim(strings.TrimSpace(s.cfg.OSSAvatarPrefix), "/")
	if prefix == "" {
		prefix = "avatar"
	}
	uid = strings.Trim(strings.TrimSpace(uid), "/")
	if uid == "" {
		return prefix
	}
	return prefix + "/" + uid
}
