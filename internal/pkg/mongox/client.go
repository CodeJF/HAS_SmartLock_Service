package mongox

import (
	"context"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"has-smartlock-service/internal/pkg/config"
)

type Client struct {
	raw      *mongo.Client
	database *mongo.Database
}

func Open(cfg config.Config) (*Client, error) {
	if strings.HasPrefix(strings.TrimSpace(cfg.MongoURI), "memory://") {
		return &Client{}, nil
	}
	if cfg.MongoURI == "" {
		return &Client{}, nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	raw, err := mongo.Connect(options.Client().ApplyURI(cfg.MongoURI))
	if err != nil {
		return nil, err
	}
	if err := raw.Ping(ctx, nil); err != nil {
		_ = raw.Disconnect(context.Background())
		return nil, err
	}

	return &Client{
		raw:      raw,
		database: raw.Database(cfg.MongoDatabase),
	}, nil
}

func (c *Client) Enabled() bool {
	return c != nil && c.raw != nil && c.database != nil
}

func (c *Client) Close(ctx context.Context) error {
	if !c.Enabled() {
		return nil
	}
	return c.raw.Disconnect(ctx)
}

func (c *Client) Database() *mongo.Database {
	if c == nil {
		return nil
	}
	return c.database
}
