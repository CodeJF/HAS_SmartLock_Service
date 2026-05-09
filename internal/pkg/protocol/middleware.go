package protocol

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"has-smartlock-service/internal/pkg/httpx"
)

const (
	defaultTimestampSkewSeconds = 300

	headerAppID      = "appid"
	headerAppVersion = "app_version"
	headerPhoneCode  = "phone_code"
	headerTimestamp  = "timestamp"
	headerRequestID  = "request_id"
	headerSign       = "sign"
	headerAccessTok  = "access_token"
	headerModel      = "model"
	headerUUID       = "uuid"
	headerUID        = "uid"
)

type UserHeaders struct {
	AppID      string
	AppVersion string
	PhoneCode  string
	Timestamp  string
	RequestID  string
	Sign       string
}

type DeviceHeaders struct {
	Model     string
	UUID      string
	UID       string
	AppID     string
	Timestamp string
	RequestID string
	Sign      string
}

func UserMiddleware(secret string, skewSeconds int) (gin.HandlerFunc, error) {
	if strings.TrimSpace(secret) == "" {
		return nil, fmt.Errorf("APP_SECRET_KEY required")
	}
	skew := normalizeSkew(skewSeconds)

	return func(c *gin.Context) {
		headers, ok := readUserHeaders(c)
		if !ok {
			return
		}
		if !validateTimestamp(c, headers.Timestamp, skew) {
			return
		}

		params, ok := extractSignedParams(c)
		if !ok {
			return
		}

		expected := BuildUserSignature(
			c.Request.Method,
			AccessTokenFromHeader(c),
			headers.AppVersion,
			headers.AppID,
			headers.PhoneCode,
			headers.RequestID,
			headers.Timestamp,
			params,
			secret,
		)
		if headers.Sign != expected {
			httpx.Fail(c, 2000, "invalid sign", nil)
			c.Abort()
			return
		}

		c.Next()
	}, nil
}

func DeviceMiddleware(modelSecrets map[string]string, skewSeconds int) (gin.HandlerFunc, error) {
	if len(modelSecrets) == 0 {
		return nil, fmt.Errorf("DEVICE_MODEL_SECRETS required")
	}
	skew := normalizeSkew(skewSeconds)

	return func(c *gin.Context) {
		headers, ok := readDeviceHeaders(c)
		if !ok {
			return
		}
		if !validateTimestamp(c, headers.Timestamp, skew) {
			return
		}

		secret := strings.TrimSpace(modelSecrets[headers.Model])
		if secret == "" {
			httpx.Fail(c, 2000, "invalid model", nil)
			c.Abort()
			return
		}

		params, ok := extractSignedParams(c)
		if !ok {
			return
		}

		expected := BuildDeviceSignature(
			c.Request.Method,
			headers.Model,
			headers.RequestID,
			headers.Timestamp,
			headers.UID,
			headers.UUID,
			params,
			secret,
		)
		if headers.Sign != expected {
			httpx.Fail(c, 2000, "invalid sign", nil)
			c.Abort()
			return
		}

		c.Next()
	}, nil
}

func normalizeSkew(skewSeconds int) time.Duration {
	if skewSeconds <= 0 {
		skewSeconds = defaultTimestampSkewSeconds
	}
	return time.Duration(skewSeconds) * time.Second
}

func readUserHeaders(c *gin.Context) (UserHeaders, bool) {
	headers := UserHeaders{
		AppID:      strings.TrimSpace(c.GetHeader(headerAppID)),
		AppVersion: strings.TrimSpace(c.GetHeader(headerAppVersion)),
		PhoneCode:  strings.TrimSpace(c.GetHeader(headerPhoneCode)),
		Timestamp:  strings.TrimSpace(c.GetHeader(headerTimestamp)),
		RequestID:  strings.TrimSpace(c.GetHeader(headerRequestID)),
		Sign:       strings.TrimSpace(c.GetHeader(headerSign)),
	}

	switch {
	case headers.AppID == "":
		failMissingHeader(c, headerAppID)
	case headers.AppVersion == "":
		failMissingHeader(c, headerAppVersion)
	case headers.PhoneCode == "":
		failMissingHeader(c, headerPhoneCode)
	case headers.Timestamp == "":
		failMissingHeader(c, headerTimestamp)
	case headers.RequestID == "":
		failMissingHeader(c, headerRequestID)
	case headers.Sign == "":
		failMissingHeader(c, headerSign)
	default:
		return headers, true
	}

	c.Abort()
	return UserHeaders{}, false
}

func readDeviceHeaders(c *gin.Context) (DeviceHeaders, bool) {
	headers := DeviceHeaders{
		Model:     strings.TrimSpace(c.GetHeader(headerModel)),
		UUID:      strings.TrimSpace(c.GetHeader(headerUUID)),
		UID:       strings.TrimSpace(c.GetHeader(headerUID)),
		AppID:     strings.TrimSpace(c.GetHeader(headerAppID)),
		Timestamp: strings.TrimSpace(c.GetHeader(headerTimestamp)),
		RequestID: strings.TrimSpace(c.GetHeader(headerRequestID)),
		Sign:      strings.TrimSpace(c.GetHeader(headerSign)),
	}

	switch {
	case headers.Model == "":
		failMissingHeader(c, headerModel)
	case headers.UUID == "":
		failMissingHeader(c, headerUUID)
	case headers.Timestamp == "":
		failMissingHeader(c, headerTimestamp)
	case headers.RequestID == "":
		failMissingHeader(c, headerRequestID)
	case headers.Sign == "":
		failMissingHeader(c, headerSign)
	case c.FullPath() == "/v1/device/bind" && headers.AppID == "":
		failMissingHeader(c, headerAppID)
	case c.FullPath() == "/v1/device/login" && headers.UID == "":
		failMissingHeader(c, headerUID)
	default:
		return headers, true
	}

	c.Abort()
	return DeviceHeaders{}, false
}

func validateTimestamp(c *gin.Context, timestamp string, skew time.Duration) bool {
	seconds, err := strconv.ParseInt(timestamp, 10, 64)
	if err != nil {
		httpx.Fail(c, 1004, "timestamp invalid", nil)
		c.Abort()
		return false
	}

	now := time.Now().Unix()
	if diff := now - seconds; diff > int64(skew/time.Second) || diff < -int64(skew/time.Second) {
		httpx.Fail(c, 1004, "timestamp expired", nil)
		c.Abort()
		return false
	}

	return true
}

func extractSignedParams(c *gin.Context) (map[string]any, bool) {
	switch c.Request.Method {
	case http.MethodGet, http.MethodDelete:
		result := make(map[string]any)
		for key, values := range c.Request.URL.Query() {
			if len(values) == 0 {
				continue
			}
			result[key] = values[0]
		}
		return result, true
	default:
		body, err := io.ReadAll(c.Request.Body)
		if err != nil {
			httpx.Fail(c, 2000, "invalid request body", nil)
			c.Abort()
			return nil, false
		}
		c.Request.Body = io.NopCloser(bytes.NewReader(body))

		payload, err := ParseJSONBodyToMap(body)
		if err != nil {
			httpx.Fail(c, 2000, "invalid request body", nil)
			c.Abort()
			return nil, false
		}
		return payload, true
	}
}

func failMissingHeader(c *gin.Context, name string) {
	httpx.Fail(c, 2000, "missing header: "+name, nil)
}

func AccessTokenFromHeader(c *gin.Context) string {
	return strings.TrimSpace(c.GetHeader(headerAccessTok))
}

func ParseModelSecrets(raw string) (map[string]string, error) {
	payload := strings.TrimSpace(raw)
	if payload == "" {
		return nil, fmt.Errorf("DEVICE_MODEL_SECRETS required")
	}

	result := make(map[string]string)
	if err := json.Unmarshal([]byte(payload), &result); err != nil {
		return nil, err
	}
	for key, value := range result {
		if strings.TrimSpace(key) == "" || strings.TrimSpace(value) == "" {
			return nil, fmt.Errorf("DEVICE_MODEL_SECRETS contains empty model or secret")
		}
	}
	return result, nil
}
