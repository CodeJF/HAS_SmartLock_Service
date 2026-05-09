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
	baseString := strings.ToUpper(method) + "&" + accessToken + appVersion + appID + phoneCode + requestID + timestamp + "&" + canonicalUserParams(params)
	return legacySign(baseString, secret)
}

func BuildDeviceSignature(method, model, requestID, timestamp, uid, uuid string, params map[string]any, secret string) string {
	headerString := model + requestID + timestamp + uuid
	if strings.TrimSpace(uid) != "" {
		headerString = model + requestID + timestamp + uid + uuid
	}
	baseString := strings.ToUpper(method) + "&" + headerString + "&" + canonicalDeviceParams(method, params)
	return legacySign(baseString, secret)
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

func canonicalUserParams(params map[string]any) string {
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
		pairs = append(pairs, key+"="+stringifyUserValue(params[key]))
	}

	return strings.Join(pairs, "&")
}

func stringifyUserValue(value any) string {
	switch v := value.(type) {
	case nil:
		return "null"
	case string:
		return v
	case bool:
		return strconv.FormatBool(v)
	case int:
		return strconv.Itoa(v)
	case int8:
		return strconv.FormatInt(int64(v), 10)
	case int16:
		return strconv.FormatInt(int64(v), 10)
	case int32:
		return strconv.FormatInt(int64(v), 10)
	case int64:
		return strconv.FormatInt(v, 10)
	case uint:
		return strconv.FormatUint(uint64(v), 10)
	case uint8:
		return strconv.FormatUint(uint64(v), 10)
	case uint16:
		return strconv.FormatUint(uint64(v), 10)
	case uint32:
		return strconv.FormatUint(uint64(v), 10)
	case uint64:
		return strconv.FormatUint(v, 10)
	case float32:
		return formatFloat(float64(v))
	case float64:
		return formatFloat(v)
	case json.Number:
		return v.String()
	case []any:
		items := make([]string, 0, len(v))
		for _, item := range v {
			items = append(items, stringifyUserValue(item))
		}
		return strings.Join(items, ",")
	default:
		return "[object Object]"
	}
}

func canonicalDeviceParams(method string, params map[string]any) string {
	if len(params) == 0 {
		return ""
	}

	keys := make([]string, 0, len(params))
	for key := range params {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	pairs := make([]string, 0, len(keys))
	encodeQuery := isDeviceQueryMethod(method)
	for _, key := range keys {
		pairs = append(pairs, key+"="+stringifyDeviceValue(params[key], encodeQuery))
	}

	return strings.Join(pairs, "&")
}

func stringifyDeviceValue(value any, encodeQuery bool) string {
	result := stringifyDeviceValueRaw(value)
	if encodeQuery {
		return encodeURIComponent(result)
	}
	return result
}

func stringifyDeviceValueRaw(value any) string {
	switch v := value.(type) {
	case nil:
		return "null"
	case string:
		return v
	case bool:
		return strconv.FormatBool(v)
	case int:
		return strconv.Itoa(v)
	case int8:
		return strconv.FormatInt(int64(v), 10)
	case int16:
		return strconv.FormatInt(int64(v), 10)
	case int32:
		return strconv.FormatInt(int64(v), 10)
	case int64:
		return strconv.FormatInt(v, 10)
	case uint:
		return strconv.FormatUint(uint64(v), 10)
	case uint8:
		return strconv.FormatUint(uint64(v), 10)
	case uint16:
		return strconv.FormatUint(uint64(v), 10)
	case uint32:
		return strconv.FormatUint(uint64(v), 10)
	case uint64:
		return strconv.FormatUint(v, 10)
	case float32:
		return formatFloat(float64(v))
	case float64:
		return formatFloat(v)
	case json.Number:
		return v.String()
	case []any:
		items := make([]string, 0, len(v))
		for _, item := range v {
			items = append(items, stringifyDeviceValueRaw(item))
		}
		return strings.Join(items, ",")
	default:
		return "[object Object]"
	}
}

func isDeviceQueryMethod(method string) bool {
	switch strings.ToUpper(method) {
	case "GET", "DELETE":
		return true
	default:
		return false
	}
}

func encodeURIComponent(value string) string {
	if value == "" {
		return ""
	}

	var builder strings.Builder
	for _, b := range []byte(value) {
		switch {
		case b >= 'A' && b <= 'Z':
			builder.WriteByte(b)
		case b >= 'a' && b <= 'z':
			builder.WriteByte(b)
		case b >= '0' && b <= '9':
			builder.WriteByte(b)
		case strings.ContainsRune("-_.!~*'()", rune(b)):
			builder.WriteByte(b)
		default:
			builder.WriteString(fmt.Sprintf("%%%02X", b))
		}
	}
	return builder.String()
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
