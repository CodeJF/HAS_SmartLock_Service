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

type HomeShareInviteMessage struct {
	Invite struct {
		ID        uint
		MsgID     string
		Accept    int
		IsRead    int
		CreatedAt time.Time
	}
	Home struct {
		HomeID string
		Name   string
	}
	FromUser struct {
		UID      string
		Username string
	}
}

type HomeShareFeedbackMessageView struct {
	Message struct {
		ID        uint
		MsgID     string
		Status    int
		IsRead    int
		CreatedAt time.Time
	}
	Home struct {
		HomeID string
		Name   string
	}
	FromUser struct {
		UID      string
		Username string
	}
	ToUser struct {
		UID string
	}
}

type HomeShareRemoveMessageView struct {
	Message struct {
		ID        uint
		MsgID     string
		IsRead    int
		CreatedAt time.Time
	}
	Home struct {
		HomeID string
		Name   string
	}
	FromUser struct {
		UID      string
		Username string
	}
	ToUser struct {
		UID string
	}
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

func (r *Repository) CreateHomeShareInvite(invite *homemodel.HomeShareInvite) error {
	return r.db.Create(invite).Error
}

func (r *Repository) CreateHomeShareFeedbackMessage(message *homemodel.HomeShareFeedbackMessage) error {
	return r.db.Create(message).Error
}

func (r *Repository) CreateHomeShareRemoveMessage(message *homemodel.HomeShareRemoveMessage) error {
	return r.db.Create(message).Error
}

func (r *Repository) FindHomeByInternalID(id uint) (*homemodel.Home, error) {
	var home homemodel.Home
	err := r.db.Where("id = ? AND deleted_at IS NULL", id).Take(&home).Error
	if err != nil {
		return nil, err
	}
	return &home, nil
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

func (r *Repository) FindActiveHomeShareInviteByInternalHomeIDAndToUserID(homeID, toUserID uint) (*homemodel.HomeShareInvite, error) {
	var invite homemodel.HomeShareInvite
	err := r.db.
		Where("home_id = ? AND to_user_id = ? AND deleted_at IS NULL AND accept = 0", homeID, toUserID).
		Take(&invite).Error
	if err != nil {
		return nil, err
	}
	return &invite, nil
}

func (r *Repository) FindHomeShareInviteByMsgID(msgID string) (*homemodel.HomeShareInvite, error) {
	var invite homemodel.HomeShareInvite
	err := r.db.Where("msg_id = ? AND deleted_at IS NULL", msgID).Take(&invite).Error
	if err != nil {
		return nil, err
	}
	return &invite, nil
}

func (r *Repository) FindHomeShareFeedbackMessageByMsgID(msgID string) (*homemodel.HomeShareFeedbackMessage, error) {
	var message homemodel.HomeShareFeedbackMessage
	err := r.db.Where("msg_id = ? AND deleted_at IS NULL", msgID).Take(&message).Error
	if err != nil {
		return nil, err
	}
	return &message, nil
}

func (r *Repository) FindHomeShareRemoveMessageByMsgID(msgID string) (*homemodel.HomeShareRemoveMessage, error) {
	var message homemodel.HomeShareRemoveMessage
	err := r.db.Where("msg_id = ? AND deleted_at IS NULL", msgID).Take(&message).Error
	if err != nil {
		return nil, err
	}
	return &message, nil
}

func (r *Repository) ListHomeShareInviteMessagesByToUserID(toUserID uint, before time.Time, limit int) ([]HomeShareInviteMessage, error) {
	var rows []struct {
		InviteID        uint      `gorm:"column:invite_id"`
		InviteMsgID     string    `gorm:"column:invite_msg_id"`
		InviteAccept    int       `gorm:"column:invite_accept"`
		InviteIsRead    int       `gorm:"column:invite_is_read"`
		InviteCreatedAt time.Time `gorm:"column:invite_created_at"`
		HomeHomeID      string    `gorm:"column:home_home_id"`
		HomeName        string    `gorm:"column:home_name"`
		FromUserUID     string    `gorm:"column:from_user_uid"`
		FromUsername    string    `gorm:"column:from_username"`
	}

	query := r.db.Table("home_share_invites").
		Select("home_share_invites.id as invite_id, home_share_invites.msg_id as invite_msg_id, home_share_invites.accept as invite_accept, home_share_invites.is_read as invite_is_read, home_share_invites.created_at as invite_created_at, homes.home_id as home_home_id, homes.name as home_name, users.uid as from_user_uid, users.username as from_username").
		Joins("JOIN homes ON homes.id = home_share_invites.home_id").
		Joins("JOIN users ON users.id = home_share_invites.from_user_id").
		Where("home_share_invites.to_user_id = ? AND home_share_invites.deleted_at IS NULL AND homes.deleted_at IS NULL AND users.deleted_at IS NULL", toUserID)
	if !before.IsZero() {
		query = query.Where("home_share_invites.created_at < ?", before)
	}

	err := query.Order("home_share_invites.id DESC").Limit(limit).Scan(&rows).Error
	if err != nil {
		return nil, err
	}

	result := make([]HomeShareInviteMessage, 0, len(rows))
	for _, row := range rows {
		result = append(result, HomeShareInviteMessage{
			Invite: struct {
				ID        uint
				MsgID     string
				Accept    int
				IsRead    int
				CreatedAt time.Time
			}{
				ID:        row.InviteID,
				MsgID:     row.InviteMsgID,
				Accept:    row.InviteAccept,
				IsRead:    row.InviteIsRead,
				CreatedAt: row.InviteCreatedAt,
			},
			Home: struct {
				HomeID string
				Name   string
			}{
				HomeID: row.HomeHomeID,
				Name:   row.HomeName,
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

func (r *Repository) ListHomeShareFeedbackMessagesByToUserID(toUserID uint, before time.Time, limit int) ([]HomeShareFeedbackMessageView, error) {
	var rows []struct {
		MessageID        uint      `gorm:"column:message_id"`
		MessageMsgID     string    `gorm:"column:message_msg_id"`
		MessageStatus    int       `gorm:"column:message_status"`
		MessageIsRead    int       `gorm:"column:message_is_read"`
		MessageCreatedAt time.Time `gorm:"column:message_created_at"`
		HomeHomeID       string    `gorm:"column:home_home_id"`
		HomeName         string    `gorm:"column:home_name"`
		FromUserUID      string    `gorm:"column:from_user_uid"`
		FromUsername     string    `gorm:"column:from_username"`
		ToUserUID        string    `gorm:"column:to_user_uid"`
	}

	query := r.db.Table("home_share_feedback_messages").
		Select("home_share_feedback_messages.id as message_id, home_share_feedback_messages.msg_id as message_msg_id, home_share_feedback_messages.status as message_status, home_share_feedback_messages.is_read as message_is_read, home_share_feedback_messages.created_at as message_created_at, homes.home_id as home_home_id, homes.name as home_name, from_users.uid as from_user_uid, from_users.username as from_username, to_users.uid as to_user_uid").
		Joins("JOIN homes ON homes.id = home_share_feedback_messages.home_id").
		Joins("JOIN users as from_users ON from_users.id = home_share_feedback_messages.from_user_id").
		Joins("JOIN users as to_users ON to_users.id = home_share_feedback_messages.to_user_id").
		Where("home_share_feedback_messages.to_user_id = ? AND home_share_feedback_messages.deleted_at IS NULL AND homes.deleted_at IS NULL AND from_users.deleted_at IS NULL AND to_users.deleted_at IS NULL", toUserID)
	if !before.IsZero() {
		query = query.Where("home_share_feedback_messages.created_at < ?", before)
	}

	err := query.Order("home_share_feedback_messages.id DESC").Limit(limit).Scan(&rows).Error
	if err != nil {
		return nil, err
	}

	result := make([]HomeShareFeedbackMessageView, 0, len(rows))
	for _, row := range rows {
		result = append(result, HomeShareFeedbackMessageView{
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
			Home: struct {
				HomeID string
				Name   string
			}{
				HomeID: row.HomeHomeID,
				Name:   row.HomeName,
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

func (r *Repository) ListHomeShareRemoveMessagesByToUserID(toUserID uint, before time.Time, limit int) ([]HomeShareRemoveMessageView, error) {
	var rows []struct {
		MessageID        uint      `gorm:"column:message_id"`
		MessageMsgID     string    `gorm:"column:message_msg_id"`
		MessageIsRead    int       `gorm:"column:message_is_read"`
		MessageCreatedAt time.Time `gorm:"column:message_created_at"`
		HomeHomeID       string    `gorm:"column:home_home_id"`
		HomeName         string    `gorm:"column:home_name"`
		FromUserUID      string    `gorm:"column:from_user_uid"`
		FromUsername     string    `gorm:"column:from_username"`
		ToUserUID        string    `gorm:"column:to_user_uid"`
	}

	query := r.db.Table("home_share_remove_messages").
		Select("home_share_remove_messages.id as message_id, home_share_remove_messages.msg_id as message_msg_id, home_share_remove_messages.is_read as message_is_read, home_share_remove_messages.created_at as message_created_at, homes.home_id as home_home_id, homes.name as home_name, from_users.uid as from_user_uid, from_users.username as from_username, to_users.uid as to_user_uid").
		Joins("JOIN homes ON homes.id = home_share_remove_messages.home_id").
		Joins("JOIN users as from_users ON from_users.id = home_share_remove_messages.from_user_id").
		Joins("JOIN users as to_users ON to_users.id = home_share_remove_messages.to_user_id").
		Where("home_share_remove_messages.to_user_id = ? AND home_share_remove_messages.deleted_at IS NULL AND homes.deleted_at IS NULL AND from_users.deleted_at IS NULL AND to_users.deleted_at IS NULL", toUserID)
	if !before.IsZero() {
		query = query.Where("home_share_remove_messages.created_at < ?", before)
	}

	err := query.Order("home_share_remove_messages.id DESC").Limit(limit).Scan(&rows).Error
	if err != nil {
		return nil, err
	}

	result := make([]HomeShareRemoveMessageView, 0, len(rows))
	for _, row := range rows {
		result = append(result, HomeShareRemoveMessageView{
			Message: struct {
				ID        uint
				MsgID     string
				IsRead    int
				CreatedAt time.Time
			}{
				ID:        row.MessageID,
				MsgID:     row.MessageMsgID,
				IsRead:    row.MessageIsRead,
				CreatedAt: row.MessageCreatedAt,
			},
			Home: struct {
				HomeID string
				Name   string
			}{
				HomeID: row.HomeHomeID,
				Name:   row.HomeName,
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

func (r *Repository) UpdateHomeShareInviteByID(id uint, attrs map[string]any) error {
	return r.db.Model(&homemodel.HomeShareInvite{}).
		Where("id = ? AND deleted_at IS NULL", id).
		Updates(attrs).Error
}

func (r *Repository) UpdateHomeShareFeedbackMessageByID(id uint, attrs map[string]any) error {
	return r.db.Model(&homemodel.HomeShareFeedbackMessage{}).
		Where("id = ? AND deleted_at IS NULL", id).
		Updates(attrs).Error
}

func (r *Repository) UpdateHomeShareRemoveMessageByID(id uint, attrs map[string]any) error {
	return r.db.Model(&homemodel.HomeShareRemoveMessage{}).
		Where("id = ? AND deleted_at IS NULL", id).
		Updates(attrs).Error
}

func (r *Repository) CountUnreadHomeShareInvitesByToUserID(toUserID uint) (int64, error) {
	var count int64
	err := r.db.Model(&homemodel.HomeShareInvite{}).
		Where("to_user_id = ? AND deleted_at IS NULL AND is_read = 0", toUserID).
		Count(&count).Error
	if err != nil {
		return 0, err
	}
	return count, nil
}

func (r *Repository) CountUnreadHomeShareFeedbackMessagesByToUserID(toUserID uint) (int64, error) {
	var count int64
	err := r.db.Model(&homemodel.HomeShareFeedbackMessage{}).
		Where("to_user_id = ? AND deleted_at IS NULL AND is_read = 0", toUserID).
		Count(&count).Error
	if err != nil {
		return 0, err
	}
	return count, nil
}

func (r *Repository) CountUnreadHomeShareRemoveMessagesByToUserID(toUserID uint) (int64, error) {
	var count int64
	err := r.db.Model(&homemodel.HomeShareRemoveMessage{}).
		Where("to_user_id = ? AND deleted_at IS NULL AND is_read = 0", toUserID).
		Count(&count).Error
	if err != nil {
		return 0, err
	}
	return count, nil
}

func (r *Repository) MarkAllHomeShareInvitesReadByToUserID(toUserID uint) error {
	return r.db.Model(&homemodel.HomeShareInvite{}).
		Where("to_user_id = ? AND deleted_at IS NULL AND is_read = 0", toUserID).
		Update("is_read", 1).Error
}

func (r *Repository) MarkAllHomeShareFeedbackMessagesReadByToUserID(toUserID uint) error {
	return r.db.Model(&homemodel.HomeShareFeedbackMessage{}).
		Where("to_user_id = ? AND deleted_at IS NULL AND is_read = 0", toUserID).
		Update("is_read", 1).Error
}

func (r *Repository) MarkAllHomeShareRemoveMessagesReadByToUserID(toUserID uint) error {
	return r.db.Model(&homemodel.HomeShareRemoveMessage{}).
		Where("to_user_id = ? AND deleted_at IS NULL AND is_read = 0", toUserID).
		Update("is_read", 1).Error
}

func (r *Repository) SoftDeleteHomeByInternalID(id uint, now time.Time) error {
	return r.db.Model(&homemodel.Home{}).
		Where("id = ? AND deleted_at IS NULL", id).
		Update("deleted_at", now).Error
}

func (r *Repository) SoftDeleteHomeShareInviteByID(id uint, now time.Time) error {
	return r.db.Model(&homemodel.HomeShareInvite{}).
		Where("id = ? AND deleted_at IS NULL", id).
		Update("deleted_at", now).Error
}

func (r *Repository) SoftDeleteHomeShareFeedbackMessageByID(id uint, now time.Time) error {
	return r.db.Model(&homemodel.HomeShareFeedbackMessage{}).
		Where("id = ? AND deleted_at IS NULL", id).
		Update("deleted_at", now).Error
}

func (r *Repository) SoftDeleteHomeShareRemoveMessageByID(id uint, now time.Time) error {
	return r.db.Model(&homemodel.HomeShareRemoveMessage{}).
		Where("id = ? AND deleted_at IS NULL", id).
		Update("deleted_at", now).Error
}

func (r *Repository) SoftDeleteHomeMembersByInternalHomeID(homeID uint, now time.Time) error {
	return r.db.Model(&homemodel.HomeMember{}).
		Where("home_id = ? AND deleted_at IS NULL", homeID).
		Update("deleted_at", now).Error
}

func (r *Repository) SoftDeleteHomeMemberByInternalHomeIDAndUserID(homeID, userID uint, now time.Time) error {
	return r.db.Model(&homemodel.HomeMember{}).
		Where("home_id = ? AND user_id = ? AND deleted_at IS NULL", homeID, userID).
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
