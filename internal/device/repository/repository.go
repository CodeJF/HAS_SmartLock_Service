package repository

import (
	"errors"
	"time"

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

type DeviceShareInviteMessage struct {
	Invite struct {
		ID        uint
		MsgID     string
		Status    int
		IsRead    int
		CreatedAt time.Time
	}
	Device struct {
		UUID string
		Name string
	}
	FromUser struct {
		UID      string
		Username string
	}
}

type DeviceShareFeedbackMessageView struct {
	Message struct {
		ID        uint
		MsgID     string
		Status    int
		IsRead    int
		CreatedAt time.Time
	}
	Device struct {
		UUID string
		Name string
	}
	FromUser struct {
		UID      string
		Username string
	}
	ToUser struct {
		UID string
	}
}

type DeviceShareInviteRecordView struct {
	Invite struct {
		ID        uint
		Status    int
		CreatedAt time.Time
	}
	User struct {
		ID       uint
		UID      string
		Username string
	}
}

type DeviceShareMemberView struct {
	Member struct {
		ID        uint
		Role      int
		CreatedAt time.Time
	}
	User struct {
		ID       uint
		UID      string
		Username string
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

func (r *Repository) ListVisibleDevices(userID uint, uid string) ([]VisibleDevice, error) {
	var rows []VisibleDevice

	err := r.db.Table("devices").
		Select("devices.id as id, devices.uuid, devices.device_id, devices.uid, CASE WHEN devices.uid = ? THEN devices.bind_type WHEN device_share_members.id IS NOT NULL THEN 2 ELSE devices.bind_type END as bind_type, devices.secret, devices.name, devices.first_bind_time, devices.bind_time", uid).
		Joins("LEFT JOIN home_devices ON home_devices.device_id = devices.id AND home_devices.deleted_at IS NULL").
		Joins("LEFT JOIN home_members ON home_members.home_id = home_devices.home_id AND home_members.user_id = ? AND home_members.deleted_at IS NULL", userID).
		Joins("LEFT JOIN device_share_members ON device_share_members.device_id = devices.id AND device_share_members.user_id = ? AND device_share_members.deleted_at IS NULL", userID).
		Where("devices.deleted_at IS NULL AND (devices.uid = ? OR home_members.id IS NOT NULL OR device_share_members.id IS NOT NULL)", uid).
		Group("devices.id, devices.uuid, devices.device_id, devices.uid, devices.bind_type, device_share_members.id, devices.secret, devices.name, devices.first_bind_time, devices.bind_time").
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

func (r *Repository) FindDeviceByInternalID(id uint) (*devicemodel.Device, error) {
	var device devicemodel.Device
	err := r.db.Where("id = ? AND deleted_at IS NULL", id).Take(&device).Error
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
		Select("devices.id as id, devices.uuid, devices.device_id, devices.uid, CASE WHEN devices.uid = ? THEN devices.bind_type WHEN device_share_members.id IS NOT NULL THEN 2 ELSE devices.bind_type END as bind_type, devices.secret, devices.name, devices.first_bind_time, devices.bind_time", uid).
		Joins("LEFT JOIN home_devices ON home_devices.device_id = devices.id AND home_devices.deleted_at IS NULL").
		Joins("LEFT JOIN home_members ON home_members.home_id = home_devices.home_id AND home_members.user_id = ? AND home_members.deleted_at IS NULL", userID).
		Joins("LEFT JOIN device_share_members ON device_share_members.device_id = devices.id AND device_share_members.user_id = ? AND device_share_members.deleted_at IS NULL", userID).
		Where("devices.uuid = ? AND devices.deleted_at IS NULL AND (devices.uid = ? OR home_members.id IS NOT NULL OR device_share_members.id IS NOT NULL)", uuid, uid).
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

func (r *Repository) CreateDeviceShareInvite(invite *devicemodel.DeviceShareInvite) error {
	return r.db.Create(invite).Error
}

func (r *Repository) CreateDeviceShareMember(member *devicemodel.DeviceShareMember) error {
	return r.db.Create(member).Error
}

func (r *Repository) CreateDeviceShareFeedbackMessage(message *devicemodel.DeviceShareFeedbackMessage) error {
	return r.db.Create(message).Error
}

func (r *Repository) FindActiveDeviceShareInviteByInternalDeviceIDAndToUserID(deviceID, toUserID uint) (*devicemodel.DeviceShareInvite, error) {
	var invite devicemodel.DeviceShareInvite
	err := r.db.
		Where("device_id = ? AND to_user_id = ? AND deleted_at IS NULL AND status = 0", deviceID, toUserID).
		Take(&invite).Error
	if err != nil {
		return nil, err
	}
	return &invite, nil
}

func (r *Repository) FindActiveDeviceShareMemberByInternalDeviceIDAndUserID(deviceID, userID uint) (*devicemodel.DeviceShareMember, error) {
	var member devicemodel.DeviceShareMember
	err := r.db.
		Where("device_id = ? AND user_id = ? AND deleted_at IS NULL", deviceID, userID).
		Take(&member).Error
	if err != nil {
		return nil, err
	}
	return &member, nil
}

func (r *Repository) FindDeviceShareInviteByMsgID(msgID string) (*devicemodel.DeviceShareInvite, error) {
	var invite devicemodel.DeviceShareInvite
	err := r.db.Where("msg_id = ? AND deleted_at IS NULL", msgID).Take(&invite).Error
	if err != nil {
		return nil, err
	}
	return &invite, nil
}

func (r *Repository) FindDeviceShareFeedbackMessageByMsgID(msgID string) (*devicemodel.DeviceShareFeedbackMessage, error) {
	var message devicemodel.DeviceShareFeedbackMessage
	err := r.db.Where("msg_id = ? AND deleted_at IS NULL", msgID).Take(&message).Error
	if err != nil {
		return nil, err
	}
	return &message, nil
}

func (r *Repository) ListDeviceShareInviteMessagesByToUserID(toUserID uint, before time.Time, limit int) ([]DeviceShareInviteMessage, error) {
	var rows []struct {
		InviteID        uint      `gorm:"column:invite_id"`
		InviteMsgID     string    `gorm:"column:invite_msg_id"`
		InviteStatus    int       `gorm:"column:invite_status"`
		InviteIsRead    int       `gorm:"column:invite_is_read"`
		InviteCreatedAt time.Time `gorm:"column:invite_created_at"`
		DeviceUUID      string    `gorm:"column:device_uuid"`
		DeviceName      string    `gorm:"column:device_name"`
		FromUserUID     string    `gorm:"column:from_user_uid"`
		FromUsername    string    `gorm:"column:from_username"`
	}

	query := r.db.Table("device_share_invites").
		Select("device_share_invites.id as invite_id, device_share_invites.msg_id as invite_msg_id, device_share_invites.status as invite_status, device_share_invites.is_read as invite_is_read, device_share_invites.created_at as invite_created_at, devices.uuid as device_uuid, devices.name as device_name, users.uid as from_user_uid, users.username as from_username").
		Joins("JOIN devices ON devices.id = device_share_invites.device_id").
		Joins("JOIN users ON users.id = device_share_invites.from_user_id").
		Where("device_share_invites.to_user_id = ? AND device_share_invites.deleted_at IS NULL AND devices.deleted_at IS NULL AND users.deleted_at IS NULL", toUserID)
	if !before.IsZero() {
		query = query.Where("device_share_invites.created_at < ?", before)
	}

	err := query.Order("device_share_invites.id DESC").Limit(limit).Scan(&rows).Error
	if err != nil {
		return nil, err
	}

	result := make([]DeviceShareInviteMessage, 0, len(rows))
	for _, row := range rows {
		result = append(result, DeviceShareInviteMessage{
			Invite: struct {
				ID        uint
				MsgID     string
				Status    int
				IsRead    int
				CreatedAt time.Time
			}{
				ID:        row.InviteID,
				MsgID:     row.InviteMsgID,
				Status:    row.InviteStatus,
				IsRead:    row.InviteIsRead,
				CreatedAt: row.InviteCreatedAt,
			},
			Device: struct {
				UUID string
				Name string
			}{
				UUID: row.DeviceUUID,
				Name: row.DeviceName,
			},
			FromUser: struct {
				UID      string
				Username string
			}{
				UID:      row.FromUserUID,
				Username: row.FromUsername,
			},
		})
	}
	return result, nil
}

func (r *Repository) ListDeviceShareFeedbackMessagesByToUserID(toUserID uint, before time.Time, limit int) ([]DeviceShareFeedbackMessageView, error) {
	var rows []struct {
		MessageID        uint      `gorm:"column:message_id"`
		MessageMsgID     string    `gorm:"column:message_msg_id"`
		MessageStatus    int       `gorm:"column:message_status"`
		MessageIsRead    int       `gorm:"column:message_is_read"`
		MessageCreatedAt time.Time `gorm:"column:message_created_at"`
		DeviceUUID       string    `gorm:"column:device_uuid"`
		DeviceName       string    `gorm:"column:device_name"`
		FromUserUID      string    `gorm:"column:from_user_uid"`
		FromUsername     string    `gorm:"column:from_username"`
		ToUserUID        string    `gorm:"column:to_user_uid"`
	}

	query := r.db.Table("device_share_feedback_messages").
		Select("device_share_feedback_messages.id as message_id, device_share_feedback_messages.msg_id as message_msg_id, device_share_feedback_messages.status as message_status, device_share_feedback_messages.is_read as message_is_read, device_share_feedback_messages.created_at as message_created_at, devices.uuid as device_uuid, devices.name as device_name, from_users.uid as from_user_uid, from_users.username as from_username, to_users.uid as to_user_uid").
		Joins("JOIN devices ON devices.id = device_share_feedback_messages.device_id").
		Joins("JOIN users as from_users ON from_users.id = device_share_feedback_messages.from_user_id").
		Joins("JOIN users as to_users ON to_users.id = device_share_feedback_messages.to_user_id").
		Where("device_share_feedback_messages.to_user_id = ? AND device_share_feedback_messages.deleted_at IS NULL AND devices.deleted_at IS NULL AND from_users.deleted_at IS NULL AND to_users.deleted_at IS NULL", toUserID)
	if !before.IsZero() {
		query = query.Where("device_share_feedback_messages.created_at < ?", before)
	}

	err := query.Order("device_share_feedback_messages.id DESC").Limit(limit).Scan(&rows).Error
	if err != nil {
		return nil, err
	}

	result := make([]DeviceShareFeedbackMessageView, 0, len(rows))
	for _, row := range rows {
		result = append(result, DeviceShareFeedbackMessageView{
			Message: struct {
				ID        uint
				MsgID     string
				Status    int
				IsRead    int
				CreatedAt time.Time
			}{
				ID:        row.MessageID,
				MsgID:     row.MessageMsgID,
				Status:    row.MessageStatus,
				IsRead:    row.MessageIsRead,
				CreatedAt: row.MessageCreatedAt,
			},
			Device: struct {
				UUID string
				Name string
			}{
				UUID: row.DeviceUUID,
				Name: row.DeviceName,
			},
			FromUser: struct {
				UID      string
				Username string
			}{
				UID:      row.FromUserUID,
				Username: row.FromUsername,
			},
			ToUser: struct {
				UID string
			}{
				UID: row.ToUserUID,
			},
		})
	}
	return result, nil
}

func (r *Repository) ListDeviceShareInvitesByInternalDeviceID(deviceID uint) ([]DeviceShareInviteRecordView, error) {
	var rows []struct {
		InviteID        uint      `gorm:"column:invite_id"`
		InviteStatus    int       `gorm:"column:invite_status"`
		InviteCreatedAt time.Time `gorm:"column:invite_created_at"`
		UserID          uint      `gorm:"column:user_id"`
		UserUID         string    `gorm:"column:user_uid"`
		Username        string    `gorm:"column:username"`
	}

	err := r.db.Table("device_share_invites").
		Select("device_share_invites.id as invite_id, device_share_invites.status as invite_status, device_share_invites.created_at as invite_created_at, users.id as user_id, users.uid as user_uid, users.username as username").
		Joins("JOIN users ON users.id = device_share_invites.to_user_id").
		Where("device_share_invites.device_id = ? AND device_share_invites.deleted_at IS NULL AND users.deleted_at IS NULL", deviceID).
		Order("device_share_invites.created_at DESC, device_share_invites.id DESC").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}

	result := make([]DeviceShareInviteRecordView, 0, len(rows))
	for _, row := range rows {
		result = append(result, DeviceShareInviteRecordView{
			Invite: struct {
				ID        uint
				Status    int
				CreatedAt time.Time
			}{
				ID:        row.InviteID,
				Status:    row.InviteStatus,
				CreatedAt: row.InviteCreatedAt,
			},
			User: struct {
				ID       uint
				UID      string
				Username string
			}{
				ID:       row.UserID,
				UID:      row.UserUID,
				Username: row.Username,
			},
		})
	}
	return result, nil
}

func (r *Repository) ListActiveDeviceShareMembersByInternalDeviceID(deviceID uint) ([]DeviceShareMemberView, error) {
	var rows []struct {
		MemberID        uint      `gorm:"column:member_id"`
		MemberRole      int       `gorm:"column:member_role"`
		MemberCreatedAt time.Time `gorm:"column:member_created_at"`
		UserID          uint      `gorm:"column:user_id"`
		UserUID         string    `gorm:"column:user_uid"`
		Username        string    `gorm:"column:username"`
	}

	err := r.db.Table("device_share_members").
		Select("device_share_members.id as member_id, device_share_members.role as member_role, device_share_members.created_at as member_created_at, users.id as user_id, users.uid as user_uid, users.username as username").
		Joins("JOIN users ON users.id = device_share_members.user_id").
		Where("device_share_members.device_id = ? AND device_share_members.deleted_at IS NULL AND users.deleted_at IS NULL", deviceID).
		Order("device_share_members.created_at DESC, device_share_members.id DESC").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}

	result := make([]DeviceShareMemberView, 0, len(rows))
	for _, row := range rows {
		result = append(result, DeviceShareMemberView{
			Member: struct {
				ID        uint
				Role      int
				CreatedAt time.Time
			}{
				ID:        row.MemberID,
				Role:      row.MemberRole,
				CreatedAt: row.MemberCreatedAt,
			},
			User: struct {
				ID       uint
				UID      string
				Username string
			}{
				ID:       row.UserID,
				UID:      row.UserUID,
				Username: row.Username,
			},
		})
	}
	return result, nil
}

func (r *Repository) UpdateDeviceShareInviteByID(id uint, attrs map[string]any) error {
	return r.db.Model(&devicemodel.DeviceShareInvite{}).
		Where("id = ? AND deleted_at IS NULL", id).
		Updates(attrs).Error
}

func (r *Repository) UpdateDeviceShareFeedbackMessageByID(id uint, attrs map[string]any) error {
	return r.db.Model(&devicemodel.DeviceShareFeedbackMessage{}).
		Where("id = ? AND deleted_at IS NULL", id).
		Updates(attrs).Error
}

func (r *Repository) CountUnreadDeviceShareInvitesByToUserID(toUserID uint) (int64, error) {
	var count int64
	err := r.db.Model(&devicemodel.DeviceShareInvite{}).
		Where("to_user_id = ? AND deleted_at IS NULL AND is_read = 0", toUserID).
		Count(&count).Error
	if err != nil {
		return 0, err
	}
	return count, nil
}

func (r *Repository) CountUnreadDeviceShareFeedbackMessagesByToUserID(toUserID uint) (int64, error) {
	var count int64
	err := r.db.Model(&devicemodel.DeviceShareFeedbackMessage{}).
		Where("to_user_id = ? AND deleted_at IS NULL AND is_read = 0", toUserID).
		Count(&count).Error
	if err != nil {
		return 0, err
	}
	return count, nil
}

func (r *Repository) MarkAllDeviceShareInvitesReadByToUserID(toUserID uint) error {
	return r.db.Model(&devicemodel.DeviceShareInvite{}).
		Where("to_user_id = ? AND deleted_at IS NULL AND is_read = 0", toUserID).
		Update("is_read", 1).Error
}

func (r *Repository) MarkAllDeviceShareFeedbackMessagesReadByToUserID(toUserID uint) error {
	return r.db.Model(&devicemodel.DeviceShareFeedbackMessage{}).
		Where("to_user_id = ? AND deleted_at IS NULL AND is_read = 0", toUserID).
		Update("is_read", 1).Error
}

func (r *Repository) SoftDeleteDeviceShareInviteByID(id uint, now time.Time) error {
	return r.db.Model(&devicemodel.DeviceShareInvite{}).
		Where("id = ? AND deleted_at IS NULL", id).
		Update("deleted_at", now).Error
}

func (r *Repository) SoftDeleteDeviceShareFeedbackMessageByID(id uint, now time.Time) error {
	return r.db.Model(&devicemodel.DeviceShareFeedbackMessage{}).
		Where("id = ? AND deleted_at IS NULL", id).
		Update("deleted_at", now).Error
}

func (r *Repository) SoftDeleteDeviceShareMemberByID(id uint, now time.Time) error {
	return r.db.Model(&devicemodel.DeviceShareMember{}).
		Where("id = ? AND deleted_at IS NULL", id).
		Update("deleted_at", now).Error
}

func (r *Repository) SoftDeleteActiveDeviceShareInvitesByInternalDeviceIDAndToUserID(deviceID, toUserID uint, now time.Time) error {
	return r.db.Model(&devicemodel.DeviceShareInvite{}).
		Where("device_id = ? AND to_user_id = ? AND deleted_at IS NULL AND status = 0", deviceID, toUserID).
		Update("deleted_at", now).Error
}

func IsNotFound(err error) bool {
	return errors.Is(err, gorm.ErrRecordNotFound)
}
