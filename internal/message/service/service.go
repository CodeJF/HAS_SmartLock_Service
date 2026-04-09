package service

import (
	"errors"
	"sort"
	"strings"
	"time"

	homerepo "has-smartlock-service/internal/home/repository"
	usermodel "has-smartlock-service/internal/user/model"
	userrepo "has-smartlock-service/internal/user/repository"
)

const pageSize = 20

var ErrUserNotFound = errors.New("user not found")

type Service struct {
	homeRepo *homerepo.Repository
	userRepo *userrepo.Repository
}

type MessagePayload struct {
	HomeID   string `json:"home_id,omitempty"`
	HomeName string `json:"home_name,omitempty"`
	Status   int    `json:"status"`
	UID      string `json:"uid,omitempty"`
	Username string `json:"username,omitempty"`
}

type MessageItem struct {
	ID      string         `json:"id"`
	UID     string         `json:"uid,omitempty"`
	Type    int            `json:"type"`
	Time    int64          `json:"time"`
	IsRead  int            `json:"is_read"`
	Payload MessagePayload `json:"payload"`
}

type ListResult struct {
	Has  bool          `json:"has"`
	List []MessageItem `json:"list"`
}

type UnreadNumResult struct {
	Number int64 `json:"number"`
}

func New(homeRepo *homerepo.Repository, userRepo *userrepo.Repository) *Service {
	return &Service{homeRepo: homeRepo, userRepo: userRepo}
}

func (s *Service) List(uid, startID string) (ListResult, error) {
	if strings.TrimSpace(uid) == "" {
		return ListResult{}, ErrUserNotFound
	}

	user, err := s.userRepo.FindUserByUID(strings.TrimSpace(uid))
	if err != nil {
		if userrepo.IsNotFound(err) {
			return ListResult{}, ErrUserNotFound
		}
		return ListResult{}, err
	}

	var before time.Time
	if strings.TrimSpace(startID) != "" {
		invite, err := s.homeRepo.FindHomeShareInviteByMsgID(strings.TrimSpace(startID))
		if err == nil && invite.ToUserID == user.ID {
			before = invite.CreatedAt
		} else if err != nil && !homerepo.IsNotFound(err) {
			return ListResult{}, err
		}
		if before.IsZero() {
			feedback, err := s.homeRepo.FindHomeShareFeedbackMessageByMsgID(strings.TrimSpace(startID))
			if err == nil && feedback.ToUserID == user.ID {
				before = feedback.CreatedAt
			} else if err != nil && !homerepo.IsNotFound(err) {
				return ListResult{}, err
			}
		}
		if before.IsZero() {
			removeMsg, err := s.homeRepo.FindHomeShareRemoveMessageByMsgID(strings.TrimSpace(startID))
			if err == nil && removeMsg.ToUserID == user.ID {
				before = removeMsg.CreatedAt
			} else if err != nil && !homerepo.IsNotFound(err) {
				return ListResult{}, err
			}
		}
	}

	inviteRows, err := s.homeRepo.ListHomeShareInviteMessagesByToUserID(user.ID, before, pageSize+1)
	if err != nil {
		return ListResult{}, err
	}
	feedbackRows, err := s.homeRepo.ListHomeShareFeedbackMessagesByToUserID(user.ID, before, pageSize+1)
	if err != nil {
		return ListResult{}, err
	}
	removeRows, err := s.homeRepo.ListHomeShareRemoveMessagesByToUserID(user.ID, before, pageSize+1)
	if err != nil {
		return ListResult{}, err
	}

	items := make([]MessageItem, 0, len(inviteRows)+len(feedbackRows)+len(removeRows))
	for _, row := range inviteRows {
		if !before.IsZero() && !row.Invite.CreatedAt.Before(before) {
			continue
		}
		items = append(items, MessageItem{
			ID:     row.Invite.MsgID,
			UID:    user.UID,
			Type:   1,
			Time:   row.Invite.CreatedAt.Unix(),
			IsRead: row.Invite.IsRead,
			Payload: MessagePayload{
				HomeID:   row.Home.HomeID,
				HomeName: row.Home.Name,
				Status:   row.Invite.Accept,
				UID:      row.FromUser.UID,
				Username: row.FromUser.Username,
			},
		})
	}
	for _, row := range feedbackRows {
		items = append(items, MessageItem{
			ID:     row.Message.MsgID,
			UID:    row.ToUser.UID,
			Type:   2,
			Time:   row.Message.CreatedAt.Unix(),
			IsRead: row.Message.IsRead,
			Payload: MessagePayload{
				HomeID:   row.Home.HomeID,
				HomeName: row.Home.Name,
				Status:   row.Message.Status,
				UID:      row.FromUser.UID,
				Username: row.FromUser.Username,
			},
		})
	}
	for _, row := range removeRows {
		items = append(items, MessageItem{
			ID:     row.Message.MsgID,
			UID:    row.ToUser.UID,
			Type:   3,
			Time:   row.Message.CreatedAt.Unix(),
			IsRead: row.Message.IsRead,
			Payload: MessagePayload{
				HomeID:   row.Home.HomeID,
				HomeName: row.Home.Name,
				UID:      row.FromUser.UID,
				Username: row.FromUser.Username,
			},
		})
	}

	sort.Slice(items, func(i, j int) bool {
		if items[i].Time == items[j].Time {
			return items[i].ID > items[j].ID
		}
		return items[i].Time > items[j].Time
	})

	result := ListResult{
		Has:  len(items) > pageSize,
		List: make([]MessageItem, 0, min(len(items), pageSize)),
	}
	if len(items) > pageSize {
		items = items[:pageSize]
	}
	result.List = append(result.List, items...)
	return result, nil
}

func (s *Service) UnreadNum(uid string) (UnreadNumResult, error) {
	user, err := s.findUser(uid)
	if err != nil {
		return UnreadNumResult{}, err
	}

	inviteCount, err := s.homeRepo.CountUnreadHomeShareInvitesByToUserID(user.ID)
	if err != nil {
		return UnreadNumResult{}, err
	}
	feedbackCount, err := s.homeRepo.CountUnreadHomeShareFeedbackMessagesByToUserID(user.ID)
	if err != nil {
		return UnreadNumResult{}, err
	}
	removeCount, err := s.homeRepo.CountUnreadHomeShareRemoveMessagesByToUserID(user.ID)
	if err != nil {
		return UnreadNumResult{}, err
	}

	return UnreadNumResult{Number: inviteCount + feedbackCount + removeCount}, nil
}

func (s *Service) Read(uid, messageID string) error {
	user, err := s.findUser(uid)
	if err != nil {
		return err
	}

	if strings.TrimSpace(messageID) == "" {
		if err := s.homeRepo.MarkAllHomeShareInvitesReadByToUserID(user.ID); err != nil {
			return err
		}
		if err := s.homeRepo.MarkAllHomeShareFeedbackMessagesReadByToUserID(user.ID); err != nil {
			return err
		}
		return s.homeRepo.MarkAllHomeShareRemoveMessagesReadByToUserID(user.ID)
	}

	invite, err := s.homeRepo.FindHomeShareInviteByMsgID(strings.TrimSpace(messageID))
	if err == nil {
		if invite.ToUserID != user.ID {
			return ErrMessageForbidden
		}
		return s.homeRepo.UpdateHomeShareInviteByID(invite.ID, map[string]any{"is_read": 1})
	}
	if err != nil && !homerepo.IsNotFound(err) {
		return err
	}

	feedback, err := s.homeRepo.FindHomeShareFeedbackMessageByMsgID(strings.TrimSpace(messageID))
	if err == nil {
		if feedback.ToUserID != user.ID {
			return ErrMessageForbidden
		}
		return s.homeRepo.UpdateHomeShareFeedbackMessageByID(feedback.ID, map[string]any{"is_read": 1})
	}
	if err != nil && !homerepo.IsNotFound(err) {
		return err
	}

	removeMsg, err := s.homeRepo.FindHomeShareRemoveMessageByMsgID(strings.TrimSpace(messageID))
	if err == nil {
		if removeMsg.ToUserID != user.ID {
			return ErrMessageForbidden
		}
		return s.homeRepo.UpdateHomeShareRemoveMessageByID(removeMsg.ID, map[string]any{"is_read": 1})
	}
	if err != nil && !homerepo.IsNotFound(err) {
		return err
	}

	return ErrMessageNotFound
}

var (
	ErrMessageNotFound  = errors.New("message not found")
	ErrMessageForbidden = errors.New("message forbidden")
)

func (s *Service) findUser(uid string) (*usermodel.User, error) {
	if strings.TrimSpace(uid) == "" {
		return nil, ErrUserNotFound
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

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
