package redisx

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"

	"has-smartlock-service/internal/pkg/config"
)

type Client struct {
	raw *redis.Client
}

func Open(cfg config.Config) (*Client, error) {
	if cfg.RedisAddr == "" {
		return &Client{}, nil
	}

	raw := redis.NewClient(&redis.Options{
		Addr:     cfg.RedisAddr,
		Password: cfg.RedisPassword,
		DB:       cfg.RedisDB,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := raw.Ping(ctx).Err(); err != nil {
		return nil, err
	}

	return &Client{raw: raw}, nil
}

func (c *Client) Enabled() bool {
	return c != nil && c.raw != nil
}

func (c *Client) Close() error {
	if !c.Enabled() {
		return nil
	}
	return c.raw.Close()
}

func (c *Client) Raw() *redis.Client {
	if c == nil {
		return nil
	}
	return c.raw
}

func (c *Client) SetMQTTCredentials(ctx context.Context, username, password string, isSuperuser bool) error {
	if !c.Enabled() {
		return nil
	}
	salt := RandString(8)
	hash := sha256Hex(password + salt)
	values := map[string]any{
		"password_hash": hash,
		"salt":          salt,
	}
	if isSuperuser {
		values["is_superuser"] = 1
	}
	return c.raw.HSet(ctx, "mqtt_user:"+username, values).Err()
}

func (c *Client) DeleteMQTTCredentials(ctx context.Context, username string) error {
	if !c.Enabled() {
		return nil
	}
	return c.raw.Del(ctx, "mqtt_user:"+username).Err()
}

func (c *Client) SetMQTTACL(ctx context.Context, username string, rules map[string]string) error {
	if !c.Enabled() {
		return nil
	}
	values := make(map[string]any, len(rules))
	for key, value := range rules {
		values[key] = value
	}
	return c.raw.HSet(ctx, "mqtt_acl:"+username, values).Err()
}

func (c *Client) DeleteMQTTACL(ctx context.Context, username string) error {
	if !c.Enabled() {
		return nil
	}
	return c.raw.Del(ctx, "mqtt_acl:"+username).Err()
}

func (c *Client) SetDeviceBinding(ctx context.Context, uuid, uid string, isMaster int) error {
	if !c.Enabled() {
		return nil
	}
	return c.raw.HSet(ctx, "device_bind:"+uuid, uid, isMaster).Err()
}

func (c *Client) DeleteDeviceBinding(ctx context.Context, uuid string) error {
	if !c.Enabled() {
		return nil
	}
	return c.raw.Del(ctx, "device_bind:"+uuid).Err()
}

func (c *Client) DeviceBindings(ctx context.Context, uuid string) (map[string]string, error) {
	if !c.Enabled() {
		return map[string]string{}, nil
	}
	return c.raw.HGetAll(ctx, "device_bind:"+uuid).Result()
}

func (c *Client) StoreJSON(ctx context.Context, key string, value any, ttl time.Duration) error {
	if !c.Enabled() {
		return nil
	}
	body, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return c.raw.Set(ctx, key, body, ttl).Err()
}

func (c *Client) LoadJSON(ctx context.Context, key string, dest any) (bool, error) {
	if !c.Enabled() {
		return false, nil
	}
	raw, err := c.raw.Get(ctx, key).Bytes()
	if err == redis.Nil {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if err := json.Unmarshal(raw, dest); err != nil {
		return false, err
	}
	return true, nil
}

func (c *Client) DeleteKey(ctx context.Context, key string) error {
	if !c.Enabled() {
		return nil
	}
	return c.raw.Del(ctx, key).Err()
}

func (c *Client) PushQueueMessage(ctx context.Context, queue string, value any) error {
	if !c.Enabled() {
		return nil
	}
	body, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return c.raw.RPush(ctx, queue, body).Err()
}

func BuildDeviceACL(model, uuid string) map[string]string {
	base := fmt.Sprintf("/thing/%s/%s", model, uuid)
	return map[string]string{
		base + "/attr/set":        "subscribe",
		base + "/attr/set/resp":   "publish",
		base + "/attr/get":        "publish",
		base + "/attr/get/resp":   "subscribe",
		base + "/attr/post":       "publish",
		base + "/attr/post/resp":  "subscribe",
		base + "/event/+":         "publish",
		base + "/event/+/resp":    "subscribe",
		base + "/func/+":          "subscribe",
		base + "/func/+/resp":     "publish",
	}
}

func RandString(length int) string {
	if length <= 0 {
		return ""
	}
	const alphabet = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	raw := make([]byte, length)
	for i := range raw {
		raw[i] = alphabet[(time.Now().UnixNano()+int64(i))%int64(len(alphabet))]
	}
	return string(raw)
}

func sha256Hex(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}
