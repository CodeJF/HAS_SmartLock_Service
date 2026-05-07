package eventstore

import (
	"context"
	"sort"
	"sync"
	"time"

	"go.mongodb.org/mongo-driver/v2/mongo"
)

type MemoryStore struct {
	mu     sync.RWMutex
	events map[string]Event
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		events: make(map[string]Event),
	}
}

func (s *MemoryStore) EnsureIndexes(context.Context) error {
	return nil
}

func (s *MemoryStore) Create(_ context.Context, event Event) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if event.ID == "" {
		event.ID = "evt_" + time.Now().Format("20060102150405.000000000")
	}
	if event.ExpireAt.IsZero() {
		event.ExpireAt = time.Now().Add(7 * 24 * time.Hour)
	}
	s.events[event.ID] = cloneEvent(event)
	return nil
}

func (s *MemoryStore) List(_ context.Context, filter Filter) ([]Event, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]Event, 0, len(s.events))
	for _, event := range s.events {
		if !matchesFilter(event, filter) {
			continue
		}
		view := cloneEvent(event)
		enrichView(&view, filter.UID)
		if isDeletedForUser(view, filter.UID) {
			continue
		}
		result = append(result, view)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Time > result[j].Time
	})
	if filter.Limit > 0 && int64(len(result)) > filter.Limit {
		result = result[:filter.Limit]
	}
	return result, nil
}

func (s *MemoryStore) CountUnread(ctx context.Context, uid, uuid string) (int64, error) {
	rows, err := s.List(ctx, Filter{UID: uid, UUID: uuid, Limit: 10000})
	if err != nil {
		return 0, err
	}
	var count int64
	for _, row := range rows {
		if len(row.BelongTo) > 0 && row.BelongTo[0].IsRead == 0 && row.BelongTo[0].Deleted == 0 {
			count++
		}
	}
	return count, nil
}

func (s *MemoryStore) MarkRead(_ context.Context, uid, eventID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	event, ok := s.events[eventID]
	if !ok {
		return mongo.ErrNoDocuments
	}
	for index := range event.BelongTo {
		if event.BelongTo[index].UID == uid {
			event.BelongTo[index].IsRead = 1
			s.events[eventID] = event
			return nil
		}
	}
	return mongo.ErrNoDocuments
}

func (s *MemoryStore) SoftDelete(_ context.Context, uid, eventID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	event, ok := s.events[eventID]
	if !ok {
		return mongo.ErrNoDocuments
	}
	for index := range event.BelongTo {
		if event.BelongTo[index].UID == uid {
			event.BelongTo[index].IsRead = 1
			event.BelongTo[index].Deleted = 1
			s.events[eventID] = event
			return nil
		}
	}
	return mongo.ErrNoDocuments
}

func (s *MemoryStore) CountDays(ctx context.Context, filter Filter) (map[string]int, error) {
	rows, err := s.List(ctx, Filter{
		UID:       filter.UID,
		UUID:      filter.UUID,
		HomeID:    filter.HomeID,
		Types:     filter.Types,
		StartTime: filter.StartTime,
		StartAt:   filter.StartAt,
		EndAt:     filter.EndAt,
		Limit:     10000,
	})
	if err != nil {
		return nil, err
	}
	result := make(map[string]int)
	for _, row := range rows {
		day := time.Unix(row.Time, 0).Format("2006-01-02")
		result[day]++
	}
	return result, nil
}

func (s *MemoryStore) GetVisible(_ context.Context, uid, eventID string) (*Event, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	event, ok := s.events[eventID]
	if !ok {
		return nil, mongo.ErrNoDocuments
	}
	for _, state := range event.BelongTo {
		if state.UID == uid && state.Deleted == 0 {
			view := cloneEvent(event)
			enrichView(&view, uid)
			return &view, nil
		}
	}
	return nil, mongo.ErrNoDocuments
}

func matchesFilter(event Event, filter Filter) bool {
	if filter.UUID != "" && event.UUID != filter.UUID {
		return false
	}
	if filter.HomeID != "" && event.HomeID != filter.HomeID {
		return false
	}
	if len(filter.Types) > 0 {
		matchType := false
		for _, eventType := range filter.Types {
			if event.Type == eventType {
				matchType = true
				break
			}
		}
		if !matchType {
			return false
		}
	}
	if filter.StartTime != nil && event.Time >= *filter.StartTime {
		return false
	}
	if filter.StartAt != nil && event.Time < *filter.StartAt {
		return false
	}
	if filter.EndAt != nil && event.Time >= *filter.EndAt {
		return false
	}
	for _, state := range event.BelongTo {
		if state.UID == filter.UID {
			return true
		}
	}
	return false
}

func cloneEvent(event Event) Event {
	out := event
	out.BelongTo = append([]BelongTo(nil), event.BelongTo...)
	if event.Payload != nil {
		out.Payload = make(map[string]any, len(event.Payload))
		for key, value := range event.Payload {
			out.Payload[key] = value
		}
	}
	return out
}

var _ Store = (*MemoryStore)(nil)
