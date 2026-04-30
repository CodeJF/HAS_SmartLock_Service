package repository

import (
	"errors"

	"gorm.io/gorm"

	eventmodel "has-smartlock-service/internal/event/model"
)

type EventListFilter struct {
	UserID     uint
	UID        string
	DeviceUUID string
	HomeID     *uint
	StartTime  *int64
	StartAt    *int64
	EndAt      *int64
	Types      []int
	Limit      int
}

type EventView struct {
	ID         uint
	UUID       string
	DeviceName string
	Type       int
	IsRead     int
	Time       int64
	DeviceTime int64
	Thumbnail  string
	Payload    string
}

type EventDayCount struct {
	Day   string
	Count int64
}

type Repository struct {
	db *gorm.DB
}

func New(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) ListVisibleEvents(filter EventListFilter) ([]EventView, error) {
	var rows []EventView

	err := r.visibleEventsQuery(filter).
		Select("device_events.id, devices.uuid, devices.name as device_name, device_events.event_type as type, COALESCE(device_event_user_states.is_read, 0) as is_read, device_events.event_time as time, device_events.device_time, device_events.thumbnail, device_events.payload").
		Order("device_events.event_time DESC, device_events.id DESC").
		Limit(filter.Limit).
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}

	return rows, nil
}

func (r *Repository) CountUnreadVisibleEvents(filter EventListFilter) (int64, error) {
	var count int64

	err := r.visibleEventsQuery(filter).
		Where("COALESCE(device_event_user_states.is_read, 0) = 0").
		Count(&count).Error
	if err != nil {
		return 0, err
	}

	return count, nil
}

func (r *Repository) CountVisibleEventDays(filter EventListFilter) ([]EventDayCount, error) {
	var rows []EventDayCount

	err := r.visibleEventsQuery(filter).
		Select("DATE_FORMAT(FROM_UNIXTIME(device_events.event_time), '%Y-%m-%d') as day, COUNT(*) as count").
		Group("day").
		Order("day ASC").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}

	return rows, nil
}

func (r *Repository) FindEventByID(id uint) (*eventmodel.DeviceEvent, error) {
	var event eventmodel.DeviceEvent
	err := r.db.Where("id = ? AND deleted_at IS NULL", id).Take(&event).Error
	if err != nil {
		return nil, err
	}
	return &event, nil
}

func (r *Repository) FindVisibleEventByID(userID uint, uid string, id uint) (*EventView, error) {
	var row EventView

	err := r.visibleEventsQuery(EventListFilter{
		UserID: userID,
		UID:    uid,
		Limit:  1,
	}).
		Select("device_events.id, devices.uuid, devices.name as device_name, device_events.event_type as type, COALESCE(device_event_user_states.is_read, 0) as is_read, device_events.event_time as time, device_events.device_time, device_events.thumbnail, device_events.payload").
		Where("device_events.id = ?", id).
		Take(&row).Error
	if err != nil {
		return nil, err
	}

	return &row, nil
}

func (r *Repository) FindEventUserStateByEventIDAndUserID(eventID, userID uint) (*eventmodel.DeviceEventUserState, error) {
	var state eventmodel.DeviceEventUserState
	err := r.db.Where("event_id = ? AND user_id = ?", eventID, userID).Take(&state).Error
	if err != nil {
		return nil, err
	}
	return &state, nil
}

func (r *Repository) CreateEventUserState(state *eventmodel.DeviceEventUserState) error {
	return r.db.Create(state).Error
}

func (r *Repository) UpdateEventUserStateByID(id uint, attrs map[string]any) error {
	result := r.db.Model(&eventmodel.DeviceEventUserState{}).
		Where("id = ?", id).
		Updates(attrs)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (r *Repository) visibleEventsQuery(filter EventListFilter) *gorm.DB {
	query := r.db.Table("device_events").
		Joins("JOIN devices ON devices.id = device_events.device_id AND devices.deleted_at IS NULL").
		Joins("LEFT JOIN home_members ON home_members.home_id = device_events.home_id AND home_members.user_id = ? AND home_members.deleted_at IS NULL", filter.UserID).
		Joins("LEFT JOIN device_event_user_states ON device_event_user_states.event_id = device_events.id AND device_event_user_states.user_id = ?", filter.UserID).
		Where("device_events.deleted_at IS NULL").
		Where("(devices.uid = ? OR home_members.id IS NOT NULL)", filter.UID).
		Where("(device_event_user_states.id IS NULL OR device_event_user_states.deleted_at IS NULL)")

	if filter.DeviceUUID != "" {
		query = query.Where("devices.uuid = ?", filter.DeviceUUID)
	}
	if filter.HomeID != nil {
		query = query.Where("device_events.home_id = ?", *filter.HomeID)
	}
	if filter.StartTime != nil {
		query = query.Where("device_events.event_time < ?", *filter.StartTime)
	}
	if filter.StartAt != nil {
		query = query.Where("device_events.event_time >= ?", *filter.StartAt)
	}
	if filter.EndAt != nil {
		query = query.Where("device_events.event_time < ?", *filter.EndAt)
	}
	if len(filter.Types) > 0 {
		query = query.Where("device_events.event_type IN ?", filter.Types)
	}

	return query
}

func IsNotFound(err error) bool {
	return errors.Is(err, gorm.ErrRecordNotFound)
}
