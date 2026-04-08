package repository

import (
	"errors"

	"gorm.io/gorm"

	devicemodel "has-smartlock-service/internal/device/model"
)

type VisibleDevice struct {
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

type Repository struct {
	db *gorm.DB
}

func New(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) ListVisibleDevices(userID uint, uid string) ([]VisibleDevice, error) {
	var rows []VisibleDevice

	err := r.db.Table("devices").
		Select("COALESCE(home_devices.id, devices.id) as id, devices.uuid, devices.device_id, devices.uid, devices.bind_type, devices.secret, devices.name, devices.first_bind_time, devices.bind_time").
		Joins("LEFT JOIN home_devices ON home_devices.device_id = devices.id AND home_devices.deleted_at IS NULL").
		Joins("LEFT JOIN home_members ON home_members.home_id = home_devices.home_id AND home_members.user_id = ? AND home_members.deleted_at IS NULL", userID).
		Where("devices.deleted_at IS NULL AND (devices.uid = ? OR home_members.id IS NOT NULL)", uid).
		Group("devices.id, home_devices.id, devices.uuid, devices.device_id, devices.uid, devices.bind_type, devices.secret, devices.name, devices.first_bind_time, devices.bind_time").
		Order("devices.id DESC").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}

	return rows, nil
}

func (r *Repository) ListDevicesByHomeID(homeID uint) ([]VisibleDevice, error) {
	var rows []VisibleDevice

	err := r.db.Table("home_devices").
		Select("home_devices.id as id, devices.uuid, devices.device_id, devices.uid, devices.bind_type, devices.secret, devices.name, devices.first_bind_time, devices.bind_time").
		Joins("JOIN devices ON devices.id = home_devices.device_id").
		Where("home_devices.home_id = ? AND home_devices.deleted_at IS NULL AND devices.deleted_at IS NULL", homeID).
		Order("home_devices.id ASC").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}

	return rows, nil
}

func (r *Repository) ListOwnedDevices(uid string) ([]devicemodel.Device, error) {
	var devices []devicemodel.Device
	err := r.db.Where("uid = ? AND deleted_at IS NULL", uid).Order("id DESC").Find(&devices).Error
	if err != nil {
		return nil, err
	}
	return devices, nil
}

func (r *Repository) CreateDevice(device *devicemodel.Device) error {
	return r.db.Create(device).Error
}

func (r *Repository) FindDeviceByUUID(uuid string) (*devicemodel.Device, error) {
	var device devicemodel.Device
	err := r.db.Where("uuid = ? AND deleted_at IS NULL", uuid).Take(&device).Error
	if err != nil {
		return nil, err
	}
	return &device, nil
}

func (r *Repository) UpdateDeviceByID(id uint, attrs map[string]any) error {
	result := r.db.Model(&devicemodel.Device{}).
		Where("id = ? AND deleted_at IS NULL", id).
		Updates(attrs)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (r *Repository) FindVisibleDeviceByUUID(userID uint, uid, uuid string) (*VisibleDevice, error) {
	var row VisibleDevice

	err := r.db.Table("devices").
		Select("COALESCE(home_devices.id, devices.id) as id, devices.uuid, devices.device_id, devices.uid, devices.bind_type, devices.secret, devices.name, devices.first_bind_time, devices.bind_time").
		Joins("LEFT JOIN home_devices ON home_devices.device_id = devices.id AND home_devices.deleted_at IS NULL").
		Joins("LEFT JOIN home_members ON home_members.home_id = home_devices.home_id AND home_members.user_id = ? AND home_members.deleted_at IS NULL", userID).
		Where("devices.uuid = ? AND devices.deleted_at IS NULL AND (devices.uid = ? OR home_members.id IS NOT NULL)", uuid, uid).
		Take(&row).Error
	if err != nil {
		return nil, err
	}

	return &row, nil
}

func (r *Repository) UpdateDeviceNameByID(id uint, name string) error {
	result := r.db.Model(&devicemodel.Device{}).
		Where("id = ? AND deleted_at IS NULL", id).
		Update("name", name)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func IsNotFound(err error) bool {
	return errors.Is(err, gorm.ErrRecordNotFound)
}
