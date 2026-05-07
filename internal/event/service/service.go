package service

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"time"

	devicerepo "has-smartlock-service/internal/device/repository"
	eventrepo "has-smartlock-service/internal/event/repository"
	homerepo "has-smartlock-service/internal/home/repository"
	eventstore "has-smartlock-service/internal/realtime/eventstore"
	usermodel "has-smartlock-service/internal/user/model"
	userrepo "has-smartlock-service/internal/user/repository"
)

const pageSize = 20

var (
	ErrInvalidInput    = errors.New("invalid input")
	ErrUserNotFound    = errors.New("user not found")
	ErrHomeNotFound    = errors.New("home not found")
	ErrDeviceNotFound  = errors.New("device not found")
	ErrDeviceForbidden = errors.New("device forbidden")
	ErrEventNotFound   = errors.New("event not found")
	ErrEventForbidden  = errors.New("event forbidden")
)

type ListInput struct {
	Date      string
	UUID      string
	HomeID    string
	Type      string
	StartTime string
}

type ExistDayInput struct {
	Month  string
	UUID   string
	HomeID string
}

type ListPayload struct {
	Result int `json:"result"`
}

type EventItem struct {
	ID         string       `json:"id"`
	UUID       string       `json:"uuid"`
	DeviceName string       `json:"device_name,omitempty"`
	Type       int          `json:"type"`
	IsRead     int          `json:"is_read"`
	Time       int64        `json:"time"`
	DeviceTime int64        `json:"device_time"`
	Thumbnail  string       `json:"thumbnail,omitempty"`
	Payload    *ListPayload `json:"payload,omitempty"`
}

type ListResult struct {
	Has  bool        `json:"has"`
	List []EventItem `json:"list"`
}

type UnreadNumResult struct {
	Number int64 `json:"number"`
}

type Service struct {
	eventRepo  *eventrepo.Repository
	eventStore eventstore.Store
	deviceRepo *devicerepo.Repository
	homeRepo   *homerepo.Repository
	userRepo   *userrepo.Repository
}

func New(eventRepo *eventrepo.Repository, eventStore eventstore.Store, deviceRepo *devicerepo.Repository, homeRepo *homerepo.Repository, userRepo *userrepo.Repository) *Service {
	return &Service{
		eventRepo:  eventRepo,
		eventStore: eventStore,
		deviceRepo: deviceRepo,
		homeRepo:   homeRepo,
		userRepo:   userRepo,
	}
}

func (s *Service) List(uid string, input ListInput) (ListResult, error) {
	user, err := s.mustFindUser(uid)
	if err != nil {
		return ListResult{}, err
	}

	filter, err := s.buildListFilter(user, input, pageSize+1)
	if err != nil {
		return ListResult{}, err
	}

	rows, err := s.requireStore().List(context.Background(), eventstore.Filter{
		UID:       user.UID,
		UUID:      filter.DeviceUUID,
		HomeID:    homeBusinessIDToString(filter.HomeID),
		Types:     filter.Types,
		StartTime: filter.StartTime,
		StartAt:   filter.StartAt,
		EndAt:     filter.EndAt,
		Limit:     int64(pageSize + 1),
	})
	if err != nil {
		return ListResult{}, err
	}
	result := ListResult{
		Has:  len(rows) > pageSize,
		List: make([]EventItem, 0, min(len(rows), pageSize)),
	}
	if len(rows) > pageSize {
		rows = rows[:pageSize]
	}
	for _, row := range rows {
		payload, err := parseMapPayload(row.Payload)
		if err != nil {
			return ListResult{}, err
		}
		isRead := 0
		if len(row.BelongTo) > 0 {
			isRead = row.BelongTo[0].IsRead
		}
		result.List = append(result.List, EventItem{
			ID:         row.ID,
			UUID:       row.UUID,
			DeviceName: row.DeviceName,
			Type:       row.Type,
			IsRead:     isRead,
			Time:       row.Time,
			DeviceTime: row.DeviceTime,
			Thumbnail:  row.Thumbnail,
			Payload:    payload,
		})
	}
	return result, nil
}

func (s *Service) UnreadNum(uid, uuid string) (UnreadNumResult, error) {
	user, err := s.mustFindUser(uid)
	if err != nil {
		return UnreadNumResult{}, err
	}

	deviceUUID := strings.TrimSpace(uuid)
	if deviceUUID == "" {
		return UnreadNumResult{}, ErrInvalidInput
	}
	if err := s.ensureVisibleDevice(user, deviceUUID); err != nil {
		return UnreadNumResult{}, err
	}

	count, err := s.requireStore().CountUnread(context.Background(), user.UID, deviceUUID)
	if err != nil {
		return UnreadNumResult{}, err
	}
	return UnreadNumResult{Number: count}, nil
}

func (s *Service) ExistDay(uid string, input ExistDayInput) (map[string]int, error) {
	user, err := s.mustFindUser(uid)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(input.Month) == "" {
		return nil, ErrInvalidInput
	}

	startAt, endAt, err := parseMonthRange(strings.TrimSpace(input.Month))
	if err != nil {
		return nil, ErrInvalidInput
	}

	filter, err := s.buildVisibilityFilter(user, strings.TrimSpace(input.UUID), strings.TrimSpace(input.HomeID))
	if err != nil {
		return nil, err
	}
	filter.StartAt = &startAt
	filter.EndAt = &endAt

	return s.requireStore().CountDays(context.Background(), eventstore.Filter{
		UID:     user.UID,
		UUID:    filter.DeviceUUID,
		HomeID:  homeBusinessIDToString(filter.HomeID),
		StartAt: filter.StartAt,
		EndAt:   filter.EndAt,
		Types:   filter.Types,
	})
}

func (s *Service) Read(uid, msgID, uuid string) error {
	user, err := s.mustFindUser(uid)
	if err != nil {
		return err
	}

	if strings.TrimSpace(msgID) == "" || strings.TrimSpace(uuid) == "" {
		return ErrInvalidInput
	}
	if err := s.ensureVisibleDevice(user, strings.TrimSpace(uuid)); err != nil {
		if errors.Is(err, ErrDeviceForbidden) {
			return ErrEventForbidden
		}
		return err
	}
	event, err := s.requireStore().GetVisible(context.Background(), user.UID, strings.TrimSpace(msgID))
	if err != nil {
		return ErrEventNotFound
	}
	if event.UUID != strings.TrimSpace(uuid) {
		return ErrEventNotFound
	}
	return s.requireStore().MarkRead(context.Background(), user.UID, strings.TrimSpace(msgID))
}

func (s *Service) Delete(uid, msgID, uuid string) error {
	user, err := s.mustFindUser(uid)
	if err != nil {
		return err
	}

	if strings.TrimSpace(msgID) == "" || strings.TrimSpace(uuid) == "" {
		return ErrInvalidInput
	}
	if err := s.ensureVisibleDevice(user, strings.TrimSpace(uuid)); err != nil {
		if errors.Is(err, ErrDeviceForbidden) {
			return ErrEventForbidden
		}
		return err
	}
	event, err := s.requireStore().GetVisible(context.Background(), user.UID, strings.TrimSpace(msgID))
	if err != nil {
		return ErrEventNotFound
	}
	if event.UUID != strings.TrimSpace(uuid) {
		return ErrEventNotFound
	}
	return s.requireStore().SoftDelete(context.Background(), user.UID, strings.TrimSpace(msgID))
}

func (s *Service) buildListFilter(user *usermodel.User, input ListInput, limit int) (eventrepo.EventListFilter, error) {
	filter := eventrepo.EventListFilter{
		UserID: user.ID,
		UID:    user.UID,
		Limit:  limit,
	}

	filter, err := s.buildVisibilityFilter(user, strings.TrimSpace(input.UUID), strings.TrimSpace(input.HomeID))
	if err != nil {
		return eventrepo.EventListFilter{}, err
	}
	filter.Limit = limit

	if strings.TrimSpace(input.StartTime) != "" {
		startTime, err := strconv.ParseInt(strings.TrimSpace(input.StartTime), 10, 64)
		if err != nil {
			return eventrepo.EventListFilter{}, ErrInvalidInput
		}
		filter.StartTime = &startTime
	}

	if strings.TrimSpace(input.Date) != "" {
		startAt, endAt, err := parseDateRange(strings.TrimSpace(input.Date))
		if err != nil {
			return eventrepo.EventListFilter{}, ErrInvalidInput
		}
		filter.StartAt = &startAt
		filter.EndAt = &endAt
	}

	if strings.TrimSpace(input.Type) != "" {
		types, err := parseTypes(strings.TrimSpace(input.Type))
		if err != nil {
			return eventrepo.EventListFilter{}, ErrInvalidInput
		}
		filter.Types = types
	}

	return filter, nil
}

func (s *Service) buildVisibilityFilter(user *usermodel.User, deviceUUID, homeBusinessID string) (eventrepo.EventListFilter, error) {
	filter := eventrepo.EventListFilter{
		UserID: user.ID,
		UID:    user.UID,
	}

	if deviceUUID != "" {
		if err := s.ensureVisibleDevice(user, deviceUUID); err != nil {
			return eventrepo.EventListFilter{}, err
		}
		filter.DeviceUUID = deviceUUID
	}

	if homeBusinessID != "" {
		homeWithMember, err := s.homeRepo.FindHomeMembershipByHomeIDAndUserID(homeBusinessID, user.ID)
		if err != nil {
			if homerepo.IsNotFound(err) {
				return eventrepo.EventListFilter{}, ErrHomeNotFound
			}
			return eventrepo.EventListFilter{}, err
		}
		filter.HomeID = &homeWithMember.Home.ID
	}

	return filter, nil
}

func (s *Service) ensureVisibleDevice(user *usermodel.User, uuid string) error {
	device, err := s.deviceRepo.FindDeviceByUUID(uuid)
	if err != nil {
		if devicerepo.IsNotFound(err) {
			return ErrDeviceNotFound
		}
		return err
	}

	if device.UID == user.UID {
		return nil
	}

	link, err := s.homeRepo.FindActiveHomeDeviceByInternalDeviceID(device.ID)
	if err != nil {
		if homerepo.IsNotFound(err) {
			return ErrDeviceForbidden
		}
		return err
	}

	if _, err := s.homeRepo.FindHomeMembershipByInternalHomeIDAndUserID(link.HomeID, user.ID); err != nil {
		if homerepo.IsNotFound(err) {
			return ErrDeviceForbidden
		}
		return err
	}

	return nil
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

func parseDateRange(date string) (int64, int64, error) {
	start, err := time.ParseInLocation("2006-01-02", date, time.Local)
	if err != nil {
		return 0, 0, err
	}
	end := start.Add(24 * time.Hour)
	return start.Unix(), end.Unix(), nil
}

func parseMonthRange(month string) (int64, int64, error) {
	start, err := time.ParseInLocation("2006-01", month, time.Local)
	if err != nil {
		return 0, 0, err
	}
	end := start.AddDate(0, 1, 0)
	return start.Unix(), end.Unix(), nil
}

func parseTypes(raw string) ([]int, error) {
	parts := strings.Split(raw, ",")
	result := make([]int, 0, len(parts))
	for _, part := range parts {
		value := strings.TrimSpace(part)
		if value == "" {
			return nil, ErrInvalidInput
		}
		parsed, err := strconv.Atoi(value)
		if err != nil {
			return nil, err
		}
		result = append(result, parsed)
	}
	return result, nil
}

func parsePayload(raw string) (*ListPayload, error) {
	if strings.TrimSpace(raw) == "" {
		return nil, nil
	}

	var payload ListPayload
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		return nil, err
	}
	return &payload, nil
}

func parseMapPayload(raw map[string]any) (*ListPayload, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	body, err := json.Marshal(raw)
	if err != nil {
		return nil, err
	}
	return parsePayload(string(body))
}

func homeBusinessIDToString(homeID *uint) string {
	if homeID == nil {
		return ""
	}
	return strconv.FormatUint(uint64(*homeID), 10)
}

func (s *Service) requireStore() eventstore.Store {
	return s.eventStore
}
