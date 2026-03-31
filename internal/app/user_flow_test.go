package app

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"gorm.io/gorm"

	"has-smartlock-service/internal/pkg/config"
	"has-smartlock-service/internal/pkg/db"
)

type envelope struct {
	Code int             `json:"code"`
	Msg  string          `json:"msg"`
	Data json.RawMessage `json:"data"`
}

type tokenResponse struct {
	UID          string `json:"uid"`
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	IsDebug      int    `json:"is_debug"`
	Expiration   int64  `json:"expiration"`
}

type userInfoResponse struct {
	Username     string `json:"username"`
	Nickname     string `json:"nickname"`
	Avatar       string `json:"avatar"`
	IsDebug      int    `json:"is_debug"`
	RegisterTime int64  `json:"register_time"`
}

func TestUserRegisterLoginFlow(t *testing.T) {
	application := newTestApp(t)

	registerSendResp := performJSONRequest(t, application.router, http.MethodPost, "/v1/user/registerSend", map[string]any{
		"username": "demo@example.com",
		"country":  "86",
	}, "")
	if registerSendResp.Code != 1000 {
		t.Fatalf("registerSend code = %d, want 1000", registerSendResp.Code)
	}

	registerResp := performJSONRequest(t, application.router, http.MethodPost, "/v1/user/register", map[string]any{
		"username": "demo@example.com",
		"country":  "86",
		"code":     "123456",
		"password": "md5-password",
	}, "")
	if registerResp.Code != 1000 {
		t.Fatalf("register code = %d, want 1000", registerResp.Code)
	}

	var registered tokenResponse
	if err := json.Unmarshal(registerResp.Data, &registered); err != nil {
		t.Fatalf("unmarshal register response: %v", err)
	}
	if registered.AccessToken == "" || registered.RefreshToken == "" || registered.UID == "" {
		t.Fatalf("register returned incomplete tokens: %+v", registered)
	}

	loginResp := performJSONRequest(t, application.router, http.MethodPost, "/v1/user/login", map[string]any{
		"username":    "demo@example.com",
		"type":        "password",
		"password":    "md5-password",
		"phone_brand": "iPhone",
	}, "")
	if loginResp.Code != 1000 {
		t.Fatalf("login code = %d, want 1000", loginResp.Code)
	}

	var loggedIn tokenResponse
	if err := json.Unmarshal(loginResp.Data, &loggedIn); err != nil {
		t.Fatalf("unmarshal login response: %v", err)
	}

	infoResp := performJSONRequest(t, application.router, http.MethodGet, "/v1/user/info", nil, "Bearer "+loggedIn.AccessToken)
	if infoResp.Code != 1000 {
		t.Fatalf("info code = %d, want 1000", infoResp.Code)
	}

	var userInfo userInfoResponse
	if err := json.Unmarshal(infoResp.Data, &userInfo); err != nil {
		t.Fatalf("unmarshal info response: %v", err)
	}
	if userInfo.Username != "demo@example.com" {
		t.Fatalf("username = %q, want demo@example.com", userInfo.Username)
	}
}

func newTestApp(t *testing.T) *App {
	t.Helper()

	cfg := config.Config{
		AppName:         "has-smartlock-service-test",
		AppEnv:          "test",
		HTTPAddr:        ":0",
		DBDriver:        "sqlite",
		DBDSN:           "file:user_flow_test?mode=memory&cache=shared",
		AutoMigrate:     true,
		JWTSecret:       "test-secret",
		AccessTokenTTL:  3600,
		RefreshTokenTTL: 86400,
		VerificationTTL: 300,
	}

	database, err := db.Open(cfg)
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}

	cleanupTables(t, database)

	application, err := NewWithDependencies(cfg, database)
	if err != nil {
		t.Fatalf("new app: %v", err)
	}

	return application
}

func cleanupTables(t *testing.T, database *gorm.DB) {
	t.Helper()

	for _, table := range []string{"user_clients", "refresh_tokens", "verification_codes", "users"} {
		if err := database.Exec("DELETE FROM " + table).Error; err != nil {
			t.Fatalf("cleanup table %s: %v", table, err)
		}
	}
}

func performJSONRequest(t *testing.T, router http.Handler, method, path string, body any, authHeader string) envelope {
	t.Helper()

	var payload []byte
	var err error
	if body != nil {
		payload, err = json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal request body: %v", err)
		}
	}

	req := httptest.NewRequest(method, path, bytes.NewReader(payload))
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if authHeader != "" {
		req.Header.Set("Authorization", authHeader)
	}

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	var response envelope
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("unmarshal response body: %v; body=%s", err, recorder.Body.String())
	}

	return response
}
