package shadow

import (
	"context"
	"errors"
	"sort"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

type TimestampedValue struct {
	Timestamp int64 `bson:"timestamp" json:"timestamp"`
}

type State struct {
	Desired  map[string]any `bson:"desired" json:"desired"`
	Reported map[string]any `bson:"reported" json:"reported"`
}

type Metadata struct {
	Desired  map[string]TimestampedValue `bson:"desired" json:"desired"`
	Reported map[string]TimestampedValue `bson:"reported" json:"reported"`
}

type Document struct {
	UUID      string   `bson:"uuid"`
	UID       string   `bson:"uid"`
	State     State    `bson:"state"`
	Metadata  Metadata `bson:"metadata"`
	Timestamp int64    `bson:"timestamp"`
	Version   int64    `bson:"version"`
}

type Store interface {
	EnsureIndexes(ctx context.Context) error
	Get(ctx context.Context, uuid string) (*Document, error)
	EnsureDevice(ctx context.Context, uuid, uid string) error
	Delete(ctx context.Context, uuid string) error
	UpdateReported(ctx context.Context, uuid, uid string, timestamp int64, values map[string]map[string]any) (*Document, error)
	UpdateDesired(ctx context.Context, uuid, uid string, timestamp int64, values map[string]any) (*Document, error)
}

type MongoStore struct {
	coll *mongo.Collection
}

func NewMongoStore(db *mongo.Database) *MongoStore {
	if db == nil {
		return nil
	}
	return &MongoStore{coll: db.Collection("device_shadow")}
}

func (s *MongoStore) Enabled() bool {
	return s != nil && s.coll != nil
}

func (s *MongoStore) EnsureIndexes(ctx context.Context) error {
	if !s.Enabled() {
		return nil
	}
	_, err := s.coll.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{
			Keys:    bson.D{{Key: "uuid", Value: 1}},
			Options: options.Index().SetUnique(true),
		},
	})
	return err
}

func (s *MongoStore) Get(ctx context.Context, uuid string) (*Document, error) {
	if !s.Enabled() {
		return &Document{UUID: uuid, State: State{Desired: map[string]any{}, Reported: map[string]any{}}, Metadata: Metadata{Desired: map[string]TimestampedValue{}, Reported: map[string]TimestampedValue{}}}, nil
	}
	var doc Document
	err := s.coll.FindOne(ctx, bson.M{"uuid": uuid}).Decode(&doc)
	if errors.Is(err, mongo.ErrNoDocuments) {
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
	if err != nil {
		return nil, err
	}
	normalizeDocument(&doc)
	return &doc, nil
}

func (s *MongoStore) EnsureDevice(ctx context.Context, uuid, uid string) error {
	if !s.Enabled() {
		return nil
	}
	now := time.Now().Unix()
	_, err := s.coll.UpdateOne(ctx, bson.M{"uuid": uuid}, bson.M{
		"$setOnInsert": bson.M{
			"uuid": uuid,
			"uid":  uid,
			"state": bson.M{
				"desired":  bson.M{},
				"reported": bson.M{},
			},
			"metadata": bson.M{
				"desired":  bson.M{},
				"reported": bson.M{},
			},
			"timestamp": now,
			"version":   1,
		},
	}, options.UpdateOne().SetUpsert(true))
	return err
}

func (s *MongoStore) Delete(ctx context.Context, uuid string) error {
	if !s.Enabled() {
		return nil
	}
	_, err := s.coll.DeleteOne(ctx, bson.M{"uuid": uuid})
	return err
}

func (s *MongoStore) UpdateReported(ctx context.Context, uuid, uid string, timestamp int64, values map[string]map[string]any) (*Document, error) {
	doc, err := s.Get(ctx, uuid)
	if err != nil {
		return nil, err
	}
	normalizeDocument(doc)
	changed := false
	for key, payload := range values {
		valueTimestamp := timestamp
		if raw, ok := payload["timestamp"]; ok {
			switch v := raw.(type) {
			case int64:
				valueTimestamp = v
			case int32:
				valueTimestamp = int64(v)
			case int:
				valueTimestamp = int64(v)
			case float64:
				valueTimestamp = int64(v)
			}
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
	return doc, s.replace(ctx, doc)
}

func (s *MongoStore) UpdateDesired(ctx context.Context, uuid, uid string, timestamp int64, values map[string]any) (*Document, error) {
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
	return doc, s.replace(ctx, doc)
}

func (s *MongoStore) replace(ctx context.Context, doc *Document) error {
	if !s.Enabled() {
		return nil
	}
	_, err := s.coll.ReplaceOne(ctx, bson.M{"uuid": doc.UUID}, doc, options.Replace().SetUpsert(true))
	return err
}

func normalizeDocument(doc *Document) {
	if doc.State.Desired == nil {
		doc.State.Desired = map[string]any{}
	}
	if doc.State.Reported == nil {
		doc.State.Reported = map[string]any{}
	}
	if doc.Metadata.Desired == nil {
		doc.Metadata.Desired = map[string]TimestampedValue{}
	}
	if doc.Metadata.Reported == nil {
		doc.Metadata.Reported = map[string]TimestampedValue{}
	}
}
