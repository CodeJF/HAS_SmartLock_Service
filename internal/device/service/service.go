package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	devicemodel "has-smartlock-service/internal/device/model"
	devicerepo "has-smartlock-service/internal/device/repository"
	homerepo "has-smartlock-service/internal/home/repository"
	"has-smartlock-service/internal/pkg/config"
	"has-smartlock-service/internal/pkg/redisx"
	eventstore "has-smartlock-service/internal/realtime/eventstore"
	"has-smartlock-service/internal/realtime/shadow"
	usermodel "has-smartlock-service/internal/user/model"
	userrepo "has-smartlock-service/internal/user/repository"
)

var (
	ErrInvalidInput       = errors.New("invalid input")
	ErrUserNotFound       = errors.New("user not found")
	ErrHomeNotFound       = errors.New("home not found")
	ErrHomeForbidden      = errors.New("home forbidden")
	ErrDeviceNotFound     = errors.New("device not found")
	ErrDeviceForbidden    = errors.New("device forbidden")
	ErrDeviceShareInvalid = errors.New("device share invalid")
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

type ShareRecordItem struct {
	Username string `json:"username"`
	UUID     string `json:"uuid"`
	UID      string `json:"uid"`
	Status   int    `json:"status"`
	Role     int    `json:"role"`
}

type DeviceModelItem struct {
	ModelCode   string `json:"model_code"`
	Status      int    `json:"status"`
	ModelName   string `json:"model_name"`
	Category    string `json:"category"`
	ShowName    string `json:"show_name"`
	DefaultName string `json:"default_name"`
	Thumbnail   string `json:"thumbnail"`
}

type DeviceUpgradeVersion struct {
	Flag    string `json:"flag"`
	Version string `json:"version,omitempty"`
}

type DeviceUpgradeResult struct {
	Has     bool                  `json:"has"`
	Version *DeviceUpgradeVersion `json:"version"`
}

type DeviceCredential struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Secret   string `json:"secret"`
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

type DeviceShareInput struct {
	UUID     string
	Username string
}

type DeviceShareDeleteInput struct {
	UID  string
	UUID string
}

type DeviceShareFeedbackInput struct {
	MsgID  string
	Status int
}

type DeviceUpgradeInput struct {
	UUID string
	Flag string
}

type DeviceRemoveInput struct {
	UUID      string
	CleanData bool
}

type MQTTClient interface {
	Publish(ctx context.Context, topic string, payload any) error
}

type RealtimeNotifier interface {
	NotifyDeviceBind(ctx context.Context, uid, uuid string)
	NotifyDeviceUnBind(ctx context.Context, uid, uuid string)
}

type Service struct {
	deviceRepo     *devicerepo.Repository
	homeRepo       *homerepo.Repository
	userRepo       *userrepo.Repository
	redisClient    *redisx.Client
	shadowStore    shadow.Store
	eventStore     eventstore.Store
	mqttClient     MQTTClient
	deviceModels   []DeviceModelItem
	deviceUpgrades []config.DeviceUpgradeConfig
	clock          func() time.Time
	shareMsgIDGen  func() string
	realtime       RealtimeNotifier
}

func New(deviceRepo *devicerepo.Repository, homeRepo *homerepo.Repository, userRepo *userrepo.Repository, redisClient *redisx.Client, shadowStore shadow.Store, eventStore eventstore.Store, mqttClient MQTTClient, cfg config.Config) *Service {
	return &Service{
		deviceRepo:     deviceRepo,
		homeRepo:       homeRepo,
		userRepo:       userRepo,
		redisClient:    redisClient,
		shadowStore:    shadowStore,
		eventStore:     eventStore,
		mqttClient:     mqttClient,
		deviceModels:   buildDeviceModels(cfg),
		deviceUpgrades: buildDeviceUpgrades(cfg),
		clock:          time.Now,
		shareMsgIDGen: func() string {
			return fmt.Sprintf("msg_%d", time.Now().UnixNano())
		},
	}
}

func (s *Service) WithClock(clock func() time.Time) *Service {
	s.clock = clock
	return s
}

func (s *Service) WithShareMessageIDGenerator(generator func() string) *Service {
	s.shareMsgIDGen = generator
	return s
}

func (s *Service) WithMQTTClient(client MQTTClient) *Service {
	s.mqttClient = client
	return s
}

func (s *Service) WithRealtimeNotifier(notifier RealtimeNotifier) *Service {
	s.realtime = notifier
	return s
}

func (s *Service) Bind(input BindInput) (*DeviceCredential, error) {
	if strings.TrimSpace(input.Model) == "" ||
		strings.TrimSpace(input.UUID) == "" ||
		strings.TrimSpace(input.AppID) == "" ||
		strings.TrimSpace(input.UID) == "" ||
		strings.TrimSpace(input.MAC) == "" ||
		strings.TrimSpace(input.Zone) == "" ||
		strings.TrimSpace(input.Version) == "" {
		return nil, ErrInvalidInput
	}

	if _, err := s.mustFindUser(strings.TrimSpace(input.UID)); err != nil {
		return nil, err
	}

	now := time.Now().Unix()
	password := redisx.RandString(16)
	deviceSecret := ""
	device, err := s.deviceRepo.FindDeviceByUUID(strings.TrimSpace(input.UUID))
	if err != nil {
		if !devicerepo.IsNotFound(err) {
			return nil, err
		}

		deviceSecret = password
		if err := s.deviceRepo.CreateDevice(&devicemodel.Device{
			UUID:           strings.TrimSpace(input.UUID),
			MAC:            strings.TrimSpace(input.MAC),
			DeviceID:       strings.TrimSpace(input.UUID),
			UID:            strings.TrimSpace(input.UID),
			BindType:       1,
			Secret:         deviceSecret,
			ModelCode:      strings.TrimSpace(input.Model),
			CurrentVersion: strings.TrimSpace(input.Version),
			Zone:           strings.TrimSpace(input.Zone),
			Name:           strings.TrimSpace(input.Model),
			Online:         0,
			UpdateTime:     now,
			ActiveTime:     now,
			FirstBindTime:  now,
			BindTime:       now,
		}); err != nil {
			return nil, err
		}
	} else {
		deviceSecret = strings.TrimSpace(device.Secret)
		attrs := map[string]any{
			"uid":             strings.TrimSpace(input.UID),
			"bind_type":       1,
			"bind_time":       now,
			"model_code":      strings.TrimSpace(input.Model),
			"current_version": strings.TrimSpace(input.Version),
			"mac":             strings.TrimSpace(input.MAC),
			"zone":            strings.TrimSpace(input.Zone),
			"active_time":     now,
		}
		if strings.TrimSpace(device.DeviceID) == "" {
			attrs["device_id"] = strings.TrimSpace(input.UUID)
		}
		if deviceSecret == "" {
			deviceSecret = password
			attrs["secret"] = deviceSecret
		}
		if strings.TrimSpace(device.Name) == "" {
			attrs["name"] = strings.TrimSpace(input.Model)
		}
		if err := s.deviceRepo.UpdateDeviceByID(device.ID, attrs); err != nil {
			return nil, err
		}
	}

	ctx := context.Background()
	if s.redisClient != nil {
		if err := s.redisClient.SetMQTTCredentials(ctx, strings.TrimSpace(input.UUID), password, false); err != nil {
			return nil, err
		}
		if err := s.redisClient.SetMQTTACL(ctx, strings.TrimSpace(input.UUID), redisx.BuildDeviceACL(strings.TrimSpace(input.Model), strings.TrimSpace(input.UUID))); err != nil {
			return nil, err
		}
		if err := s.redisClient.SetDeviceBinding(ctx, strings.TrimSpace(input.UUID), strings.TrimSpace(input.UID), 1); err != nil {
			return nil, err
		}
	}
	if s.shadowStore != nil {
		if err := s.shadowStore.EnsureDevice(ctx, strings.TrimSpace(input.UUID), strings.TrimSpace(input.UID)); err != nil {
			return nil, err
		}
	}
	if s.realtime != nil {
		s.realtime.NotifyDeviceBind(ctx, strings.TrimSpace(input.UID), strings.TrimSpace(input.UUID))
	}
	return &DeviceCredential{
		Username: strings.TrimSpace(input.UUID),
		Password: password,
		Secret:   deviceSecret,
	}, nil
}

func (s *Service) DeviceLogin(input DeviceLoginInput) (*DeviceCredential, error) {
	if strings.TrimSpace(input.Model) == "" ||
		strings.TrimSpace(input.UUID) == "" ||
		strings.TrimSpace(input.UID) == "" ||
		strings.TrimSpace(input.Zone) == "" ||
		strings.TrimSpace(input.Version) == "" {
		return nil, ErrInvalidInput
	}

	if _, err := s.mustFindUser(strings.TrimSpace(input.UID)); err != nil {
		return nil, err
	}

	device, err := s.deviceRepo.FindDeviceByUUID(strings.TrimSpace(input.UUID))
	if err != nil {
		if devicerepo.IsNotFound(err) {
			return nil, ErrDeviceNotFound
		}
		return nil, err
	}
	if device.UID != strings.TrimSpace(input.UID) {
		return nil, ErrDeviceForbidden
	}

	if err := s.deviceRepo.UpdateDeviceByID(device.ID, map[string]any{
		"model_code":      strings.TrimSpace(input.Model),
		"current_version": strings.TrimSpace(input.Version),
		"zone":            strings.TrimSpace(input.Zone),
		"active_time":     time.Now().Unix(),
	}); err != nil {
		return nil, err
	}

	password := redisx.RandString(16)
	ctx := context.Background()
	if s.redisClient != nil {
		if err := s.redisClient.SetMQTTCredentials(ctx, strings.TrimSpace(input.UUID), password, false); err != nil {
			return nil, err
		}
		if err := s.redisClient.SetMQTTACL(ctx, strings.TrimSpace(input.UUID), redisx.BuildDeviceACL(strings.TrimSpace(input.Model), strings.TrimSpace(input.UUID))); err != nil {
			return nil, err
		}
	}

	return &DeviceCredential{
		Username: strings.TrimSpace(input.UUID),
		Password: password,
		Secret:   strings.TrimSpace(device.Secret),
	}, nil
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

	return s.mapVisibleDevices(devices), nil
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

func (s *Service) Remove(uid string, input DeviceRemoveInput) error {
	user, err := s.mustFindUser(uid)
	if err != nil {
		return err
	}
	if strings.TrimSpace(input.UUID) == "" {
		return ErrInvalidInput
	}

	device, err := s.deviceRepo.FindDeviceByUUID(strings.TrimSpace(input.UUID))
	if err != nil {
		if devicerepo.IsNotFound(err) {
			return ErrDeviceNotFound
		}
		return err
	}
	if device.UID != user.UID {
		return ErrDeviceForbidden
	}

	payload := map[string]any{
		"msg_id": fmt.Sprintf("unbind_%d", time.Now().UnixNano()),
		"time":   time.Now().Unix(),
		"data": map[string]any{
			"clean_data": boolToInt(input.CleanData),
		},
	}
	if s.mqttClient != nil && strings.TrimSpace(device.ModelCode) != "" {
		topic := fmt.Sprintf("/thing/%s/%s/func/UnBind", strings.TrimSpace(device.ModelCode), strings.TrimSpace(device.UUID))
		_ = s.mqttClient.Publish(context.Background(), topic, payload)
	}

	now := time.Now()
	if err := s.deviceRepo.SoftDeleteDeviceByID(device.ID, now); err != nil {
		return err
	}
	if s.shadowStore != nil {
		if err := s.shadowStore.Delete(context.Background(), device.UUID); err != nil {
			return err
		}
	}
	if s.redisClient != nil {
		if err := s.redisClient.DeleteMQTTCredentials(context.Background(), device.UUID); err != nil {
			return err
		}
		if err := s.redisClient.DeleteMQTTACL(context.Background(), device.UUID); err != nil {
			return err
		}
		if err := s.redisClient.DeleteDeviceBinding(context.Background(), device.UUID); err != nil {
			return err
		}
	}
	if s.realtime != nil {
		s.realtime.NotifyDeviceUnBind(context.Background(), user.UID, device.UUID)
	}
	return nil
}

func (s *Service) Models(uid string) ([]DeviceModelItem, error) {
	user, err := s.mustFindUser(uid)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, ErrUserNotFound
	}

	result := make([]DeviceModelItem, len(s.deviceModels))
	copy(result, s.deviceModels)
	return result, nil
}

func (s *Service) UpgradedVersion(uid string, input DeviceUpgradeInput) (DeviceUpgradeResult, error) {
	user, err := s.mustFindUser(uid)
	if err != nil {
		return DeviceUpgradeResult{}, err
	}
	if strings.TrimSpace(input.UUID) == "" || strings.TrimSpace(input.Flag) == "" {
		return DeviceUpgradeResult{}, ErrInvalidInput
	}

	device, err := s.deviceRepo.FindDeviceByUUID(strings.TrimSpace(input.UUID))
	if err != nil {
		if devicerepo.IsNotFound(err) {
			return DeviceUpgradeResult{}, ErrDeviceNotFound
		}
		return DeviceUpgradeResult{}, err
	}

	if _, err := s.deviceRepo.FindVisibleDeviceByUUID(user.ID, user.UID, strings.TrimSpace(input.UUID)); err != nil {
		if devicerepo.IsNotFound(err) {
			return DeviceUpgradeResult{}, ErrDeviceForbidden
		}
		return DeviceUpgradeResult{}, err
	}

	modelCode := strings.TrimSpace(device.ModelCode)
	if modelCode == "" {
		modelCode = strings.TrimSpace(device.Name)
	}
	currentVersion := strings.TrimSpace(device.CurrentVersion)

	for _, candidate := range s.deviceUpgrades {
		if strings.TrimSpace(candidate.ModelCode) != modelCode {
			continue
		}
		if strings.TrimSpace(candidate.Flag) != strings.TrimSpace(input.Flag) {
			continue
		}
		if currentVersion == "" || currentVersion == strings.TrimSpace(candidate.Version) {
			return DeviceUpgradeResult{Has: false, Version: nil}, nil
		}
		return DeviceUpgradeResult{
			Has: true,
			Version: &DeviceUpgradeVersion{
				Flag:    strings.TrimSpace(candidate.Flag),
				Version: strings.TrimSpace(candidate.Version),
			},
		}, nil
	}

	return DeviceUpgradeResult{Has: false, Version: nil}, nil
}

func (s *Service) Share(uid string, input DeviceShareInput) error {
	user, err := s.mustFindUser(uid)
	if err != nil {
		return err
	}
	if strings.TrimSpace(input.UUID) == "" || strings.TrimSpace(input.Username) == "" {
		return ErrInvalidInput
	}

	device, err := s.deviceRepo.FindDeviceByUUID(strings.TrimSpace(input.UUID))
	if err != nil {
		if devicerepo.IsNotFound(err) {
			return ErrDeviceNotFound
		}
		return err
	}
	if device.UID != user.UID {
		return ErrDeviceForbidden
	}

	targetUser, err := s.userRepo.FindUserByUsername(strings.TrimSpace(input.Username))
	if err != nil {
		if userrepo.IsNotFound(err) {
			return ErrUserNotFound
		}
		return err
	}
	if targetUser.ID == user.ID {
		return ErrDeviceShareInvalid
	}

	if _, err := s.deviceRepo.FindActiveDeviceShareMemberByInternalDeviceIDAndUserID(device.ID, targetUser.ID); err == nil {
		return ErrDeviceShareInvalid
	} else if !devicerepo.IsNotFound(err) {
		return err
	}

	if _, err := s.deviceRepo.FindActiveDeviceShareInviteByInternalDeviceIDAndToUserID(device.ID, targetUser.ID); err == nil {
		return ErrDeviceShareInvalid
	} else if !devicerepo.IsNotFound(err) {
		return err
	}

	return s.deviceRepo.CreateDeviceShareInvite(&devicemodel.DeviceShareInvite{
		MsgID:      s.shareMsgIDGen(),
		DeviceID:   device.ID,
		FromUserID: user.ID,
		ToUserID:   targetUser.ID,
		Status:     0,
		IsRead:     0,
	})
}

func (s *Service) ShareRecords(uid, uuid string) ([]ShareRecordItem, error) {
	user, err := s.mustFindUser(uid)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(uuid) == "" {
		return nil, ErrInvalidInput
	}

	device, err := s.deviceRepo.FindDeviceByUUID(strings.TrimSpace(uuid))
	if err != nil {
		if devicerepo.IsNotFound(err) {
			return nil, ErrDeviceNotFound
		}
		return nil, err
	}
	if device.UID != user.UID {
		return nil, ErrDeviceForbidden
	}

	members, err := s.deviceRepo.ListActiveDeviceShareMembersByInternalDeviceID(device.ID)
	if err != nil {
		return nil, err
	}
	invites, err := s.deviceRepo.ListDeviceShareInvitesByInternalDeviceID(device.ID)
	if err != nil {
		return nil, err
	}

	type aggregate struct {
		item      ShareRecordItem
		updatedAt time.Time
		active    bool
	}

	records := make(map[uint]aggregate, len(members)+len(invites))
	for _, member := range members {
		records[member.User.ID] = aggregate{
			item: ShareRecordItem{
				Username: member.User.Username,
				UUID:     device.UUID,
				UID:      member.User.UID,
				Status:   1,
				Role:     member.Member.Role,
			},
			updatedAt: member.Member.CreatedAt,
			active:    true,
		}
	}

	for _, invite := range invites {
		existing, ok := records[invite.User.ID]
		if ok {
			if existing.active {
				if invite.Invite.CreatedAt.After(existing.updatedAt) {
					existing.updatedAt = invite.Invite.CreatedAt
					records[invite.User.ID] = existing
				}
				continue
			}
			if !invite.Invite.CreatedAt.After(existing.updatedAt) {
				continue
			}
		}

		status := invite.Invite.Status
		if status == 1 {
			status = 3
		}

		records[invite.User.ID] = aggregate{
			item: ShareRecordItem{
				Username: invite.User.Username,
				UUID:     device.UUID,
				UID:      invite.User.UID,
				Status:   status,
				Role:     2,
			},
			updatedAt: invite.Invite.CreatedAt,
			active:    false,
		}
	}

	result := make([]aggregate, 0, len(records))
	for _, record := range records {
		result = append(result, record)
	}

	sort.Slice(result, func(i, j int) bool {
		if result[i].updatedAt.Equal(result[j].updatedAt) {
			return result[i].item.UID < result[j].item.UID
		}
		return result[i].updatedAt.After(result[j].updatedAt)
	})

	items := make([]ShareRecordItem, 0, len(result))
	for _, record := range result {
		items = append(items, record.item)
	}
	return items, nil
}

func (s *Service) ShareDelete(uid string, input DeviceShareDeleteInput) error {
	user, err := s.mustFindUser(uid)
	if err != nil {
		return err
	}
	if strings.TrimSpace(input.UUID) == "" || strings.TrimSpace(input.UID) == "" {
		return ErrInvalidInput
	}
	if strings.TrimSpace(input.UID) == user.UID {
		return ErrDeviceShareInvalid
	}

	device, err := s.deviceRepo.FindDeviceByUUID(strings.TrimSpace(input.UUID))
	if err != nil {
		if devicerepo.IsNotFound(err) {
			return ErrDeviceNotFound
		}
		return err
	}
	if device.UID != user.UID {
		return ErrDeviceForbidden
	}

	targetUser, err := s.userRepo.FindUserByUID(strings.TrimSpace(input.UID))
	if err != nil {
		if userrepo.IsNotFound(err) {
			return ErrUserNotFound
		}
		return err
	}

	member, err := s.deviceRepo.FindActiveDeviceShareMemberByInternalDeviceIDAndUserID(device.ID, targetUser.ID)
	if err != nil && !devicerepo.IsNotFound(err) {
		return err
	}
	pendingInvite, inviteErr := s.deviceRepo.FindActiveDeviceShareInviteByInternalDeviceIDAndToUserID(device.ID, targetUser.ID)
	if inviteErr != nil && !devicerepo.IsNotFound(inviteErr) {
		return inviteErr
	}

	if member == nil && pendingInvite == nil {
		return nil
	}

	now := s.clock()
	return s.deviceRepo.WithTx(func(txRepo *devicerepo.Repository) error {
		if member != nil {
			if err := txRepo.SoftDeleteDeviceShareMemberByID(member.ID, now); err != nil {
				return err
			}
		}
		if pendingInvite != nil {
			return txRepo.SoftDeleteActiveDeviceShareInvitesByInternalDeviceIDAndToUserID(device.ID, targetUser.ID, now)
		}
		return nil
	})
}

func (s *Service) ShareFeedback(uid string, input DeviceShareFeedbackInput) error {
	if strings.TrimSpace(uid) == "" || strings.TrimSpace(input.MsgID) == "" {
		return ErrInvalidInput
	}
	if input.Status != 1 && input.Status != 2 {
		return ErrInvalidInput
	}

	user, err := s.mustFindUser(strings.TrimSpace(uid))
	if err != nil {
		return err
	}

	invite, err := s.deviceRepo.FindDeviceShareInviteByMsgID(strings.TrimSpace(input.MsgID))
	if err != nil {
		if devicerepo.IsNotFound(err) {
			return ErrDeviceShareInvalid
		}
		return err
	}
	if invite.ToUserID != user.ID {
		return ErrDeviceForbidden
	}
	if invite.Status != 0 {
		return ErrDeviceShareInvalid
	}

	device, err := s.deviceRepo.FindDeviceByInternalID(invite.DeviceID)
	if err != nil {
		if devicerepo.IsNotFound(err) {
			return ErrDeviceNotFound
		}
		return err
	}
	if device.UID == user.UID {
		return ErrDeviceShareInvalid
	}

	if _, err := s.deviceRepo.FindActiveDeviceShareMemberByInternalDeviceIDAndUserID(invite.DeviceID, user.ID); err == nil {
		return ErrDeviceShareInvalid
	} else if !devicerepo.IsNotFound(err) {
		return err
	}

	return s.deviceRepo.WithTx(func(txRepo *devicerepo.Repository) error {
		if input.Status == 1 {
			if err := txRepo.CreateDeviceShareMember(&devicemodel.DeviceShareMember{
				DeviceID:        invite.DeviceID,
				UserID:          user.ID,
				Role:            2,
				GrantedByUserID: invite.FromUserID,
			}); err != nil {
				return err
			}
		}

		if err := txRepo.UpdateDeviceShareInviteByID(invite.ID, map[string]any{
			"status":  input.Status,
			"is_read": 1,
		}); err != nil {
			return err
		}

		return txRepo.CreateDeviceShareFeedbackMessage(&devicemodel.DeviceShareFeedbackMessage{
			MsgID:      s.shareMsgIDGen(),
			DeviceID:   invite.DeviceID,
			FromUserID: user.ID,
			ToUserID:   invite.FromUserID,
			Status:     input.Status,
			IsRead:     0,
		})
	})
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

func (s *Service) mapVisibleDevices(items []devicerepo.VisibleDevice) []DeviceItem {
	result := make([]DeviceItem, 0, len(items))
	for _, item := range items {
		state := DeviceState{
			Desired:  map[string]any{},
			Reported: map[string]any{},
		}
		if s.shadowStore != nil {
			if shadowDoc, err := s.shadowStore.Get(context.Background(), item.UUID); err == nil && shadowDoc != nil {
				state.Desired = shadowDoc.State.Desired
				state.Reported = shadowDoc.State.Reported
			}
		}
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
			State:         state,
		})
	}
	return result
}

func boolToInt(value bool) int {
	if value {
		return 1
	}
	return 0
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

func buildDeviceModels(cfg config.Config) []DeviceModelItem {
	models := cfg.DeviceModels
	if len(models) == 0 && strings.TrimSpace(cfg.DeviceModelsRaw) != "" {
		_ = json.Unmarshal([]byte(cfg.DeviceModelsRaw), &models)
	}
	if len(models) > 0 {
		items := make([]DeviceModelItem, 0, len(models))
		for _, model := range models {
			items = append(items, DeviceModelItem{
				ModelCode:   strings.TrimSpace(model.ModelCode),
				Status:      model.Status,
				ModelName:   strings.TrimSpace(model.ModelName),
				Category:    strings.TrimSpace(model.Category),
				ShowName:    strings.TrimSpace(model.ShowName),
				DefaultName: strings.TrimSpace(model.DefaultName),
				Thumbnail:   strings.TrimSpace(model.Thumbnail),
			})
		}
		sort.Slice(items, func(i, j int) bool {
			return items[i].ModelCode < items[j].ModelCode
		})
		return items
	}

	modelSecrets := cfg.DeviceModelSecrets
	if len(modelSecrets) == 0 && strings.TrimSpace(cfg.DeviceModelSecretsRaw) != "" {
		_ = json.Unmarshal([]byte(cfg.DeviceModelSecretsRaw), &modelSecrets)
	}

	items := make([]DeviceModelItem, 0, len(modelSecrets))
	for code := range modelSecrets {
		trimmed := strings.TrimSpace(code)
		if trimmed == "" {
			continue
		}
		items = append(items, DeviceModelItem{
			ModelCode:   trimmed,
			Status:      1,
			ModelName:   trimmed,
			Category:    "lock",
			ShowName:    trimmed,
			DefaultName: trimmed,
			Thumbnail:   "",
		})
	}
	sort.Slice(items, func(i, j int) bool {
		return items[i].ModelCode < items[j].ModelCode
	})
	return items
}

func buildDeviceUpgrades(cfg config.Config) []config.DeviceUpgradeConfig {
	upgrades := cfg.DeviceUpgrades
	if len(upgrades) == 0 && strings.TrimSpace(cfg.DeviceUpgradesRaw) != "" {
		_ = json.Unmarshal([]byte(cfg.DeviceUpgradesRaw), &upgrades)
	}
	return append([]config.DeviceUpgradeConfig(nil), upgrades...)
}
