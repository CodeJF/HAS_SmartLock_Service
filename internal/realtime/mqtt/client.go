package mqtt

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	paho "github.com/eclipse/paho.mqtt.golang"
	"github.com/google/uuid"

	"has-smartlock-service/internal/pkg/config"
)

type MessageHandler interface {
	HandleAttrSetResp(ctx context.Context, topic string, payload []byte)
	HandleAttrPost(ctx context.Context, topic string, payload []byte)
	HandleAttrGet(ctx context.Context, topic string, payload []byte)
	HandleEvent(ctx context.Context, topic string, payload []byte)
	HandleFuncResp(ctx context.Context, topic string, payload []byte)
	HandleBrokerConnected(ctx context.Context, topic string, payload []byte)
	HandleBrokerDisconnected(ctx context.Context, topic string, payload []byte)
}

type Client struct {
	raw paho.Client
}

func Open(cfg config.Config, handler MessageHandler) (*Client, error) {
	if cfg.MQTTHost == "" || cfg.MQTTUsername == "" {
		return &Client{}, nil
	}

	clientID := cfg.MQTTClientID
	if clientID == "" {
		clientID = uuid.NewString()
	}

	opts := paho.NewClientOptions().
		AddBroker(fmt.Sprintf("tcp://%s:%d", cfg.MQTTHost, cfg.MQTTPort)).
		SetClientID(clientID).
		SetUsername(cfg.MQTTUsername).
		SetPassword(cfg.MQTTPassword).
		SetKeepAlive(time.Duration(cfg.MQTTKeepAliveSeconds) * time.Second).
		SetPingTimeout(time.Duration(cfg.MQTTPingTimeoutSeconds) * time.Second).
		SetCleanSession(cfg.MQTTCleanSession).
		SetAutoReconnect(cfg.MQTTAutoReconnect).
		SetConnectRetry(true).
		SetResumeSubs(true)

	client := &Client{}
	opts.SetOnConnectHandler(func(raw paho.Client) {
		client.raw = raw
		subscribeAll(raw, handler)
	})

	raw := paho.NewClient(opts)
	token := raw.Connect()
	if ok := token.WaitTimeout(5 * time.Second); !ok {
		return nil, fmt.Errorf("mqtt connect timeout")
	}
	if err := token.Error(); err != nil {
		return nil, err
	}
	client.raw = raw
	return client, nil
}

func (c *Client) Enabled() bool {
	return c != nil && c.raw != nil && c.raw.IsConnected()
}

func (c *Client) Publish(ctx context.Context, topic string, payload any) error {
	if c == nil || c.raw == nil {
		return nil
	}
	var body []byte
	switch value := payload.(type) {
	case []byte:
		body = value
	default:
		encoded, err := json.Marshal(payload)
		if err != nil {
			return err
		}
		body = encoded
	}
	token := c.raw.Publish(topic, 0, false, body)
	if ok := token.WaitTimeout(5 * time.Second); !ok {
		return fmt.Errorf("publish timeout")
	}
	return token.Error()
}

func (c *Client) Close() {
	if c == nil || c.raw == nil {
		return
	}
	c.raw.Disconnect(250)
}

func subscribeAll(raw paho.Client, handler MessageHandler) {
	topics := []string{
		"/thing/+/+/attr/set/resp",
		"/thing/+/+/attr/post",
		"/thing/+/+/attr/get",
		"/thing/+/+/event/+",
		"/thing/+/+/func/+/resp",
		"$SYS/brokers/+/clients/+/connected",
		"$SYS/brokers/+/clients/+/disconnected",
	}
	for _, topic := range topics {
		_ = raw.Subscribe(topic, 0, func(_ paho.Client, msg paho.Message) {
			if handler == nil {
				return
			}
			ctx := context.Background()
			switch {
			case strings.HasSuffix(msg.Topic(), "/attr/set/resp"):
				handler.HandleAttrSetResp(ctx, msg.Topic(), msg.Payload())
			case strings.HasSuffix(msg.Topic(), "/attr/post"):
				handler.HandleAttrPost(ctx, msg.Topic(), msg.Payload())
			case strings.HasSuffix(msg.Topic(), "/attr/get"):
				handler.HandleAttrGet(ctx, msg.Topic(), msg.Payload())
			case strings.Contains(msg.Topic(), "/event/"):
				handler.HandleEvent(ctx, msg.Topic(), msg.Payload())
			case strings.Contains(msg.Topic(), "/func/") && strings.HasSuffix(msg.Topic(), "/resp"):
				handler.HandleFuncResp(ctx, msg.Topic(), msg.Payload())
			case strings.HasSuffix(msg.Topic(), "/connected"):
				handler.HandleBrokerConnected(ctx, msg.Topic(), msg.Payload())
			case strings.HasSuffix(msg.Topic(), "/disconnected"):
				handler.HandleBrokerDisconnected(ctx, msg.Topic(), msg.Payload())
			}
		})
	}
}
