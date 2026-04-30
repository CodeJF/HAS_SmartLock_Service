package service

import (
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"time"

	devicerepo "has-smartlock-service/internal/device/repository"
	eventmodel "has-smartlock-service/internal/event/model"
	eventrepo "has-smartlock-service/internal/event/repository"
	homerepo "has-smartlock-service/internal/home/repository"
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
	deviceRepo *devicerepo.Repository
	homeRepo   *homerepo.Repository
	userRepo   *userrepo.Repository
}

func New(eventRepo *eventrepo.Repository, deviceRepo *devicerepo.Repository, homeRepo *homerepo.Repository, userRepo *userrepo.Repository) *Service {
	return &Service{
		eventRepo:  eventRepo,
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

	rows, err := s.eventRepo.ListVisibleEvents(filter)
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
		payload, err := parsePayload(row.Payload)
		if err != nil {
			return ListResult{}, err
		}
		result.List = append(result.List, EventItem{
			ID:         strconv.FormatUint(uint64(row.ID), 10),
			UUID:       row.UUID,
			DeviceName: row.DeviceName,
			Type:       row.Type,
			IsRead:     row.IsRead,
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

	count, err := s.eventRepo.CountUnreadVisibleEvents(eventrepo.EventListFilter{
		UserID:     user.ID,
		UID:        user.UID,
		DeviceUUID: deviceUUID,
		Limit:      1,
	})
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

	rows, err := s.eventRepo.CountVisibleEventDays(filter)
	if err != nil {
		return nil, err
	}

	result := make(map[string]int, len(rows))
	for _, row := range rows {
		result[row.Day] = int(row.Count)
	}
	return result, nil
}

func (s *Service) Read(uid, msgID, uuid string) error {
	user, err := s.mustFindUser(uid)
	if err != nil {
		return err
	}

	event, _, state, err := s.findMutableVisibleEvent(user, msgID, uuid)
	if err != nil {
		return err
	}

	if state == nil {
		return s.eventRepo.CreateEventUserState(&eventmodel.DeviceEventUserState{
			EventID: event.ID,
			UserID:  user.ID,
			IsRead:  1,
		})
	}

	return s.eventRepo.UpdateEventUserStateByID(state.ID, map[string]any{
		"is_read": 1,
	})
}

func (s *Service) Delete(uid, msgID, uuid string) error {
	user, err := s.mustFindUser(uid)
	if err != nil {
		return err
	}

	event, _, state, err := s.findMutableVisibleEvent(user, msgID, uuid)
	if err != nil {
		return err
	}

	now := time.Now()
	if state == nil {
		return s.eventRepo.CreateEventUserState(&eventmodel.DeviceEventUserState{
			EventID:   event.ID,
			UserID:    user.ID,
			IsRead:    1,
			DeletedAt: &now,
		})
	}

	return s.eventRepo.UpdateEventUserStateByID(state.ID, map[string]any{
		"is_read":    1,
		"deleted_at": &now,
	})
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

func (s *Service) findMutableVisibleEvent(user *usermodel.User, msgID, uuid string) (*eventmodel.DeviceEvent, *eventrepo.EventView, *eventmodel.DeviceEventUserState, error) {
	eventID, err := parseMessageID(msgID)
	if err != nil {
		return nil, nil, nil, err
	}

	deviceUUID := strings.TrimSpace(uuid)
	if deviceUUID == "" {
		return nil, nil, nil, ErrInvalidInput
	}

	event, err := s.eventRepo.FindEventByID(eventID)
	if err != nil {
		if eventrepo.IsNotFound(err) {
			return nil, nil, nil, ErrEventNotFound
		}
		return nil, nil, nil, err
	}

	state, stateErr := s.eventRepo.FindEventUserStateByEventIDAndUserID(event.ID, user.ID)
	if stateErr != nil && !eventrepo.IsNotFound(stateErr) {
		return nil, nil, nil, stateErr
	}
	if stateErr != nil && eventrepo.IsNotFound(stateErr) {
		state = nil
	}

	view, err := s.eventRepo.FindVisibleEventByID(user.ID, user.UID, event.ID)
	if err != nil {
		if eventrepo.IsNotFound(err) {
			if state != nil && state.DeletedAt != nil {
				return nil, nil, nil, ErrEventNotFound
			}
			return nil, nil, nil, ErrEventForbidden
		}
		return nil, nil, nil, err
	}
	if view.UUID != deviceUUID {
		return nil, nil, nil, ErrEventNotFound
	}

	return event, view, state, nil
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

func parseMessageID(msgID string) (uint, error) {
	value := strings.TrimSpace(msgID)
	if value == "" {
		return 0, ErrInvalidInput
	}

	id, err := strconv.ParseUint(value, 10, 64)
	if err != nil {
		return 0, ErrInvalidInput
	}
	return uint(id), nil
}
