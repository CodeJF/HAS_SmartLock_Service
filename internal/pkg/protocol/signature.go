package protocol

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
)

func BuildUserSignature(method, accessToken, appVersion, appID, phoneCode, requestID, timestamp string, params map[string]any, secret string) string {
	baseString := strings.ToUpper(method) + "&" + accessToken + appVersion + appID + phoneCode + requestID + timestamp + "&" + canonicalParams(params)
	return legacySign(baseString, secret)
}

func BuildDeviceSignature(method, model, requestID, timestamp, uuid string, params map[string]any, secret string) string {
	baseString := strings.ToUpper(method) + "&" + model + requestID + timestamp + uuid + "&" + canonicalParams(params)
	return sign(baseString, secret)
}

func canonicalParams(params map[string]any) string {
	if len(params) == 0 {
		return ""
	}

	keys := make([]string, 0, len(params))
	for key := range params {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	pairs := make([]string, 0, len(keys))
	for _, key := range keys {
		value, ok := stringifyValue(params[key])
		if !ok {
			continue
		}
		pairs = append(pairs, key+"="+value)
	}

	return strings.Join(pairs, "&")
}

func stringifyValue(value any) (string, bool) {
	switch v := value.(type) {
	case nil:
		return "", false
	case string:
		if v == "" {
			return "", false
		}
		return v, true
	case bool:
		return strconv.FormatBool(v), true
	case int:
		return strconv.Itoa(v), true
	case int8:
		return strconv.FormatInt(int64(v), 10), true
	case int16:
		return strconv.FormatInt(int64(v), 10), true
	case int32:
		return strconv.FormatInt(int64(v), 10), true
	case int64:
		return strconv.FormatInt(v, 10), true
	case uint:
		return strconv.FormatUint(uint64(v), 10), true
	case uint8:
		return strconv.FormatUint(uint64(v), 10), true
	case uint16:
		return strconv.FormatUint(uint64(v), 10), true
	case uint32:
		return strconv.FormatUint(uint64(v), 10), true
	case uint64:
		return strconv.FormatUint(v, 10), true
	case float32:
		return formatFloat(float64(v)), true
	case float64:
		return formatFloat(v), true
	case json.Number:
		if v == "" {
			return "", false
		}
		return v.String(), true
	default:
		body, err := json.Marshal(v)
		if err != nil || len(bytes.TrimSpace(body)) == 0 || string(body) == "null" || string(body) == "\"\"" {
			return "", false
		}
		return string(body), true
	}
}

func formatFloat(v float64) string {
	if math.Trunc(v) == v {
		return strconv.FormatInt(int64(v), 10)
	}
	return strconv.FormatFloat(v, 'f', -1, 64)
}

func sign(baseString, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(baseString))
	return base64.StdEncoding.EncodeToString(mac.Sum(nil))
}

func legacySign(baseString, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(baseString))
	hexDigest := hex.EncodeToString(mac.Sum(nil))
	return base64.StdEncoding.EncodeToString([]byte(hexDigest))
}

func ParseJSONBodyToMap(body []byte) (map[string]any, error) {
	if len(bytes.TrimSpace(body)) == 0 {
		return map[string]any{}, nil
	}

	var payload any
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, err
	}

	obj, ok := payload.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("json body must be an object")
	}
	return obj, nil
}
