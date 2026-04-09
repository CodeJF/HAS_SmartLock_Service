package repository

import (
	"errors"
	"time"

	"gorm.io/gorm"

	devicemodel "has-smartlock-service/internal/device/model"
	homemodel "has-smartlock-service/internal/home/model"
	usermodel "has-smartlock-service/internal/user/model"
)

type HomeWithMember struct {
	Home homemodel.Home
	Role int
}

type HomeMemberWithUser struct {
	Member homemodel.HomeMember
	User   usermodel.User
}

type HomeDeviceWithDevice struct {
	Link struct {
		ID uint
	}
	Device struct {
		ID            uint
		UUID          string
		DeviceID      string
		UID           string
		BindType      int
		Secret        string
		Name          string
		FirstBindTime int64
		BindTime      int64
	}
}

type Repository struct {
	db *gorm.DB
}

func New(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) WithTx(fn func(txRepo *Repository) error) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		return fn(&Repository{db: tx})
	})
}

func (r *Repository) CreateHome(home *homemodel.Home) error {
	return r.db.Create(home).Error
}

func (r *Repository) CreateHomeMember(member *homemodel.HomeMember) error {
	return r.db.Create(member).Error
}

func (r *Repository) FindActiveHomeDeviceByInternalDeviceID(deviceID uint) (*devicemodel.HomeDevice, error) {
	var link devicemodel.HomeDevice
	err := r.db.Where("device_id = ? AND deleted_at IS NULL", deviceID).Take(&link).Error
	if err != nil {
		return nil, err
	}
	return &link, nil
}

func (r *Repository) FindActiveHomeDeviceByInternalHomeIDAndDeviceID(homeID, deviceID uint) (*devicemodel.HomeDevice, error) {
	var link devicemodel.HomeDevice
	err := r.db.Where("home_id = ? AND device_id = ? AND deleted_at IS NULL", homeID, deviceID).Take(&link).Error
	if err != nil {
		return nil, err
	}
	return &link, nil
}

func (r *Repository) CreateHomeDevice(link *devicemodel.HomeDevice) error {
	return r.db.Create(link).Error
}

func (r *Repository) FindHomeMembershipByInternalHomeIDAndUserID(homeID, userID uint) (*HomeWithMember, error) {
	var row struct {
		Home homemodel.Home `gorm:"embedded;embeddedPrefix:home_"`
		Role int
	}

	err := r.db.Table("home_members").
		Select(
			"homes.id as home_id, homes.home_id as home_home_id, homes.owner_user_id as home_owner_user_id, homes.name as home_name, homes.location as home_location, homes.create_time as home_create_time, homes.created_at as home_created_at, homes.updated_at as home_updated_at, homes.deleted_at as home_deleted_at, home_members.role as role",
		).
		Joins("JOIN homes ON homes.id = home_members.home_id").
		Where("homes.id = ? AND home_members.user_id = ? AND home_members.deleted_at IS NULL AND homes.deleted_at IS NULL", homeID, userID).
		Take(&row).Error
	if err != nil {
		return nil, err
	}

	return &HomeWithMember{
		Home: row.Home,
		Role: row.Role,
	}, nil
}

func (r *Repository) ListHomesByUserID(userID uint) ([]HomeWithMember, error) {
	var rows []struct {
		Home homemodel.Home `gorm:"embedded;embeddedPrefix:home_"`
		Role int
	}

	err := r.db.Table("home_members").
		Select(
			"homes.id as home_id, homes.home_id as home_home_id, homes.owner_user_id as home_owner_user_id, homes.name as home_name, homes.location as home_location, homes.create_time as home_create_time, homes.created_at as home_created_at, homes.updated_at as home_updated_at, homes.deleted_at as home_deleted_at, home_members.role as role",
		).
		Joins("JOIN homes ON homes.id = home_members.home_id").
		Where("home_members.user_id = ? AND home_members.deleted_at IS NULL AND homes.deleted_at IS NULL", userID).
		Order("homes.id DESC").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}

	result := make([]HomeWithMember, 0, len(rows))
	for _, row := range rows {
		result = append(result, HomeWithMember{
			Home: row.Home,
			Role: row.Role,
		})
	}
	return result, nil
}

func (r *Repository) FindHomeMembershipByHomeIDAndUserID(homeID string, userID uint) (*HomeWithMember, error) {
	var row struct {
		Home homemodel.Home `gorm:"embedded;embeddedPrefix:home_"`
		Role int
	}

	err := r.db.Table("home_members").
		Select(
			"homes.id as home_id, homes.home_id as home_home_id, homes.owner_user_id as home_owner_user_id, homes.name as home_name, homes.location as home_location, homes.create_time as home_create_time, homes.created_at as home_created_at, homes.updated_at as home_updated_at, homes.deleted_at as home_deleted_at, home_members.role as role",
		).
		Joins("JOIN homes ON homes.id = home_members.home_id").
		Where("homes.home_id = ? AND home_members.user_id = ? AND home_members.deleted_at IS NULL AND homes.deleted_at IS NULL", homeID, userID).
		Take(&row).Error
	if err != nil {
		return nil, err
	}

	return &HomeWithMember{
		Home: row.Home,
		Role: row.Role,
	}, nil
}

func (r *Repository) ListHomeMembersByInternalHomeID(homeID uint) ([]HomeMemberWithUser, error) {
	var rows []struct {
		Member homemodel.HomeMember `gorm:"embedded;embeddedPrefix:member_"`
		User   usermodel.User       `gorm:"embedded;embeddedPrefix:user_"`
	}

	err := r.db.Table("home_members").
		Select(
			"home_members.id as member_id, home_members.home_id as member_home_id, home_members.user_id as member_user_id, home_members.role as member_role, home_members.accept as member_accept, home_members.created_at as member_created_at, home_members.updated_at as member_updated_at, home_members.deleted_at as member_deleted_at, users.id as user_id, users.uid as user_uid, users.username as user_username, users.country as user_country, users.password_hash as user_password_hash, users.nickname as user_nickname, users.avatar as user_avatar, users.is_debug as user_is_debug, users.register_time as user_register_time, users.created_at as user_created_at, users.updated_at as user_updated_at, users.deleted_at as user_deleted_at",
		).
		Joins("JOIN users ON users.id = home_members.user_id").
		Where("home_members.home_id = ? AND home_members.deleted_at IS NULL AND users.deleted_at IS NULL", homeID).
		Order("home_members.id ASC").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}

	result := make([]HomeMemberWithUser, 0, len(rows))
	for _, row := range rows {
		result = append(result, HomeMemberWithUser{
			Member: row.Member,
			User:   row.User,
		})
	}
	return result, nil
}

func (r *Repository) ListHomeDevicesByInternalHomeID(homeID uint) ([]HomeDeviceWithDevice, error) {
	var rows []struct {
		LinkID              uint   `gorm:"column:link_id"`
		DeviceID            uint   `gorm:"column:device_id"`
		DeviceUUID          string `gorm:"column:device_uuid"`
		DeviceDeviceID      string `gorm:"column:device_device_id"`
		DeviceUID           string `gorm:"column:device_uid"`
		DeviceBindType      int    `gorm:"column:device_bind_type"`
		DeviceSecret        string `gorm:"column:device_secret"`
		DeviceName          string `gorm:"column:device_name"`
		DeviceFirstBindTime int64  `gorm:"column:device_first_bind_time"`
		DeviceBindTime      int64  `gorm:"column:device_bind_time"`
	}

	err := r.db.Table("home_devices").
		Select("home_devices.id as link_id, devices.id as device_id, devices.uuid as device_uuid, devices.device_id as device_device_id, devices.uid as device_uid, devices.bind_type as device_bind_type, devices.secret as device_secret, devices.name as device_name, devices.first_bind_time as device_first_bind_time, devices.bind_time as device_bind_time").
		Joins("JOIN devices ON devices.id = home_devices.device_id").
		Where("home_devices.home_id = ? AND home_devices.deleted_at IS NULL AND devices.deleted_at IS NULL", homeID).
		Order("home_devices.id ASC").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}

	result := make([]HomeDeviceWithDevice, 0, len(rows))
	for _, row := range rows {
		result = append(result, HomeDeviceWithDevice{
			Link: struct {
				ID uint
			}{
				ID: row.LinkID,
			},
			Device: struct {
				ID            uint
				UUID          string
				DeviceID      string
				UID           string
				BindType      int
				Secret        string
				Name          string
				FirstBindTime int64
				BindTime      int64
			}{
				ID:            row.DeviceID,
				UUID:          row.DeviceUUID,
				DeviceID:      row.DeviceDeviceID,
				UID:           row.DeviceUID,
				BindType:      row.DeviceBindType,
				Secret:        row.DeviceSecret,
				Name:          row.DeviceName,
				FirstBindTime: row.DeviceFirstBindTime,
				BindTime:      row.DeviceBindTime,
			},
		})
	}
	return result, nil
}

func (r *Repository) UpdateHomeByInternalID(id uint, attrs map[string]any) error {
	return r.db.Model(&homemodel.Home{}).
		Where("id = ? AND deleted_at IS NULL", id).
		Updates(attrs).Error
}

func (r *Repository) SoftDeleteHomeByInternalID(id uint, now time.Time) error {
	return r.db.Model(&homemodel.Home{}).
		Where("id = ? AND deleted_at IS NULL", id).
		Update("deleted_at", now).Error
}

func (r *Repository) SoftDeleteHomeMembersByInternalHomeID(homeID uint, now time.Time) error {
	return r.db.Model(&homemodel.HomeMember{}).
		Where("home_id = ? AND deleted_at IS NULL", homeID).
		Update("deleted_at", now).Error
}

func (r *Repository) SoftDeleteHomeDeviceByID(id uint, now time.Time) error {
	return r.db.Model(&devicemodel.HomeDevice{}).
		Where("id = ? AND deleted_at IS NULL", id).
		Update("deleted_at", now).Error
}

func IsNotFound(err error) bool {
	return errors.Is(err, gorm.ErrRecordNotFound)
}
