package service

import (
	"errors"
	"strings"
	"time"

	devicemodel "has-smartlock-service/internal/device/model"
	devicerepo "has-smartlock-service/internal/device/repository"
	homerepo "has-smartlock-service/internal/home/repository"
	usermodel "has-smartlock-service/internal/user/model"
	userrepo "has-smartlock-service/internal/user/repository"
)

var (
	ErrInvalidInput    = errors.New("invalid input")
	ErrUserNotFound    = errors.New("user not found")
	ErrHomeNotFound    = errors.New("home not found")
	ErrHomeForbidden   = errors.New("home forbidden")
	ErrDeviceNotFound  = errors.New("device not found")
	ErrDeviceForbidden = errors.New("device forbidden")
)

type DeviceState struct {
	Desired  map[string]any `json:"desired"`
	Reported map[string]any `json:"reported"`
}

type DeviceItem struct {
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

type NewDeviceItem struct {
	UUID          string `json:"uuid"`
	DeviceID      string `json:"device_id"`
	UID           string `json:"uid"`
	BindType      int    `json:"bind_type"`
	Secret        string `json:"secret"`
	Name          string `json:"name"`
	BindStatus    int    `json:"bind_status"`
	FirstBindTime int64  `json:"first_bind_time"`
	BindTime      int64  `json:"bind_time"`
	DeleteTime    int64  `json:"delete_time"`
}

type BindInput struct {
	Model   string
	UUID    string
	AppID   string
	UID     string
	MAC     string
	Zone    string
	Version string
}

type DeviceLoginInput struct {
	Model   string
	UUID    string
	UID     string
	Zone    string
	Version string
}

type Service struct {
	deviceRepo *devicerepo.Repository
	homeRepo   *homerepo.Repository
	userRepo   *userrepo.Repository
}

func New(deviceRepo *devicerepo.Repository, homeRepo *homerepo.Repository, userRepo *userrepo.Repository) *Service {
	return &Service{
		deviceRepo: deviceRepo,
		homeRepo:   homeRepo,
		userRepo:   userRepo,
	}
}

func (s *Service) Bind(input BindInput) error {
	if strings.TrimSpace(input.Model) == "" ||
		strings.TrimSpace(input.UUID) == "" ||
		strings.TrimSpace(input.AppID) == "" ||
		strings.TrimSpace(input.UID) == "" ||
		strings.TrimSpace(input.MAC) == "" ||
		strings.TrimSpace(input.Zone) == "" ||
		strings.TrimSpace(input.Version) == "" {
		return ErrInvalidInput
	}

	if _, err := s.mustFindUser(strings.TrimSpace(input.UID)); err != nil {
		return err
	}

	now := time.Now().Unix()
	device, err := s.deviceRepo.FindDeviceByUUID(strings.TrimSpace(input.UUID))
	if err != nil {
		if !devicerepo.IsNotFound(err) {
			return err
		}

		return s.deviceRepo.CreateDevice(&devicemodel.Device{
			UUID:          strings.TrimSpace(input.UUID),
			DeviceID:      strings.TrimSpace(input.UUID),
			UID:           strings.TrimSpace(input.UID),
			BindType:      1,
			Secret:        strings.TrimSpace(input.UUID),
			Name:          strings.TrimSpace(input.Model),
			FirstBindTime: now,
			BindTime:      now,
		})
	}

	attrs := map[string]any{
		"uid":       strings.TrimSpace(input.UID),
		"bind_type": 1,
		"bind_time": now,
	}
	if strings.TrimSpace(device.DeviceID) == "" {
		attrs["device_id"] = strings.TrimSpace(input.UUID)
	}
	if strings.TrimSpace(device.Secret) == "" {
		attrs["secret"] = strings.TrimSpace(input.UUID)
	}
	if strings.TrimSpace(device.Name) == "" {
		attrs["name"] = strings.TrimSpace(input.Model)
	}

	return s.deviceRepo.UpdateDeviceByID(device.ID, attrs)
}

func (s *Service) DeviceLogin(input DeviceLoginInput) error {
	if strings.TrimSpace(input.Model) == "" ||
		strings.TrimSpace(input.UUID) == "" ||
		strings.TrimSpace(input.UID) == "" ||
		strings.TrimSpace(input.Zone) == "" ||
		strings.TrimSpace(input.Version) == "" {
		return ErrInvalidInput
	}

	if _, err := s.mustFindUser(strings.TrimSpace(input.UID)); err != nil {
		return err
	}

	device, err := s.deviceRepo.FindDeviceByUUID(strings.TrimSpace(input.UUID))
	if err != nil {
		if devicerepo.IsNotFound(err) {
			return ErrDeviceNotFound
		}
		return err
	}
	if device.UID != strings.TrimSpace(input.UID) {
		return ErrDeviceForbidden
	}

	return nil
}

func (s *Service) List(uid, homeID string) ([]DeviceItem, error) {
	user, err := s.mustFindUser(uid)
	if err != nil {
		return nil, err
	}

	var devices []devicerepo.VisibleDevice
	if strings.TrimSpace(homeID) != "" {
		homeWithMember, err := s.homeRepo.FindHomeMembershipByHomeIDAndUserID(strings.TrimSpace(homeID), user.ID)
		if err != nil {
			if homerepo.IsNotFound(err) {
				return nil, ErrHomeNotFound
			}
			return nil, err
		}

		devices, err = s.deviceRepo.ListDevicesByHomeID(homeWithMember.Home.ID)
		if err != nil {
			return nil, err
		}
	} else {
		devices, err = s.deviceRepo.ListVisibleDevices(user.ID, user.UID)
		if err != nil {
			return nil, err
		}
	}

	return mapVisibleDevices(devices), nil
}

func (s *Service) NewList(uid string) ([]NewDeviceItem, error) {
	user, err := s.mustFindUser(uid)
	if err != nil {
		return nil, err
	}

	devices, err := s.deviceRepo.ListOwnedDevices(user.UID)
	if err != nil {
		return nil, err
	}

	result := make([]NewDeviceItem, 0, len(devices))
	for _, item := range devices {
		result = append(result, mapNewDevice(item))
	}
	return result, nil
}

func (s *Service) UpdateName(uid, uuid, name string) error {
	user, err := s.mustFindUser(uid)
	if err != nil {
		return err
	}
	if strings.TrimSpace(uuid) == "" || strings.TrimSpace(name) == "" {
		return ErrInvalidInput
	}

	device, err := s.deviceRepo.FindDeviceByUUID(strings.TrimSpace(uuid))
	if err != nil {
		if devicerepo.IsNotFound(err) {
			return ErrDeviceNotFound
		}
		return err
	}

	if _, err := s.deviceRepo.FindVisibleDeviceByUUID(user.ID, user.UID, strings.TrimSpace(uuid)); err != nil {
		if devicerepo.IsNotFound(err) {
			return ErrDeviceForbidden
		}
		return err
	}

	return s.deviceRepo.UpdateDeviceNameByID(device.ID, strings.TrimSpace(name))
}

func (s *Service) mustFindUser(uid string) (*usermodel.User, error) {
	if strings.TrimSpace(uid) == "" {
		return nil, ErrInvalidInput
	}

	user, err := s.userRepo.FindUserByUID(strings.TrimSpace(uid))
	if err != nil {
		if userrepo.IsNotFound(err) {
			return nil, ErrUserNotFound
		}
		return nil, err
	}
	return user, nil
}

func mapVisibleDevices(items []devicerepo.VisibleDevice) []DeviceItem {
	result := make([]DeviceItem, 0, len(items))
	for _, item := range items {
		result = append(result, DeviceItem{
			ID:            int(item.ID),
			UUID:          item.UUID,
			DeviceID:      item.DeviceID,
			UID:           item.UID,
			BindType:      item.BindType,
			Secret:        item.Secret,
			Name:          item.Name,
			FirstBindTime: item.FirstBindTime,
			BindTime:      item.BindTime,
			State: DeviceState{
				Desired:  map[string]any{},
				Reported: map[string]any{},
			},
		})
	}
	return result
}

func mapNewDevice(item devicemodel.Device) NewDeviceItem {
	return NewDeviceItem{
		UUID:          item.UUID,
		DeviceID:      item.DeviceID,
		UID:           item.UID,
		BindType:      item.BindType,
		Secret:        item.Secret,
		Name:          item.Name,
		BindStatus:    1,
		FirstBindTime: item.FirstBindTime,
		BindTime:      item.BindTime,
		DeleteTime:    0,
	}
}
