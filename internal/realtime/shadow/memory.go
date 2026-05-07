package shadow

import (
	"context"
	"sort"
	"sync"
	"time"
)

type MemoryStore struct {
	mu   sync.RWMutex
	docs map[string]*Document
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		docs: make(map[string]*Document),
	}
}

func (s *MemoryStore) EnsureIndexes(context.Context) error {
	return nil
}

func (s *MemoryStore) Get(_ context.Context, uuid string) (*Document, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if doc, ok := s.docs[uuid]; ok {
		return cloneDocument(doc), nil
	}
	return &Document{
		UUID: uuid,
		State: State{
			Desired:  map[string]any{},
			Reported: map[string]any{},
		},
		Metadata: Metadata{
			Desired:  map[string]TimestampedValue{},
			Reported: map[string]TimestampedValue{},
		},
	}, nil
}

func (s *MemoryStore) EnsureDevice(_ context.Context, uuid, uid string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.docs[uuid]; ok {
		return nil
	}
	s.docs[uuid] = &Document{
		UUID: uuid,
		UID:  uid,
		State: State{
			Desired:  map[string]any{},
			Reported: map[string]any{},
		},
		Metadata: Metadata{
			Desired:  map[string]TimestampedValue{},
			Reported: map[string]TimestampedValue{},
		},
		Timestamp: time.Now().Unix(),
		Version:   1,
	}
	return nil
}

func (s *MemoryStore) Delete(_ context.Context, uuid string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.docs, uuid)
	return nil
}

func (s *MemoryStore) UpdateReported(ctx context.Context, uuid, uid string, timestamp int64, values map[string]map[string]any) (*Document, error) {
	doc, err := s.Get(ctx, uuid)
	if err != nil {
		return nil, err
	}
	normalizeDocument(doc)
	changed := false
	for key, payload := range values {
		valueTimestamp := timestamp
		switch raw := payload["timestamp"].(type) {
		case int64:
			valueTimestamp = raw
		case int32:
			valueTimestamp = int64(raw)
		case int:
			valueTimestamp = int64(raw)
		case float64:
			valueTimestamp = int64(raw)
		}
		if previous, ok := doc.Metadata.Reported[key]; ok && previous.Timestamp >= valueTimestamp {
			continue
		}
		doc.State.Reported[key] = payload["value"]
		doc.Metadata.Reported[key] = TimestampedValue{Timestamp: valueTimestamp}
		changed = true
	}
	if !changed {
		return doc, nil
	}
	doc.Timestamp = time.Now().Unix()
	doc.Version++
	doc.UID = uid
	s.mu.Lock()
	s.docs[uuid] = cloneDocument(doc)
	s.mu.Unlock()
	return cloneDocument(doc), nil
}

func (s *MemoryStore) UpdateDesired(ctx context.Context, uuid, uid string, timestamp int64, values map[string]any) (*Document, error) {
	doc, err := s.Get(ctx, uuid)
	if err != nil {
		return nil, err
	}
	normalizeDocument(doc)
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		doc.State.Desired[key] = values[key]
		doc.Metadata.Desired[key] = TimestampedValue{Timestamp: timestamp}
	}
	doc.Timestamp = time.Now().Unix()
	doc.Version++
	doc.UID = uid
	s.mu.Lock()
	s.docs[uuid] = cloneDocument(doc)
	s.mu.Unlock()
	return cloneDocument(doc), nil
}

func cloneDocument(doc *Document) *Document {
	if doc == nil {
		return nil
	}
	out := *doc
	out.State.Desired = cloneMap(doc.State.Desired)
	out.State.Reported = cloneMap(doc.State.Reported)
	out.Metadata.Desired = cloneMetadata(doc.Metadata.Desired)
	out.Metadata.Reported = cloneMetadata(doc.Metadata.Reported)
	return &out
}

func cloneMap(source map[string]any) map[string]any {
	if source == nil {
		return map[string]any{}
	}
	out := make(map[string]any, len(source))
	for key, value := range source {
		switch typed := value.(type) {
		case map[string]any:
			out[key] = cloneMap(typed)
		default:
			out[key] = typed
		}
	}
	return out
}

func cloneMetadata(source map[string]TimestampedValue) map[string]TimestampedValue {
	if source == nil {
		return map[string]TimestampedValue{}
	}
	out := make(map[string]TimestampedValue, len(source))
	for key, value := range source {
		out[key] = value
	}
	return out
}
