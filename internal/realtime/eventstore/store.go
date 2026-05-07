package eventstore

import (
	"context"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

type BelongTo struct {
	UID      string `bson:"uid" json:"uid"`
	IsRead   int    `bson:"is_read" json:"is_read"`
	Deleted  int    `bson:"deleted" json:"deleted"`
}

type Event struct {
	ID         string         `bson:"_id" json:"id"`
	UUID       string         `bson:"uuid" json:"uuid"`
	HomeID     string         `bson:"home_id,omitempty" json:"home_id,omitempty"`
	DeviceName string         `bson:"device_name,omitempty" json:"device_name,omitempty"`
	Type       int            `bson:"type" json:"type"`
	Time       int64          `bson:"time" json:"time"`
	DeviceTime int64          `bson:"device_time" json:"device_time"`
	Thumbnail  string         `bson:"thumbnail,omitempty" json:"thumbnail,omitempty"`
	Payload    map[string]any `bson:"payload,omitempty" json:"payload,omitempty"`
	BelongTo   []BelongTo     `bson:"belong_to" json:"belong_to"`
	ExpireAt   time.Time      `bson:"expire_at" json:"expire_at"`
}

type Filter struct {
	UID        string
	UUID       string
	HomeID     string
	Types      []int
	StartTime  *int64
	StartAt    *int64
	EndAt      *int64
	Limit      int64
}

type Store interface {
	EnsureIndexes(ctx context.Context) error
	Create(ctx context.Context, event Event) error
	List(ctx context.Context, filter Filter) ([]Event, error)
	CountUnread(ctx context.Context, uid, uuid string) (int64, error)
	MarkRead(ctx context.Context, uid, eventID string) error
	SoftDelete(ctx context.Context, uid, eventID string) error
	CountDays(ctx context.Context, filter Filter) (map[string]int, error)
	GetVisible(ctx context.Context, uid, eventID string) (*Event, error)
}

type MongoStore struct {
	coll *mongo.Collection
}

func NewMongoStore(db *mongo.Database) *MongoStore {
	if db == nil {
		return nil
	}
	return &MongoStore{coll: db.Collection("device_event")}
}

func (s *MongoStore) Enabled() bool {
	return s != nil && s.coll != nil
}

func (s *MongoStore) EnsureIndexes(ctx context.Context) error {
	if !s.Enabled() {
		return nil
	}
	_, err := s.coll.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{Keys: bson.D{{Key: "uuid", Value: 1}, {Key: "time", Value: -1}}},
		{Keys: bson.D{{Key: "belong_to.uid", Value: 1}, {Key: "belong_to.is_read", Value: 1}}},
		{Keys: bson.D{{Key: "expire_at", Value: 1}}, Options: options.Index().SetExpireAfterSeconds(0)},
	})
	return err
}

func (s *MongoStore) Create(ctx context.Context, event Event) error {
	if !s.Enabled() {
		return nil
	}
	if event.ID == "" {
		event.ID = fmt.Sprintf("evt_%d", time.Now().UnixNano())
	}
	if event.ExpireAt.IsZero() {
		event.ExpireAt = time.Now().Add(7 * 24 * time.Hour)
	}
	_, err := s.coll.InsertOne(ctx, event)
	return err
}

func (s *MongoStore) List(ctx context.Context, filter Filter) ([]Event, error) {
	if !s.Enabled() {
		return []Event{}, nil
	}
	cur, err := s.coll.Find(ctx, buildMatch(filter), options.Find().SetSort(bson.D{{Key: "time", Value: -1}}).SetLimit(filter.Limit))
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)

	var result []Event
	for cur.Next(ctx) {
		var item Event
		if err := cur.Decode(&item); err != nil {
			return nil, err
		}
		enrichView(&item, filter.UID)
		if isDeletedForUser(item, filter.UID) {
			continue
		}
		result = append(result, item)
	}
	return result, cur.Err()
}

func (s *MongoStore) CountUnread(ctx context.Context, uid, uuid string) (int64, error) {
	rows, err := s.List(ctx, Filter{UID: uid, UUID: uuid, Limit: 1000})
	if err != nil {
		return 0, err
	}
	var count int64
	for _, row := range rows {
		for _, state := range row.BelongTo {
			if state.UID == uid && state.IsRead == 0 && state.Deleted == 0 {
				count++
				break
			}
		}
	}
	return count, nil
}

func (s *MongoStore) MarkRead(ctx context.Context, uid, eventID string) error {
	if !s.Enabled() {
		return nil
	}
	_, err := s.coll.UpdateOne(ctx, bson.M{"_id": eventID, "belong_to.uid": uid}, bson.M{
		"$set": bson.M{"belong_to.$.is_read": 1},
	})
	return err
}

func (s *MongoStore) SoftDelete(ctx context.Context, uid, eventID string) error {
	if !s.Enabled() {
		return nil
	}
	_, err := s.coll.UpdateOne(ctx, bson.M{"_id": eventID, "belong_to.uid": uid}, bson.M{
		"$set": bson.M{"belong_to.$.is_read": 1, "belong_to.$.deleted": 1},
	})
	return err
}

func (s *MongoStore) CountDays(ctx context.Context, filter Filter) (map[string]int, error) {
	rows, err := s.List(ctx, Filter{
		UID:       filter.UID,
		UUID:      filter.UUID,
		HomeID:    filter.HomeID,
		Types:     filter.Types,
		StartAt:   filter.StartAt,
		EndAt:     filter.EndAt,
		Limit:     5000,
		StartTime: filter.StartTime,
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

func (s *MongoStore) GetVisible(ctx context.Context, uid, eventID string) (*Event, error) {
	if !s.Enabled() {
		return nil, mongo.ErrNoDocuments
	}
	var event Event
	if err := s.coll.FindOne(ctx, bson.M{"_id": eventID, "belong_to.uid": uid}).Decode(&event); err != nil {
		return nil, err
	}
	enrichView(&event, uid)
	if isDeletedForUser(event, uid) {
		return nil, mongo.ErrNoDocuments
	}
	return &event, nil
}

func buildMatch(filter Filter) bson.M {
	match := bson.M{
		"belong_to.uid": filter.UID,
	}
	if filter.UUID != "" {
		match["uuid"] = filter.UUID
	}
	if filter.HomeID != "" {
		match["home_id"] = filter.HomeID
	}
	if filter.StartTime != nil {
		match["time"] = bson.M{"$lt": *filter.StartTime}
	}
	if filter.StartAt != nil || filter.EndAt != nil {
		rangeCond := bson.M{}
		if filter.StartAt != nil {
			rangeCond["$gte"] = *filter.StartAt
		}
		if filter.EndAt != nil {
			rangeCond["$lt"] = *filter.EndAt
		}
		if existing, ok := match["time"].(bson.M); ok {
			for key, value := range rangeCond {
				existing[key] = value
			}
			match["time"] = existing
		} else {
			match["time"] = rangeCond
		}
	}
	if len(filter.Types) > 0 {
		match["type"] = bson.M{"$in": filter.Types}
	}
	return match
}

func enrichView(event *Event, uid string) {
	for i, state := range event.BelongTo {
		if state.UID == uid {
			event.BelongTo = []BelongTo{event.BelongTo[i]}
			return
		}
	}
}

func isDeletedForUser(event Event, uid string) bool {
	for _, state := range event.BelongTo {
		if state.UID == uid {
			return state.Deleted == 1
		}
	}
	return true
}
