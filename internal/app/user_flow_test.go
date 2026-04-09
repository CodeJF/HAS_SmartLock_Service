package app

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	gosqlmysql "github.com/go-sql-driver/mysql"
	"gorm.io/gorm"

	devicemodel "has-smartlock-service/internal/device/model"
	homemodel "has-smartlock-service/internal/home/model"
	"has-smartlock-service/internal/pkg/config"
	"has-smartlock-service/internal/pkg/db"
	"has-smartlock-service/internal/pkg/protocol"
	usermodel "has-smartlock-service/internal/user/model"
)

type envelope struct {
	Code int             `json:"code"`
	Msg  string          `json:"msg"`
	Data json.RawMessage `json:"data"`
}

type tokenResponse struct {
	UID          string `json:"uid"`
	Username     string `json:"-"`
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

type cloudTokenResponse struct {
	AccessTokenID   string `json:"access_token_id"`
	AccessKeySecret string `json:"access_key_secret"`
	SecurityToken   string `json:"security_token"`
	Expiration      int64  `json:"expiration"`
	RegionID        string `json:"region_id"`
	Endpoint        string `json:"endpoint"`
	Bucket          string `json:"bucket"`
}

type homeItemResponse struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Location   string `json:"location"`
	Count      int    `json:"count"`
	Role       int    `json:"role"`
	CreateTime int64  `json:"create_time"`
}

type homeUserResponse struct {
	UID      string `json:"uid"`
	Username string `json:"username"`
	Avatar   string `json:"avatar"`
	Role     int    `json:"role"`
	Accept   int    `json:"accept"`
}

type homeDeviceStateResponse struct {
	Desired  map[string]any `json:"desired"`
	Reported map[string]any `json:"reported"`
}

type homeDeviceResponse struct {
	ID            int                     `json:"id"`
	UUID          string                  `json:"uuid"`
	DeviceID      string                  `json:"device_id"`
	UID           string                  `json:"uid"`
	BindType      int                     `json:"bind_type"`
	Secret        string                  `json:"secret"`
	Name          string                  `json:"name"`
	FirstBindTime int64                   `json:"first_bind_time"`
	BindTime      int64                   `json:"bind_time"`
	State         homeDeviceStateResponse `json:"State"`
}

type newDeviceResponse struct {
	UUID          string `json:"uuid"`
	DeviceID      string `json:"device_id"`
	UID           string `json:"uid"`
	BindType      int    `json:"bind_type"`
	Secret        string `json:"secret"`
	Name          string `json:"name"`
	BindStatus    int    `json:"bind_status"`
	FirstBindTime int64  `json:"first_bind_time"`
	BindTime      int64  `json:"bind_time"`
	DeleteTime    int64  `json:"delete_time"`
}

type messagePayloadResponse struct {
	HomeID   string `json:"home_id"`
	HomeName string `json:"home_name"`
	Status   int    `json:"status"`
	UID      string `json:"uid"`
	Username string `json:"username"`
}

type messageItemResponse struct {
	ID      string                 `json:"id"`
	UID     string                 `json:"uid"`
	Type    int                    `json:"type"`
	Time    int64                  `json:"time"`
	IsRead  int                    `json:"is_read"`
	Payload messagePayloadResponse `json:"payload"`
}

type messageListResponse struct {
	Has  bool                  `json:"has"`
	List []messageItemResponse `json:"list"`
}

type unreadNumResponse struct {
	Number int64 `json:"number"`
}

type requestOptions struct {
	AccessToken       string
	SkipUserHeaders   bool
	SkipDeviceHeaders bool
	HeaderOverrides   map[string]string
	Timestamp         int64
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

	loginSendResp := performJSONRequest(t, application.router, http.MethodPost, "/v1/user/loginSend", map[string]any{
		"username": "demo@example.com",
		"country":  "86",
	}, "")
	if loginSendResp.Code != 1000 {
		t.Fatalf("loginSend code = %d, want 1000", loginSendResp.Code)
	}

	validateResp := performJSONRequest(t, application.router, http.MethodPost, "/v1/user/validateCode", map[string]any{
		"username": "demo@example.com",
		"code":     "123456",
		"type":     "login",
	}, "")
	if validateResp.Code != 1000 {
		t.Fatalf("validateCode code = %d, want 1000", validateResp.Code)
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
	if userInfo.Avatar != "avatar/"+loggedIn.UID {
		t.Fatalf("avatar = %q, want %q", userInfo.Avatar, "avatar/"+loggedIn.UID)
	}

	refreshResp := performJSONRequest(t, application.router, http.MethodPost, "/v1/user/refresh", map[string]any{
		"refresh_token": loggedIn.RefreshToken,
	}, "")
	if refreshResp.Code != 1000 {
		t.Fatalf("refresh code = %d, want 1000", refreshResp.Code)
	}

	var refreshed tokenResponse
	if err := json.Unmarshal(refreshResp.Data, &refreshed); err != nil {
		t.Fatalf("unmarshal refresh response: %v", err)
	}
	if refreshed.RefreshToken == loggedIn.RefreshToken {
		t.Fatalf("refresh token should rotate")
	}

	reusedRefreshResp := performJSONRequest(t, application.router, http.MethodPost, "/v1/user/refresh", map[string]any{
		"refresh_token": loggedIn.RefreshToken,
	}, "")
	if reusedRefreshResp.Code != 2007 {
		t.Fatalf("reused refresh code = %d, want 2007", reusedRefreshResp.Code)
	}

	logoutResp := performJSONRequest(t, application.router, http.MethodPost, "/v1/user/logout", nil, "Bearer "+refreshed.AccessToken)
	if logoutResp.Code != 1000 {
		t.Fatalf("logout code = %d, want 1000", logoutResp.Code)
	}

	refreshAfterLogoutResp := performJSONRequest(t, application.router, http.MethodPost, "/v1/user/refresh", map[string]any{
		"refresh_token": refreshed.RefreshToken,
	}, "")
	if refreshAfterLogoutResp.Code != 2007 {
		t.Fatalf("refresh after logout code = %d, want 2007", refreshAfterLogoutResp.Code)
	}

	infoWithOldAccessResp := performJSONRequest(t, application.router, http.MethodGet, "/v1/user/info", nil, "Bearer "+refreshed.AccessToken)
	if infoWithOldAccessResp.Code != 1000 {
		t.Fatalf("info with old access token code = %d, want 1000", infoWithOldAccessResp.Code)
	}
}

func TestValidateCodeFailures(t *testing.T) {
	application := newTestApp(t)

	sendResp := performJSONRequest(t, application.router, http.MethodPost, "/v1/user/registerSend", map[string]any{
		"username": "expired@example.com",
		"country":  "86",
	}, "")
	if sendResp.Code != 1000 {
		t.Fatalf("registerSend code = %d, want 1000", sendResp.Code)
	}

	invalidResp := performJSONRequest(t, application.router, http.MethodPost, "/v1/user/validateCode", map[string]any{
		"username": "expired@example.com",
		"code":     "000000",
		"type":     "register",
	}, "")
	if invalidResp.Code != 2005 {
		t.Fatalf("invalid code = %d, want 2005", invalidResp.Code)
	}

	expiredApp := newExpiredCodeTestApp(t)
	expiredSendResp := performJSONRequest(t, expiredApp.router, http.MethodPost, "/v1/user/registerSend", map[string]any{
		"username": "expired2@example.com",
		"country":  "86",
	}, "")
	if expiredSendResp.Code != 1000 {
		t.Fatalf("expired app registerSend code = %d, want 1000", expiredSendResp.Code)
	}

	expiredValidateResp := performJSONRequest(t, expiredApp.router, http.MethodPost, "/v1/user/validateCode", map[string]any{
		"username": "expired2@example.com",
		"code":     "123456",
		"type":     "register",
	}, "")
	if expiredValidateResp.Code != 2006 {
		t.Fatalf("expired validate code = %d, want 2006", expiredValidateResp.Code)
	}
}

func TestUserResetPasswordFlow(t *testing.T) {
	application := newTestApp(t)

	registerSendResp := performJSONRequest(t, application.router, http.MethodPost, "/v1/user/registerSend", map[string]any{
		"username": "reset@example.com",
		"country":  "86",
	}, "")
	if registerSendResp.Code != 1000 {
		t.Fatalf("registerSend code = %d, want 1000", registerSendResp.Code)
	}

	registerResp := performJSONRequest(t, application.router, http.MethodPost, "/v1/user/register", map[string]any{
		"username": "reset@example.com",
		"country":  "86",
		"code":     "123456",
		"password": "old-password",
	}, "")
	if registerResp.Code != 1000 {
		t.Fatalf("register code = %d, want 1000", registerResp.Code)
	}

	resetSendResp := performJSONRequest(t, application.router, http.MethodPost, "/v1/user/resetSend", map[string]any{
		"username": "reset@example.com",
		"country":  "86",
	}, "")
	if resetSendResp.Code != 1000 {
		t.Fatalf("resetSend code = %d, want 1000", resetSendResp.Code)
	}

	resetResp := performJSONRequest(t, application.router, http.MethodPost, "/v1/user/reset", map[string]any{
		"username": "reset@example.com",
		"code":     "123456",
		"password": "new-password",
	}, "")
	if resetResp.Code != 1000 {
		t.Fatalf("reset code = %d, want 1000", resetResp.Code)
	}

	reusedResetResp := performJSONRequest(t, application.router, http.MethodPost, "/v1/user/reset", map[string]any{
		"username": "reset@example.com",
		"code":     "123456",
		"password": "another-password",
	}, "")
	if reusedResetResp.Code != 2005 {
		t.Fatalf("reused reset code = %d, want 2005", reusedResetResp.Code)
	}

	oldLoginResp := performJSONRequest(t, application.router, http.MethodPost, "/v1/user/login", map[string]any{
		"username":    "reset@example.com",
		"type":        "password",
		"password":    "old-password",
		"phone_brand": "iPhone",
	}, "")
	if oldLoginResp.Code != 2004 {
		t.Fatalf("old password login code = %d, want 2004", oldLoginResp.Code)
	}

	newLoginResp := performJSONRequest(t, application.router, http.MethodPost, "/v1/user/login", map[string]any{
		"username":    "reset@example.com",
		"type":        "password",
		"password":    "new-password",
		"phone_brand": "iPhone",
	}, "")
	if newLoginResp.Code != 1000 {
		t.Fatalf("new password login code = %d, want 1000", newLoginResp.Code)
	}
}

func TestResetFailures(t *testing.T) {
	application := newTestApp(t)

	invalidResetSendResp := performJSONRequest(t, application.router, http.MethodPost, "/v1/user/resetSend", map[string]any{
		"username": "missing-country@example.com",
	}, "")
	if invalidResetSendResp.Code != 2000 {
		t.Fatalf("invalid resetSend code = %d, want 2000", invalidResetSendResp.Code)
	}

	resetUserNotFoundResp := performJSONRequest(t, application.router, http.MethodPost, "/v1/user/reset", map[string]any{
		"username": "missing@example.com",
		"code":     "123456",
		"password": "new-password",
	}, "")
	if resetUserNotFoundResp.Code != 2005 {
		t.Fatalf("reset without code record code = %d, want 2005", resetUserNotFoundResp.Code)
	}

	registerSendResp := performJSONRequest(t, application.router, http.MethodPost, "/v1/user/registerSend", map[string]any{
		"username": "invalid-reset@example.com",
		"country":  "86",
	}, "")
	if registerSendResp.Code != 1000 {
		t.Fatalf("registerSend code = %d, want 1000", registerSendResp.Code)
	}

	registerResp := performJSONRequest(t, application.router, http.MethodPost, "/v1/user/register", map[string]any{
		"username": "invalid-reset@example.com",
		"country":  "86",
		"code":     "123456",
		"password": "old-password",
	}, "")
	if registerResp.Code != 1000 {
		t.Fatalf("register code = %d, want 1000", registerResp.Code)
	}

	resetSendResp := performJSONRequest(t, application.router, http.MethodPost, "/v1/user/resetSend", map[string]any{
		"username": "invalid-reset@example.com",
		"country":  "86",
	}, "")
	if resetSendResp.Code != 1000 {
		t.Fatalf("resetSend code = %d, want 1000", resetSendResp.Code)
	}

	invalidCodeResp := performJSONRequest(t, application.router, http.MethodPost, "/v1/user/reset", map[string]any{
		"username": "invalid-reset@example.com",
		"code":     "000000",
		"password": "new-password",
	}, "")
	if invalidCodeResp.Code != 2005 {
		t.Fatalf("invalid reset code = %d, want 2005", invalidCodeResp.Code)
	}

	expiredApp := newExpiredCodeTestApp(t)
	expiredRegisterSendResp := performJSONRequest(t, expiredApp.router, http.MethodPost, "/v1/user/registerSend", map[string]any{
		"username": "expired-reset@example.com",
		"country":  "86",
	}, "")
	if expiredRegisterSendResp.Code != 1000 {
		t.Fatalf("expired registerSend code = %d, want 1000", expiredRegisterSendResp.Code)
	}

	expiredRegisterResp := performJSONRequest(t, expiredApp.router, http.MethodPost, "/v1/user/register", map[string]any{
		"username": "expired-reset@example.com",
		"country":  "86",
		"code":     "123456",
		"password": "old-password",
	}, "")
	if expiredRegisterResp.Code != 2006 {
		t.Fatalf("expired register code = %d, want 2006", expiredRegisterResp.Code)
	}

	normalApp := newTestApp(t)
	normalRegisterSendResp := performJSONRequest(t, normalApp.router, http.MethodPost, "/v1/user/registerSend", map[string]any{
		"username": "expired-reset@example.com",
		"country":  "86",
	}, "")
	if normalRegisterSendResp.Code != 1000 {
		t.Fatalf("normal registerSend code = %d, want 1000", normalRegisterSendResp.Code)
	}

	normalRegisterResp := performJSONRequest(t, normalApp.router, http.MethodPost, "/v1/user/register", map[string]any{
		"username": "expired-reset@example.com",
		"country":  "86",
		"code":     "123456",
		"password": "old-password",
	}, "")
	if normalRegisterResp.Code != 1000 {
		t.Fatalf("normal register code = %d, want 1000", normalRegisterResp.Code)
	}

	expiredResetApp := newExpiredCodeTestApp(t)
	expiredResetSendResp := performJSONRequest(t, expiredResetApp.router, http.MethodPost, "/v1/user/resetSend", map[string]any{
		"username": "expired-reset@example.com",
		"country":  "86",
	}, "")
	if expiredResetSendResp.Code != 1000 {
		t.Fatalf("expired resetSend code = %d, want 1000", expiredResetSendResp.Code)
	}

	expiredResetResp := performJSONRequest(t, expiredResetApp.router, http.MethodPost, "/v1/user/reset", map[string]any{
		"username": "expired-reset@example.com",
		"code":     "123456",
		"password": "new-password",
	}, "")
	if expiredResetResp.Code != 2006 {
		t.Fatalf("expired reset code = %d, want 2006", expiredResetResp.Code)
	}
}

func TestUserProfileManagementFlow(t *testing.T) {
	application := newTestApp(t)

	registerSendResp := performJSONRequest(t, application.router, http.MethodPost, "/v1/user/registerSend", map[string]any{
		"username": "profile@example.com",
		"country":  "86",
	}, "")
	if registerSendResp.Code != 1000 {
		t.Fatalf("registerSend code = %d, want 1000", registerSendResp.Code)
	}

	registerResp := performJSONRequest(t, application.router, http.MethodPost, "/v1/user/register", map[string]any{
		"username": "profile@example.com",
		"country":  "86",
		"code":     "123456",
		"password": "old-password",
	}, "")
	if registerResp.Code != 1000 {
		t.Fatalf("register code = %d, want 1000", registerResp.Code)
	}

	var registered tokenResponse
	if err := json.Unmarshal(registerResp.Data, &registered); err != nil {
		t.Fatalf("unmarshal register response: %v", err)
	}

	updatePwdResp := performJSONRequest(t, application.router, http.MethodPost, "/v1/user/updatePwd", map[string]any{
		"new_password": "new-password",
	}, "Bearer "+registered.AccessToken)
	if updatePwdResp.Code != 1000 {
		t.Fatalf("updatePwd code = %d, want 1000", updatePwdResp.Code)
	}

	oldLoginResp := performJSONRequest(t, application.router, http.MethodPost, "/v1/user/login", map[string]any{
		"username":    "profile@example.com",
		"type":        "password",
		"password":    "old-password",
		"phone_brand": "iPhone",
	}, "")
	if oldLoginResp.Code != 2004 {
		t.Fatalf("old password login code = %d, want 2004", oldLoginResp.Code)
	}

	newLoginResp := performJSONRequest(t, application.router, http.MethodPost, "/v1/user/login", map[string]any{
		"username":    "profile@example.com",
		"type":        "password",
		"password":    "new-password",
		"phone_brand": "iPhone",
	}, "")
	if newLoginResp.Code != 1000 {
		t.Fatalf("new password login code = %d, want 1000", newLoginResp.Code)
	}

	updateInfoResp := performJSONRequest(t, application.router, http.MethodPost, "/v1/user/updateInfo", map[string]any{
		"nickname": "Profile Nick",
	}, "Bearer "+registered.AccessToken)
	if updateInfoResp.Code != 1000 {
		t.Fatalf("updateInfo code = %d, want 1000", updateInfoResp.Code)
	}

	putClientResp := performJSONRequest(t, application.router, http.MethodPost, "/v1/user/putClient", map[string]any{
		"push_type":  1,
		"push_token": "device-token-1",
		"brand":      "iPhone",
		"version":    "iOS 18",
		"language":   "zh_CN",
		"zone":       "Asia/Shanghai",
	}, "Bearer "+registered.AccessToken)
	if putClientResp.Code != 1000 {
		t.Fatalf("putClient code = %d, want 1000", putClientResp.Code)
	}

	infoResp := performJSONRequest(t, application.router, http.MethodGet, "/v1/user/info", nil, "Bearer "+registered.AccessToken)
	if infoResp.Code != 1000 {
		t.Fatalf("info code = %d, want 1000", infoResp.Code)
	}

	var userInfo userInfoResponse
	if err := json.Unmarshal(infoResp.Data, &userInfo); err != nil {
		t.Fatalf("unmarshal info response: %v", err)
	}
	if userInfo.Nickname != "Profile Nick" {
		t.Fatalf("nickname = %q, want Profile Nick", userInfo.Nickname)
	}

	assertUserClientSaved(t, application, "device-token-1", "iPhone", "iOS 18", "zh_CN", "Asia/Shanghai")
}

func TestAuthorizedProfileFailures(t *testing.T) {
	application := newTestApp(t)

	registerSendResp := performJSONRequest(t, application.router, http.MethodPost, "/v1/user/registerSend", map[string]any{
		"username": "authfail@example.com",
		"country":  "86",
	}, "")
	if registerSendResp.Code != 1000 {
		t.Fatalf("registerSend code = %d, want 1000", registerSendResp.Code)
	}

	registerResp := performJSONRequest(t, application.router, http.MethodPost, "/v1/user/register", map[string]any{
		"username": "authfail@example.com",
		"country":  "86",
		"code":     "123456",
		"password": "password",
	}, "")
	if registerResp.Code != 1000 {
		t.Fatalf("register code = %d, want 1000", registerResp.Code)
	}

	var registered tokenResponse
	if err := json.Unmarshal(registerResp.Data, &registered); err != nil {
		t.Fatalf("unmarshal register response: %v", err)
	}

	missingAuthResp := performJSONRequest(t, application.router, http.MethodPost, "/v1/user/updatePwd", map[string]any{
		"new_password": "new-password",
	}, "")
	if missingAuthResp.Code != 2001 {
		t.Fatalf("missing auth code = %d, want 2001", missingAuthResp.Code)
	}

	invalidBodyResp := performJSONRequest(t, application.router, http.MethodPost, "/v1/user/updateInfo", map[string]any{}, "Bearer "+registered.AccessToken)
	if invalidBodyResp.Code != 2000 {
		t.Fatalf("invalid updateInfo code = %d, want 2000", invalidBodyResp.Code)
	}

	invalidPutClientResp := performJSONRequest(t, application.router, http.MethodPost, "/v1/user/putClient", map[string]any{
		"push_type": 1,
	}, "Bearer "+registered.AccessToken)
	if invalidPutClientResp.Code != 2000 {
		t.Fatalf("invalid putClient code = %d, want 2000", invalidPutClientResp.Code)
	}

	missingAuthGetTokenResp := performJSONRequest(t, application.router, http.MethodGet, "/v1/cloud/getToken?uuid=device-1", nil, "")
	if missingAuthGetTokenResp.Code != 2001 {
		t.Fatalf("missing auth getToken code = %d, want 2001", missingAuthGetTokenResp.Code)
	}

	missingSignResp := performJSONRequestWithOptions(t, application.router, http.MethodPost, "/v1/user/login", map[string]any{
		"username":    "authfail@example.com",
		"type":        "password",
		"password":    "password",
		"phone_brand": "iPhone",
	}, requestOptions{SkipUserHeaders: true})
	if missingSignResp.Code != 2000 {
		t.Fatalf("missing sign code = %d, want 2000", missingSignResp.Code)
	}

	invalidSignResp := performJSONRequestWithOptions(t, application.router, http.MethodPost, "/v1/user/login", map[string]any{
		"username":    "authfail@example.com",
		"type":        "password",
		"password":    "password",
		"phone_brand": "iPhone",
	}, requestOptions{HeaderOverrides: map[string]string{"sign": "invalid-sign"}})
	if invalidSignResp.Code != 2000 {
		t.Fatalf("invalid sign code = %d, want 2000", invalidSignResp.Code)
	}

	expiredTimestampResp := performJSONRequestWithOptions(t, application.router, http.MethodPost, "/v1/user/login", map[string]any{
		"username":    "authfail@example.com",
		"type":        "password",
		"password":    "password",
		"phone_brand": "iPhone",
	}, requestOptions{Timestamp: time.Now().Add(-10 * time.Minute).Unix()})
	if expiredTimestampResp.Code != 1004 {
		t.Fatalf("expired timestamp code = %d, want 1004", expiredTimestampResp.Code)
	}
}

func TestCloudGetTokenFlow(t *testing.T) {
	application := newTestApp(t)

	registerSendResp := performJSONRequest(t, application.router, http.MethodPost, "/v1/user/registerSend", map[string]any{
		"username": "avatar@example.com",
		"country":  "86",
	}, "")
	if registerSendResp.Code != 1000 {
		t.Fatalf("registerSend code = %d, want 1000", registerSendResp.Code)
	}

	registerResp := performJSONRequest(t, application.router, http.MethodPost, "/v1/user/register", map[string]any{
		"username": "avatar@example.com",
		"country":  "86",
		"code":     "123456",
		"password": "avatar-password",
	}, "")
	if registerResp.Code != 1000 {
		t.Fatalf("register code = %d, want 1000", registerResp.Code)
	}

	var registered tokenResponse
	if err := json.Unmarshal(registerResp.Data, &registered); err != nil {
		t.Fatalf("unmarshal register response: %v", err)
	}

	infoResp := performJSONRequest(t, application.router, http.MethodGet, "/v1/user/info", nil, "Bearer "+registered.AccessToken)
	if infoResp.Code != 1000 {
		t.Fatalf("info code = %d, want 1000", infoResp.Code)
	}

	var userInfo userInfoResponse
	if err := json.Unmarshal(infoResp.Data, &userInfo); err != nil {
		t.Fatalf("unmarshal info response: %v", err)
	}
	if userInfo.Avatar != "avatar/"+registered.UID {
		t.Fatalf("avatar = %q, want %q", userInfo.Avatar, "avatar/"+registered.UID)
	}

	missingUUIDResp := performJSONRequest(t, application.router, http.MethodGet, "/v1/cloud/getToken", nil, "Bearer "+registered.AccessToken)
	if missingUUIDResp.Code != 2000 {
		t.Fatalf("missing uuid getToken code = %d, want 2000", missingUUIDResp.Code)
	}

	getTokenResp := performJSONRequest(t, application.router, http.MethodGet, "/v1/cloud/getToken?uuid=device-123", nil, "Bearer "+registered.AccessToken)
	if getTokenResp.Code != 1000 {
		t.Fatalf("getToken code = %d, want 1000", getTokenResp.Code)
	}

	var token cloudTokenResponse
	if err := json.Unmarshal(getTokenResp.Data, &token); err != nil {
		t.Fatalf("unmarshal getToken response: %v", err)
	}
	if token.AccessTokenID == "" || token.AccessKeySecret == "" || token.SecurityToken == "" {
		t.Fatalf("sts token fields should not be empty: %+v", token)
	}
	if token.Expiration == 0 {
		t.Fatalf("expiration should not be zero")
	}
	if token.RegionID != "cn-shenzhen" {
		t.Fatalf("region_id = %q, want cn-shenzhen", token.RegionID)
	}
	if token.Endpoint != "oss-cn-shenzhen.aliyuncs.com" {
		t.Fatalf("endpoint = %q, want oss-cn-shenzhen.aliyuncs.com", token.Endpoint)
	}
	if token.Bucket != "has-smartlock" {
		t.Fatalf("bucket = %q, want has-smartlock", token.Bucket)
	}
}

func TestHomeBasicFlow(t *testing.T) {
	application := newTestApp(t)

	registerSendResp := performJSONRequest(t, application.router, http.MethodPost, "/v1/user/registerSend", map[string]any{
		"username": "home-owner@example.com",
		"country":  "86",
	}, "")
	if registerSendResp.Code != 1000 {
		t.Fatalf("registerSend code = %d, want 1000", registerSendResp.Code)
	}

	registerResp := performJSONRequest(t, application.router, http.MethodPost, "/v1/user/register", map[string]any{
		"username": "home-owner@example.com",
		"country":  "86",
		"code":     "123456",
		"password": "home-password",
	}, "")
	if registerResp.Code != 1000 {
		t.Fatalf("register code = %d, want 1000", registerResp.Code)
	}

	var registered tokenResponse
	if err := json.Unmarshal(registerResp.Data, &registered); err != nil {
		t.Fatalf("unmarshal register response: %v", err)
	}

	createResp := performJSONRequest(t, application.router, http.MethodPost, "/v1/device/homeCreate", map[string]any{
		"name": "My Home",
	}, "Bearer "+registered.AccessToken)
	if createResp.Code != 1000 {
		t.Fatalf("homeCreate code = %d, want 1000", createResp.Code)
	}

	homesResp := performJSONRequest(t, application.router, http.MethodGet, "/v1/device/homes", nil, "Bearer "+registered.AccessToken)
	if homesResp.Code != 1000 {
		t.Fatalf("homes code = %d, want 1000", homesResp.Code)
	}

	var homes []homeItemResponse
	if err := json.Unmarshal(homesResp.Data, &homes); err != nil {
		t.Fatalf("unmarshal homes response: %v", err)
	}
	if len(homes) != 1 {
		t.Fatalf("homes length = %d, want 1", len(homes))
	}
	if homes[0].Name != "My Home" {
		t.Fatalf("home name = %q, want My Home", homes[0].Name)
	}
	if homes[0].Count != 0 {
		t.Fatalf("home count = %d, want 0", homes[0].Count)
	}
	if homes[0].Role != 1 {
		t.Fatalf("home role = %d, want 1", homes[0].Role)
	}
	if homes[0].ID == "" {
		t.Fatalf("home id should not be empty")
	}
	homeID := homes[0].ID

	homeUsersResp := performJSONRequest(t, application.router, http.MethodGet, "/v1/device/homeUsers?home_id="+homes[0].ID, nil, "Bearer "+registered.AccessToken)
	if homeUsersResp.Code != 1000 {
		t.Fatalf("homeUsers code = %d, want 1000", homeUsersResp.Code)
	}

	var homeUsers []homeUserResponse
	if err := json.Unmarshal(homeUsersResp.Data, &homeUsers); err != nil {
		t.Fatalf("unmarshal homeUsers response: %v", err)
	}
	if len(homeUsers) != 1 {
		t.Fatalf("homeUsers length = %d, want 1", len(homeUsers))
	}
	if homeUsers[0].UID != registered.UID {
		t.Fatalf("home user uid = %q, want %q", homeUsers[0].UID, registered.UID)
	}
	if homeUsers[0].Avatar != "avatar/"+registered.UID {
		t.Fatalf("home user avatar = %q, want %q", homeUsers[0].Avatar, "avatar/"+registered.UID)
	}
	if homeUsers[0].Role != 1 {
		t.Fatalf("home user role = %d, want 1", homeUsers[0].Role)
	}
	if homeUsers[0].Accept != 1 {
		t.Fatalf("home user accept = %d, want 1", homeUsers[0].Accept)
	}

	homeDevicesResp := performJSONRequest(t, application.router, http.MethodGet, "/v1/device/homeDevices?home_id="+homeID, nil, "Bearer "+registered.AccessToken)
	if homeDevicesResp.Code != 1000 {
		t.Fatalf("homeDevices code = %d, want 1000", homeDevicesResp.Code)
	}

	var homeDevices []homeDeviceResponse
	if err := json.Unmarshal(homeDevicesResp.Data, &homeDevices); err != nil {
		t.Fatalf("unmarshal homeDevices response: %v", err)
	}
	if len(homeDevices) != 0 {
		t.Fatalf("homeDevices length = %d, want 0", len(homeDevices))
	}

	updateResp := performJSONRequest(t, application.router, http.MethodPost, "/v1/device/homeUpdate", map[string]any{
		"home_id":  homeID,
		"name":     "Updated Home",
		"location": "Shenzhen",
	}, "Bearer "+registered.AccessToken)
	if updateResp.Code != 1000 {
		t.Fatalf("homeUpdate code = %d, want 1000", updateResp.Code)
	}

	homesAfterUpdateResp := performJSONRequest(t, application.router, http.MethodGet, "/v1/device/homes", nil, "Bearer "+registered.AccessToken)
	if homesAfterUpdateResp.Code != 1000 {
		t.Fatalf("homes after update code = %d, want 1000", homesAfterUpdateResp.Code)
	}
	if err := json.Unmarshal(homesAfterUpdateResp.Data, &homes); err != nil {
		t.Fatalf("unmarshal homes after update response: %v", err)
	}
	if homes[0].Name != "Updated Home" {
		t.Fatalf("updated home name = %q, want Updated Home", homes[0].Name)
	}
	if homes[0].Location != "Shenzhen" {
		t.Fatalf("updated home location = %q, want Shenzhen", homes[0].Location)
	}

	deleteResp := performJSONRequest(t, application.router, http.MethodDelete, "/v1/device/homeDelete?home_id="+homeID, nil, "Bearer "+registered.AccessToken)
	if deleteResp.Code != 1000 {
		t.Fatalf("homeDelete code = %d, want 1000", deleteResp.Code)
	}

	homesAfterDeleteResp := performJSONRequest(t, application.router, http.MethodGet, "/v1/device/homes", nil, "Bearer "+registered.AccessToken)
	if homesAfterDeleteResp.Code != 1000 {
		t.Fatalf("homes after delete code = %d, want 1000", homesAfterDeleteResp.Code)
	}
	if err := json.Unmarshal(homesAfterDeleteResp.Data, &homes); err != nil {
		t.Fatalf("unmarshal homes after delete response: %v", err)
	}
	if len(homes) != 0 {
		t.Fatalf("homes after delete length = %d, want 0", len(homes))
	}

	homeUsersAfterDeleteResp := performJSONRequest(t, application.router, http.MethodGet, "/v1/device/homeUsers?home_id="+homeID, nil, "Bearer "+registered.AccessToken)
	if homeUsersAfterDeleteResp.Code != 3001 {
		t.Fatalf("homeUsers after delete code = %d, want 3001", homeUsersAfterDeleteResp.Code)
	}

	homeDevicesAfterDeleteResp := performJSONRequest(t, application.router, http.MethodGet, "/v1/device/homeDevices?home_id="+homeID, nil, "Bearer "+registered.AccessToken)
	if homeDevicesAfterDeleteResp.Code != 3001 {
		t.Fatalf("homeDevices after delete code = %d, want 3001", homeDevicesAfterDeleteResp.Code)
	}
}

func TestHomeFailures(t *testing.T) {
	application := newTestApp(t)

	registerSendResp := performJSONRequest(t, application.router, http.MethodPost, "/v1/user/registerSend", map[string]any{
		"username": "home-fail@example.com",
		"country":  "86",
	}, "")
	if registerSendResp.Code != 1000 {
		t.Fatalf("registerSend code = %d, want 1000", registerSendResp.Code)
	}

	registerResp := performJSONRequest(t, application.router, http.MethodPost, "/v1/user/register", map[string]any{
		"username": "home-fail@example.com",
		"country":  "86",
		"code":     "123456",
		"password": "password",
	}, "")
	if registerResp.Code != 1000 {
		t.Fatalf("register code = %d, want 1000", registerResp.Code)
	}

	var registered tokenResponse
	if err := json.Unmarshal(registerResp.Data, &registered); err != nil {
		t.Fatalf("unmarshal register response: %v", err)
	}

	missingAuthCreateResp := performJSONRequest(t, application.router, http.MethodPost, "/v1/device/homeCreate", map[string]any{
		"name": "No Auth Home",
	}, "")
	if missingAuthCreateResp.Code != 2001 {
		t.Fatalf("missing auth homeCreate code = %d, want 2001", missingAuthCreateResp.Code)
	}

	invalidCreateResp := performJSONRequest(t, application.router, http.MethodPost, "/v1/device/homeCreate", map[string]any{}, "Bearer "+registered.AccessToken)
	if invalidCreateResp.Code != 2000 {
		t.Fatalf("invalid homeCreate code = %d, want 2000", invalidCreateResp.Code)
	}

	createResp := performJSONRequest(t, application.router, http.MethodPost, "/v1/device/homeCreate", map[string]any{
		"name": "Fail Home",
	}, "Bearer "+registered.AccessToken)
	if createResp.Code != 1000 {
		t.Fatalf("homeCreate code = %d, want 1000", createResp.Code)
	}

	homesResp := performJSONRequest(t, application.router, http.MethodGet, "/v1/device/homes", nil, "Bearer "+registered.AccessToken)
	if homesResp.Code != 1000 {
		t.Fatalf("homes code = %d, want 1000", homesResp.Code)
	}
	var homes []homeItemResponse
	if err := json.Unmarshal(homesResp.Data, &homes); err != nil {
		t.Fatalf("unmarshal homes response: %v", err)
	}
	homeID := homes[0].ID

	missingHomeUsersResp := performJSONRequest(t, application.router, http.MethodGet, "/v1/device/homeUsers", nil, "Bearer "+registered.AccessToken)
	if missingHomeUsersResp.Code != 2000 {
		t.Fatalf("missing homeUsers home_id code = %d, want 2000", missingHomeUsersResp.Code)
	}

	missingHomeDevicesResp := performJSONRequest(t, application.router, http.MethodGet, "/v1/device/homeDevices", nil, "Bearer "+registered.AccessToken)
	if missingHomeDevicesResp.Code != 2000 {
		t.Fatalf("missing homeDevices home_id code = %d, want 2000", missingHomeDevicesResp.Code)
	}

	notFoundHomeUsersResp := performJSONRequest(t, application.router, http.MethodGet, "/v1/device/homeUsers?home_id=h_missing", nil, "Bearer "+registered.AccessToken)
	if notFoundHomeUsersResp.Code != 3001 {
		t.Fatalf("missing homeUsers code = %d, want 3001", notFoundHomeUsersResp.Code)
	}

	notFoundHomeDevicesResp := performJSONRequest(t, application.router, http.MethodGet, "/v1/device/homeDevices?home_id=h_missing", nil, "Bearer "+registered.AccessToken)
	if notFoundHomeDevicesResp.Code != 3001 {
		t.Fatalf("missing homeDevices code = %d, want 3001", notFoundHomeDevicesResp.Code)
	}

	invalidUpdateResp := performJSONRequest(t, application.router, http.MethodPost, "/v1/device/homeUpdate", map[string]any{
		"home_id": homeID,
		"name":    "Only Name",
	}, "Bearer "+registered.AccessToken)
	if invalidUpdateResp.Code != 2000 {
		t.Fatalf("invalid homeUpdate code = %d, want 2000", invalidUpdateResp.Code)
	}

	missingDeleteResp := performJSONRequest(t, application.router, http.MethodDelete, "/v1/device/homeDelete", nil, "Bearer "+registered.AccessToken)
	if missingDeleteResp.Code != 2000 {
		t.Fatalf("missing homeDelete home_id code = %d, want 2000", missingDeleteResp.Code)
	}

	unauthorizedHomeDevicesResp := performJSONRequest(t, application.router, http.MethodGet, "/v1/device/homeDevices?home_id="+homeID, nil, "")
	if unauthorizedHomeDevicesResp.Code != 2001 {
		t.Fatalf("unauthorized homeDevices code = %d, want 2001", unauthorizedHomeDevicesResp.Code)
	}
}

func TestUserDeleteAccountFlow(t *testing.T) {
	application := newTestApp(t)

	registerSendResp := performJSONRequest(t, application.router, http.MethodPost, "/v1/user/registerSend", map[string]any{
		"username": "delete@example.com",
		"country":  "86",
	}, "")
	if registerSendResp.Code != 1000 {
		t.Fatalf("registerSend code = %d, want 1000", registerSendResp.Code)
	}

	registerResp := performJSONRequest(t, application.router, http.MethodPost, "/v1/user/register", map[string]any{
		"username": "delete@example.com",
		"country":  "86",
		"code":     "123456",
		"password": "delete-password",
	}, "")
	if registerResp.Code != 1000 {
		t.Fatalf("register code = %d, want 1000", registerResp.Code)
	}

	var registered tokenResponse
	if err := json.Unmarshal(registerResp.Data, &registered); err != nil {
		t.Fatalf("unmarshal register response: %v", err)
	}

	deleteSendResp := performJSONRequest(t, application.router, http.MethodPost, "/v1/user/deleteSend", map[string]any{
		"username": "delete@example.com",
	}, "Bearer "+registered.AccessToken)
	if deleteSendResp.Code != 1000 {
		t.Fatalf("deleteSend code = %d, want 1000", deleteSendResp.Code)
	}

	deleteResp := performJSONRequest(t, application.router, http.MethodPost, "/v1/user/delete", map[string]any{
		"username": "delete@example.com",
		"code":     "123456",
	}, "Bearer "+registered.AccessToken)
	if deleteResp.Code != 1000 {
		t.Fatalf("delete code = %d, want 1000", deleteResp.Code)
	}

	reusedDeleteResp := performJSONRequest(t, application.router, http.MethodPost, "/v1/user/delete", map[string]any{
		"username": "delete@example.com",
		"code":     "123456",
	}, "Bearer "+registered.AccessToken)
	if reusedDeleteResp.Code != 2003 {
		t.Fatalf("reused delete code = %d, want 2003", reusedDeleteResp.Code)
	}

	loginResp := performJSONRequest(t, application.router, http.MethodPost, "/v1/user/login", map[string]any{
		"username":    "delete@example.com",
		"type":        "password",
		"password":    "delete-password",
		"phone_brand": "iPhone",
	}, "")
	if loginResp.Code != 2003 {
		t.Fatalf("deleted user login code = %d, want 2003", loginResp.Code)
	}

	refreshResp := performJSONRequest(t, application.router, http.MethodPost, "/v1/user/refresh", map[string]any{
		"refresh_token": registered.RefreshToken,
	}, "")
	if refreshResp.Code != 2007 {
		t.Fatalf("deleted user refresh code = %d, want 2007", refreshResp.Code)
	}

	infoResp := performJSONRequest(t, application.router, http.MethodGet, "/v1/user/info", nil, "Bearer "+registered.AccessToken)
	if infoResp.Code != 2003 {
		t.Fatalf("deleted user info code = %d, want 2003", infoResp.Code)
	}
}

func TestDeleteAccountFailures(t *testing.T) {
	application := newTestApp(t)

	registerSendResp := performJSONRequest(t, application.router, http.MethodPost, "/v1/user/registerSend", map[string]any{
		"username": "delete-fail@example.com",
		"country":  "86",
	}, "")
	if registerSendResp.Code != 1000 {
		t.Fatalf("registerSend code = %d, want 1000", registerSendResp.Code)
	}

	registerResp := performJSONRequest(t, application.router, http.MethodPost, "/v1/user/register", map[string]any{
		"username": "delete-fail@example.com",
		"country":  "86",
		"code":     "123456",
		"password": "password",
	}, "")
	if registerResp.Code != 1000 {
		t.Fatalf("register code = %d, want 1000", registerResp.Code)
	}

	var registered tokenResponse
	if err := json.Unmarshal(registerResp.Data, &registered); err != nil {
		t.Fatalf("unmarshal register response: %v", err)
	}

	missingAuthDeleteSendResp := performJSONRequest(t, application.router, http.MethodPost, "/v1/user/deleteSend", map[string]any{
		"username": "delete-fail@example.com",
	}, "")
	if missingAuthDeleteSendResp.Code != 2001 {
		t.Fatalf("missing auth deleteSend code = %d, want 2001", missingAuthDeleteSendResp.Code)
	}

	invalidDeleteSendResp := performJSONRequest(t, application.router, http.MethodPost, "/v1/user/deleteSend", map[string]any{
		"username": "other@example.com",
	}, "Bearer "+registered.AccessToken)
	if invalidDeleteSendResp.Code != 2000 {
		t.Fatalf("invalid deleteSend code = %d, want 2000", invalidDeleteSendResp.Code)
	}

	deleteSendResp := performJSONRequest(t, application.router, http.MethodPost, "/v1/user/deleteSend", map[string]any{
		"username": "delete-fail@example.com",
	}, "Bearer "+registered.AccessToken)
	if deleteSendResp.Code != 1000 {
		t.Fatalf("deleteSend code = %d, want 1000", deleteSendResp.Code)
	}

	invalidCodeDeleteResp := performJSONRequest(t, application.router, http.MethodPost, "/v1/user/delete", map[string]any{
		"username": "delete-fail@example.com",
		"code":     "000000",
	}, "Bearer "+registered.AccessToken)
	if invalidCodeDeleteResp.Code != 2005 {
		t.Fatalf("invalid delete code = %d, want 2005", invalidCodeDeleteResp.Code)
	}

	mismatchDeleteResp := performJSONRequest(t, application.router, http.MethodPost, "/v1/user/delete", map[string]any{
		"username": "other@example.com",
		"code":     "123456",
	}, "Bearer "+registered.AccessToken)
	if mismatchDeleteResp.Code != 2000 {
		t.Fatalf("mismatch delete code = %d, want 2000", mismatchDeleteResp.Code)
	}

	expiredApp := newExpiredCodeTestApp(t)
	expiredRegisterSendResp := performJSONRequest(t, expiredApp.router, http.MethodPost, "/v1/user/registerSend", map[string]any{
		"username": "expired-delete@example.com",
		"country":  "86",
	}, "")
	if expiredRegisterSendResp.Code != 1000 {
		t.Fatalf("expired registerSend code = %d, want 1000", expiredRegisterSendResp.Code)
	}

	expiredRegisterResp := performJSONRequest(t, expiredApp.router, http.MethodPost, "/v1/user/register", map[string]any{
		"username": "expired-delete@example.com",
		"country":  "86",
		"code":     "123456",
		"password": "password",
	}, "")
	if expiredRegisterResp.Code != 2006 {
		t.Fatalf("expired register code = %d, want 2006", expiredRegisterResp.Code)
	}

	normalApp := newTestApp(t)
	normalRegisterSendResp := performJSONRequest(t, normalApp.router, http.MethodPost, "/v1/user/registerSend", map[string]any{
		"username": "expired-delete@example.com",
		"country":  "86",
	}, "")
	if normalRegisterSendResp.Code != 1000 {
		t.Fatalf("normal registerSend code = %d, want 1000", normalRegisterSendResp.Code)
	}

	normalRegisterResp := performJSONRequest(t, normalApp.router, http.MethodPost, "/v1/user/register", map[string]any{
		"username": "expired-delete@example.com",
		"country":  "86",
		"code":     "123456",
		"password": "password",
	}, "")
	if normalRegisterResp.Code != 1000 {
		t.Fatalf("normal register code = %d, want 1000", normalRegisterResp.Code)
	}

	var normalRegistered tokenResponse
	if err := json.Unmarshal(normalRegisterResp.Data, &normalRegistered); err != nil {
		t.Fatalf("unmarshal normal register response: %v", err)
	}

	expiredDeleteApp := newExpiredCodeTestApp(t)
	expiredDeleteSendResp := performJSONRequest(t, expiredDeleteApp.router, http.MethodPost, "/v1/user/deleteSend", map[string]any{
		"username": "expired-delete@example.com",
	}, "Bearer "+normalRegistered.AccessToken)
	if expiredDeleteSendResp.Code != 2003 {
		t.Fatalf("expired deleteSend with foreign db code = %d, want 2003", expiredDeleteSendResp.Code)
	}
}

func TestDeviceBasicFlow(t *testing.T) {
	application := newTestApp(t)
	registered := registerUserForTest(t, application, "device-owner@example.com")

	homeID := createHomeForTest(t, application, registered.AccessToken, "Device Home")
	insertOwnedDeviceForTest(t, application, registered.UID, "dev-uuid-1", "device-001", "Front Door")
	addDeviceToHomeForTest(t, application, registered.AccessToken, homeID, "dev-uuid-1")

	listResp := performJSONRequest(t, application.router, http.MethodGet, "/v1/device/list", nil, "Bearer "+registered.AccessToken)
	if listResp.Code != 1000 {
		t.Fatalf("device list code = %d, want 1000", listResp.Code)
	}

	var devices []homeDeviceResponse
	if err := json.Unmarshal(listResp.Data, &devices); err != nil {
		t.Fatalf("unmarshal device list response: %v", err)
	}
	if len(devices) != 1 {
		t.Fatalf("device list length = %d, want 1", len(devices))
	}
	if devices[0].UUID != "dev-uuid-1" {
		t.Fatalf("device uuid = %q, want dev-uuid-1", devices[0].UUID)
	}
	if devices[0].Name != "Front Door" {
		t.Fatalf("device name = %q, want Front Door", devices[0].Name)
	}
	if devices[0].State.Desired == nil || devices[0].State.Reported == nil {
		t.Fatalf("device state should contain empty objects")
	}

	homeListResp := performJSONRequest(t, application.router, http.MethodGet, "/v1/device/list?home_id="+homeID, nil, "Bearer "+registered.AccessToken)
	if homeListResp.Code != 1000 {
		t.Fatalf("device list by home code = %d, want 1000", homeListResp.Code)
	}
	if err := json.Unmarshal(homeListResp.Data, &devices); err != nil {
		t.Fatalf("unmarshal device list by home response: %v", err)
	}
	if len(devices) != 1 {
		t.Fatalf("device list by home length = %d, want 1", len(devices))
	}

	newListResp := performJSONRequest(t, application.router, http.MethodGet, "/v1/device/newList", nil, "Bearer "+registered.AccessToken)
	if newListResp.Code != 1000 {
		t.Fatalf("newList code = %d, want 1000", newListResp.Code)
	}

	var newDevices []newDeviceResponse
	if err := json.Unmarshal(newListResp.Data, &newDevices); err != nil {
		t.Fatalf("unmarshal newList response: %v", err)
	}
	if len(newDevices) != 1 {
		t.Fatalf("newList length = %d, want 1", len(newDevices))
	}
	if newDevices[0].BindStatus != 1 {
		t.Fatalf("newList bind_status = %d, want 1", newDevices[0].BindStatus)
	}
	if newDevices[0].DeleteTime != 0 {
		t.Fatalf("newList delete_time = %d, want 0", newDevices[0].DeleteTime)
	}

	upNameResp := performJSONRequest(t, application.router, http.MethodPost, "/v1/device/upName", map[string]any{
		"uuid": "dev-uuid-1",
		"name": "Back Door",
	}, "Bearer "+registered.AccessToken)
	if upNameResp.Code != 1000 {
		t.Fatalf("upName code = %d, want 1000", upNameResp.Code)
	}

	listAfterRenameResp := performJSONRequest(t, application.router, http.MethodGet, "/v1/device/list", nil, "Bearer "+registered.AccessToken)
	if listAfterRenameResp.Code != 1000 {
		t.Fatalf("device list after rename code = %d, want 1000", listAfterRenameResp.Code)
	}
	if err := json.Unmarshal(listAfterRenameResp.Data, &devices); err != nil {
		t.Fatalf("unmarshal device list after rename response: %v", err)
	}
	if devices[0].Name != "Back Door" {
		t.Fatalf("device name after rename = %q, want Back Door", devices[0].Name)
	}

	homeDevicesResp := performJSONRequest(t, application.router, http.MethodGet, "/v1/device/homeDevices?home_id="+homeID, nil, "Bearer "+registered.AccessToken)
	if homeDevicesResp.Code != 1000 {
		t.Fatalf("homeDevices after insert code = %d, want 1000", homeDevicesResp.Code)
	}
	if err := json.Unmarshal(homeDevicesResp.Data, &devices); err != nil {
		t.Fatalf("unmarshal homeDevices after insert response: %v", err)
	}
	if len(devices) != 1 {
		t.Fatalf("homeDevices after insert length = %d, want 1", len(devices))
	}
	if devices[0].Name != "Back Door" {
		t.Fatalf("homeDevices device name = %q, want Back Door", devices[0].Name)
	}
}

func TestDeviceBindAndLoginFlow(t *testing.T) {
	application := newTestApp(t)
	registered := registerUserForTest(t, application, "device-bind@example.com")

	bindResp := performDeviceJSONRequest(t, application.router, http.MethodPost, "/v1/device/bind", map[string]any{
		"uid":     registered.UID,
		"mac":     "AA:BB:CC:DD:EE:FF",
		"zone":    "8.00",
		"version": "1.0.0",
	}, requestOptions{HeaderOverrides: map[string]string{"uuid": "bind-uuid-1"}})
	if bindResp.Code != 1000 {
		t.Fatalf("device bind code = %d, want 1000", bindResp.Code)
	}

	loginResp := performDeviceJSONRequest(t, application.router, http.MethodPost, "/v1/device/login", map[string]any{
		"zone":    "8.00",
		"version": "SL100_BP_1.01.10",
	}, requestOptions{HeaderOverrides: map[string]string{"uuid": "bind-uuid-1", "uid": registered.UID}})
	if loginResp.Code != 1000 {
		t.Fatalf("device login code = %d, want 1000", loginResp.Code)
	}

	newListResp := performJSONRequest(t, application.router, http.MethodGet, "/v1/device/newList", nil, "Bearer "+registered.AccessToken)
	if newListResp.Code != 1000 {
		t.Fatalf("newList after bind code = %d, want 1000", newListResp.Code)
	}

	var devices []newDeviceResponse
	if err := json.Unmarshal(newListResp.Data, &devices); err != nil {
		t.Fatalf("unmarshal newList after bind response: %v", err)
	}
	if len(devices) != 1 {
		t.Fatalf("newList after bind length = %d, want 1", len(devices))
	}
	if devices[0].UUID != "bind-uuid-1" {
		t.Fatalf("newList uuid = %q, want bind-uuid-1", devices[0].UUID)
	}
}

func TestDeviceFailures(t *testing.T) {
	application := newTestApp(t)
	registered := registerUserForTest(t, application, "device-fail@example.com")
	other := registerUserForTest(t, application, "device-other@example.com")

	homeID := createHomeForTest(t, application, registered.AccessToken, "Owner Home")
	insertOwnedDeviceForTest(t, application, registered.UID, "dev-uuid-fail", "device-fail-001", "Device Fail")
	addDeviceToHomeForTest(t, application, registered.AccessToken, homeID, "dev-uuid-fail")

	missingHomeListResp := performJSONRequest(t, application.router, http.MethodGet, "/v1/device/list?home_id=h_missing", nil, "Bearer "+registered.AccessToken)
	if missingHomeListResp.Code != 3001 {
		t.Fatalf("device list missing home code = %d, want 3001", missingHomeListResp.Code)
	}

	unauthorizedListResp := performJSONRequest(t, application.router, http.MethodGet, "/v1/device/list", nil, "")
	if unauthorizedListResp.Code != 2001 {
		t.Fatalf("unauthorized device list code = %d, want 2001", unauthorizedListResp.Code)
	}

	emptyNewListResp := performJSONRequest(t, application.router, http.MethodGet, "/v1/device/newList", nil, "Bearer "+other.AccessToken)
	if emptyNewListResp.Code != 1000 {
		t.Fatalf("empty newList code = %d, want 1000", emptyNewListResp.Code)
	}
	var newDevices []newDeviceResponse
	if err := json.Unmarshal(emptyNewListResp.Data, &newDevices); err != nil {
		t.Fatalf("unmarshal empty newList response: %v", err)
	}
	if len(newDevices) != 0 {
		t.Fatalf("empty newList length = %d, want 0", len(newDevices))
	}

	invalidUpNameResp := performJSONRequest(t, application.router, http.MethodPost, "/v1/device/upName", map[string]any{
		"uuid": "dev-uuid-fail",
	}, "Bearer "+registered.AccessToken)
	if invalidUpNameResp.Code != 2000 {
		t.Fatalf("invalid upName code = %d, want 2000", invalidUpNameResp.Code)
	}

	missingDeviceUpNameResp := performJSONRequest(t, application.router, http.MethodPost, "/v1/device/upName", map[string]any{
		"uuid": "dev-missing",
		"name": "Missing",
	}, "Bearer "+registered.AccessToken)
	if missingDeviceUpNameResp.Code != 4001 {
		t.Fatalf("missing device upName code = %d, want 4001", missingDeviceUpNameResp.Code)
	}

	forbiddenUpNameResp := performJSONRequest(t, application.router, http.MethodPost, "/v1/device/upName", map[string]any{
		"uuid": "dev-uuid-fail",
		"name": "Other Name",
	}, "Bearer "+other.AccessToken)
	if forbiddenUpNameResp.Code != 4002 {
		t.Fatalf("forbidden upName code = %d, want 4002", forbiddenUpNameResp.Code)
	}

	forbiddenHomeListResp := performJSONRequest(t, application.router, http.MethodGet, "/v1/device/list?home_id="+homeID, nil, "Bearer "+other.AccessToken)
	if forbiddenHomeListResp.Code != 3001 {
		t.Fatalf("forbidden home-scoped device list code = %d, want 3001", forbiddenHomeListResp.Code)
	}

	bindInvalidSignResp := performDeviceJSONRequest(t, application.router, http.MethodPost, "/v1/device/bind", map[string]any{
		"uid":     registered.UID,
		"mac":     "AA:BB:CC:DD:EE:11",
		"zone":    "8.00",
		"version": "1.0.0",
	}, requestOptions{HeaderOverrides: map[string]string{"sign": "bad-sign"}})
	if bindInvalidSignResp.Code != 2000 {
		t.Fatalf("device bind invalid sign code = %d, want 2000", bindInvalidSignResp.Code)
	}

	bindMissingHeaderResp := performDeviceJSONRequest(t, application.router, http.MethodPost, "/v1/device/bind", map[string]any{
		"uid":     registered.UID,
		"mac":     "AA:BB:CC:DD:EE:12",
		"zone":    "8.00",
		"version": "1.0.0",
	}, requestOptions{HeaderOverrides: map[string]string{"appid": ""}})
	if bindMissingHeaderResp.Code != 2000 {
		t.Fatalf("device bind missing header code = %d, want 2000", bindMissingHeaderResp.Code)
	}

	loginMissingDeviceResp := performDeviceJSONRequest(t, application.router, http.MethodPost, "/v1/device/login", map[string]any{
		"zone":    "8.00",
		"version": "SL100_BP_1.01.10",
	}, requestOptions{HeaderOverrides: map[string]string{"uuid": "missing-device", "uid": registered.UID}})
	if loginMissingDeviceResp.Code != 4001 {
		t.Fatalf("device login missing device code = %d, want 4001", loginMissingDeviceResp.Code)
	}

	bindResp := performDeviceJSONRequest(t, application.router, http.MethodPost, "/v1/device/bind", map[string]any{
		"uid":     registered.UID,
		"mac":     "AA:BB:CC:DD:EE:13",
		"zone":    "8.00",
		"version": "1.0.0",
	}, requestOptions{HeaderOverrides: map[string]string{"uuid": "login-forbidden-device"}})
	if bindResp.Code != 1000 {
		t.Fatalf("device bind setup code = %d, want 1000", bindResp.Code)
	}

	loginForbiddenResp := performDeviceJSONRequest(t, application.router, http.MethodPost, "/v1/device/login", map[string]any{
		"zone":    "8.00",
		"version": "SL100_BP_1.01.10",
	}, requestOptions{HeaderOverrides: map[string]string{"uuid": "login-forbidden-device", "uid": other.UID}})
	if loginForbiddenResp.Code != 4002 {
		t.Fatalf("device login forbidden code = %d, want 4002", loginForbiddenResp.Code)
	}

	missingHomeAddResp := performJSONRequest(t, application.router, http.MethodPost, "/v1/device/homeAddDevice", map[string]any{
		"uuid": "dev-uuid-fail",
	}, "Bearer "+registered.AccessToken)
	if missingHomeAddResp.Code != 2000 {
		t.Fatalf("missing homeAddDevice home_id code = %d, want 2000", missingHomeAddResp.Code)
	}

	missingUUIDAddResp := performJSONRequest(t, application.router, http.MethodPost, "/v1/device/homeAddDevice", map[string]any{
		"home_id": homeID,
	}, "Bearer "+registered.AccessToken)
	if missingUUIDAddResp.Code != 2000 {
		t.Fatalf("missing homeAddDevice uuid code = %d, want 2000", missingUUIDAddResp.Code)
	}

	missingDeviceAddResp := performJSONRequest(t, application.router, http.MethodPost, "/v1/device/homeAddDevice", map[string]any{
		"home_id": homeID,
		"uuid":    "device-missing",
	}, "Bearer "+registered.AccessToken)
	if missingDeviceAddResp.Code != 4001 {
		t.Fatalf("missing device homeAddDevice code = %d, want 4001", missingDeviceAddResp.Code)
	}

	otherHomeID := createHomeForTest(t, application, other.AccessToken, "Other Home")
	forbiddenOwnerAddResp := performJSONRequest(t, application.router, http.MethodPost, "/v1/device/homeAddDevice", map[string]any{
		"home_id": homeID,
		"uuid":    "login-forbidden-device",
	}, "Bearer "+other.AccessToken)
	if forbiddenOwnerAddResp.Code != 3001 {
		t.Fatalf("non-member homeAddDevice code = %d, want 3001", forbiddenOwnerAddResp.Code)
	}

	deviceOwnedByOther := "other-owned-device"
	insertOwnedDeviceForTest(t, application, other.UID, deviceOwnedByOther, "other-device-001", "Other Device")
	forbiddenDeviceOwnerResp := performJSONRequest(t, application.router, http.MethodPost, "/v1/device/homeAddDevice", map[string]any{
		"home_id": homeID,
		"uuid":    deviceOwnedByOther,
	}, "Bearer "+registered.AccessToken)
	if forbiddenDeviceOwnerResp.Code != 4002 {
		t.Fatalf("foreign device homeAddDevice code = %d, want 4002", forbiddenDeviceOwnerResp.Code)
	}

	conflictUUID := "device-home-conflict"
	insertOwnedDeviceForTest(t, application, registered.UID, conflictUUID, "device-conflict-001", "Conflict Device")
	addDeviceToHomeForTest(t, application, registered.AccessToken, homeID, conflictUUID)
	secondHomeID := createHomeForTest(t, application, registered.AccessToken, "Second Home")
	conflictAddResp := performJSONRequest(t, application.router, http.MethodPost, "/v1/device/homeAddDevice", map[string]any{
		"home_id": secondHomeID,
		"uuid":    conflictUUID,
	}, "Bearer "+registered.AccessToken)
	if conflictAddResp.Code != 4002 {
		t.Fatalf("cross-home homeAddDevice code = %d, want 4002", conflictAddResp.Code)
	}

	otherHomeAddResp := performJSONRequest(t, application.router, http.MethodPost, "/v1/device/homeAddDevice", map[string]any{
		"home_id": otherHomeID,
		"uuid":    "login-forbidden-device",
	}, "Bearer "+other.AccessToken)
	if otherHomeAddResp.Code != 4002 {
		t.Fatalf("other owner foreign device homeAddDevice code = %d, want 4002", otherHomeAddResp.Code)
	}
}

func TestHomeAddDeviceFlow(t *testing.T) {
	application := newTestApp(t)
	registered := registerUserForTest(t, application, "home-add-device@example.com")
	other := registerUserForTest(t, application, "home-add-device-other@example.com")

	homeID := createHomeForTest(t, application, registered.AccessToken, "Family Home")
	insertOwnedDeviceForTest(t, application, registered.UID, "home-add-uuid-1", "home-add-device-001", "Side Door")

	addResp := performJSONRequest(t, application.router, http.MethodPost, "/v1/device/homeAddDevice", map[string]any{
		"home_id": homeID,
		"uuid":    "home-add-uuid-1",
	}, "Bearer "+registered.AccessToken)
	if addResp.Code != 1000 {
		t.Fatalf("homeAddDevice code = %d, want 1000", addResp.Code)
	}

	homeDevicesResp := performJSONRequest(t, application.router, http.MethodGet, "/v1/device/homeDevices?home_id="+homeID, nil, "Bearer "+registered.AccessToken)
	if homeDevicesResp.Code != 1000 {
		t.Fatalf("homeDevices after add code = %d, want 1000", homeDevicesResp.Code)
	}

	var devices []homeDeviceResponse
	if err := json.Unmarshal(homeDevicesResp.Data, &devices); err != nil {
		t.Fatalf("unmarshal homeDevices after add response: %v", err)
	}
	if len(devices) != 1 {
		t.Fatalf("homeDevices after add length = %d, want 1", len(devices))
	}
	if devices[0].UUID != "home-add-uuid-1" {
		t.Fatalf("homeDevices uuid = %q, want home-add-uuid-1", devices[0].UUID)
	}

	idempotentResp := performJSONRequest(t, application.router, http.MethodPost, "/v1/device/homeAddDevice", map[string]any{
		"home_id": homeID,
		"uuid":    "home-add-uuid-1",
	}, "Bearer "+registered.AccessToken)
	if idempotentResp.Code != 1000 {
		t.Fatalf("homeAddDevice idempotent code = %d, want 1000", idempotentResp.Code)
	}

	homeDevicesAfterRepeatResp := performJSONRequest(t, application.router, http.MethodGet, "/v1/device/homeDevices?home_id="+homeID, nil, "Bearer "+registered.AccessToken)
	if homeDevicesAfterRepeatResp.Code != 1000 {
		t.Fatalf("homeDevices after idempotent add code = %d, want 1000", homeDevicesAfterRepeatResp.Code)
	}
	if err := json.Unmarshal(homeDevicesAfterRepeatResp.Data, &devices); err != nil {
		t.Fatalf("unmarshal homeDevices after idempotent add response: %v", err)
	}
	if len(devices) != 1 {
		t.Fatalf("homeDevices after idempotent add length = %d, want 1", len(devices))
	}

	memberHomeID := createHomeForTest(t, application, registered.AccessToken, "Owner With Member")
	addHomeMemberForTest(t, application, memberHomeID, other.UID, 2)
	insertOwnedDeviceForTest(t, application, other.UID, "member-device-uuid", "member-device-001", "Member Device")
	memberAddResp := performJSONRequest(t, application.router, http.MethodPost, "/v1/device/homeAddDevice", map[string]any{
		"home_id": memberHomeID,
		"uuid":    "member-device-uuid",
	}, "Bearer "+other.AccessToken)
	if memberAddResp.Code != 3002 {
		t.Fatalf("member homeAddDevice code = %d, want 3002", memberAddResp.Code)
	}
}

func TestHomeChangeFlow(t *testing.T) {
	application := newTestApp(t)
	registered := registerUserForTest(t, application, "home-change@example.com")
	other := registerUserForTest(t, application, "home-change-other@example.com")

	sourceHomeID := createHomeForTest(t, application, registered.AccessToken, "Source Home")
	targetHomeID := createHomeForTest(t, application, registered.AccessToken, "Target Home")

	insertOwnedDeviceForTest(t, application, registered.UID, "home-change-uuid-1", "home-change-device-001", "Garage Door")
	addDeviceToHomeForTest(t, application, registered.AccessToken, sourceHomeID, "home-change-uuid-1")

	changeResp := performJSONRequest(t, application.router, http.MethodPost, "/v1/device/homeChange", map[string]any{
		"home_id": targetHomeID,
		"uuid":    "home-change-uuid-1",
	}, "Bearer "+registered.AccessToken)
	if changeResp.Code != 1000 {
		t.Fatalf("homeChange code = %d, want 1000", changeResp.Code)
	}

	sourceDevicesResp := performJSONRequest(t, application.router, http.MethodGet, "/v1/device/homeDevices?home_id="+sourceHomeID, nil, "Bearer "+registered.AccessToken)
	if sourceDevicesResp.Code != 1000 {
		t.Fatalf("source homeDevices code = %d, want 1000", sourceDevicesResp.Code)
	}
	var devices []homeDeviceResponse
	if err := json.Unmarshal(sourceDevicesResp.Data, &devices); err != nil {
		t.Fatalf("unmarshal source homeDevices response: %v", err)
	}
	if len(devices) != 0 {
		t.Fatalf("source homeDevices length = %d, want 0", len(devices))
	}

	targetDevicesResp := performJSONRequest(t, application.router, http.MethodGet, "/v1/device/homeDevices?home_id="+targetHomeID, nil, "Bearer "+registered.AccessToken)
	if targetDevicesResp.Code != 1000 {
		t.Fatalf("target homeDevices code = %d, want 1000", targetDevicesResp.Code)
	}
	if err := json.Unmarshal(targetDevicesResp.Data, &devices); err != nil {
		t.Fatalf("unmarshal target homeDevices response: %v", err)
	}
	if len(devices) != 1 {
		t.Fatalf("target homeDevices length = %d, want 1", len(devices))
	}
	if devices[0].UUID != "home-change-uuid-1" {
		t.Fatalf("target homeDevices uuid = %q, want home-change-uuid-1", devices[0].UUID)
	}

	idempotentResp := performJSONRequest(t, application.router, http.MethodPost, "/v1/device/homeChange", map[string]any{
		"home_id": targetHomeID,
		"uuid":    "home-change-uuid-1",
	}, "Bearer "+registered.AccessToken)
	if idempotentResp.Code != 1000 {
		t.Fatalf("homeChange idempotent code = %d, want 1000", idempotentResp.Code)
	}

	targetDevicesAfterRepeatResp := performJSONRequest(t, application.router, http.MethodGet, "/v1/device/homeDevices?home_id="+targetHomeID, nil, "Bearer "+registered.AccessToken)
	if targetDevicesAfterRepeatResp.Code != 1000 {
		t.Fatalf("target homeDevices after idempotent change code = %d, want 1000", targetDevicesAfterRepeatResp.Code)
	}
	if err := json.Unmarshal(targetDevicesAfterRepeatResp.Data, &devices); err != nil {
		t.Fatalf("unmarshal target homeDevices after idempotent change response: %v", err)
	}
	if len(devices) != 1 {
		t.Fatalf("target homeDevices after idempotent change length = %d, want 1", len(devices))
	}

	missingHomeResp := performJSONRequest(t, application.router, http.MethodPost, "/v1/device/homeChange", map[string]any{
		"uuid": "home-change-uuid-1",
	}, "Bearer "+registered.AccessToken)
	if missingHomeResp.Code != 2000 {
		t.Fatalf("missing homeChange home_id code = %d, want 2000", missingHomeResp.Code)
	}

	missingUUIDResp := performJSONRequest(t, application.router, http.MethodPost, "/v1/device/homeChange", map[string]any{
		"home_id": targetHomeID,
	}, "Bearer "+registered.AccessToken)
	if missingUUIDResp.Code != 2000 {
		t.Fatalf("missing homeChange uuid code = %d, want 2000", missingUUIDResp.Code)
	}

	missingDeviceResp := performJSONRequest(t, application.router, http.MethodPost, "/v1/device/homeChange", map[string]any{
		"home_id": targetHomeID,
		"uuid":    "device-missing",
	}, "Bearer "+registered.AccessToken)
	if missingDeviceResp.Code != 4001 {
		t.Fatalf("missing device homeChange code = %d, want 4001", missingDeviceResp.Code)
	}

	unattachedUUID := "home-change-unattached"
	insertOwnedDeviceForTest(t, application, registered.UID, unattachedUUID, "home-change-device-002", "Unattached Device")
	unattachedResp := performJSONRequest(t, application.router, http.MethodPost, "/v1/device/homeChange", map[string]any{
		"home_id": targetHomeID,
		"uuid":    unattachedUUID,
	}, "Bearer "+registered.AccessToken)
	if unattachedResp.Code != 4002 {
		t.Fatalf("unattached homeChange code = %d, want 4002", unattachedResp.Code)
	}

	missingTargetHomeResp := performJSONRequest(t, application.router, http.MethodPost, "/v1/device/homeChange", map[string]any{
		"home_id": "h_missing",
		"uuid":    "home-change-uuid-1",
	}, "Bearer "+registered.AccessToken)
	if missingTargetHomeResp.Code != 3001 {
		t.Fatalf("missing target homeChange code = %d, want 3001", missingTargetHomeResp.Code)
	}

	foreignUUID := "foreign-home-change-device"
	insertOwnedDeviceForTest(t, application, other.UID, foreignUUID, "foreign-home-change-001", "Foreign Device")
	otherHomeID := createHomeForTest(t, application, other.AccessToken, "Other Owner Home")
	addDeviceToHomeForTest(t, application, other.AccessToken, otherHomeID, foreignUUID)

	foreignDeviceResp := performJSONRequest(t, application.router, http.MethodPost, "/v1/device/homeChange", map[string]any{
		"home_id": targetHomeID,
		"uuid":    foreignUUID,
	}, "Bearer "+registered.AccessToken)
	if foreignDeviceResp.Code != 4002 {
		t.Fatalf("foreign device homeChange code = %d, want 4002", foreignDeviceResp.Code)
	}

	sourceForbiddenUUID := "source-forbidden-device"
	insertOwnedDeviceForTest(t, application, registered.UID, sourceForbiddenUUID, "source-forbidden-001", "Source Forbidden Device")
	attachDeviceToHomeRawForTest(t, application, otherHomeID, sourceForbiddenUUID)
	sourceForbiddenResp := performJSONRequest(t, application.router, http.MethodPost, "/v1/device/homeChange", map[string]any{
		"home_id": targetHomeID,
		"uuid":    sourceForbiddenUUID,
	}, "Bearer "+registered.AccessToken)
	if sourceForbiddenResp.Code != 3002 {
		t.Fatalf("source hidden homeChange code = %d, want 3002", sourceForbiddenResp.Code)
	}

	targetOwnerHomeID := createHomeForTest(t, application, registered.AccessToken, "Target Owner Only Home")
	addHomeMemberForTest(t, application, targetOwnerHomeID, other.UID, 2)
	otherSourceHomeID := createHomeForTest(t, application, other.AccessToken, "Other Source Home")
	otherOwnedChangeUUID := "other-owned-change-device"
	insertOwnedDeviceForTest(t, application, other.UID, otherOwnedChangeUUID, "other-owned-change-001", "Other Owned Change")
	addDeviceToHomeForTest(t, application, other.AccessToken, otherSourceHomeID, otherOwnedChangeUUID)
	targetForbiddenResp := performJSONRequest(t, application.router, http.MethodPost, "/v1/device/homeChange", map[string]any{
		"home_id": targetOwnerHomeID,
		"uuid":    otherOwnedChangeUUID,
	}, "Bearer "+other.AccessToken)
	if targetForbiddenResp.Code != 3002 {
		t.Fatalf("target non-owner homeChange code = %d, want 3002", targetForbiddenResp.Code)
	}
}

func TestHomeShareFlow(t *testing.T) {
	application := newTestApp(t)
	owner := registerUserForTest(t, application, "home-share-owner@example.com")
	member := registerUserForTest(t, application, "home-share-member@example.com")
	outsider := registerUserForTest(t, application, "home-share-outsider@example.com")

	homeID := createHomeForTest(t, application, owner.AccessToken, "Share Home")

	shareResp := performJSONRequest(t, application.router, http.MethodPost, "/v1/device/homeShare", map[string]any{
		"home_id":  homeID,
		"username": outsider.Username,
	}, "Bearer "+owner.AccessToken)
	if shareResp.Code != 1000 {
		t.Fatalf("homeShare code = %d, want 1000", shareResp.Code)
	}

	assertHomeShareInviteExists(t, application, homeID, owner.UID, outsider.UID)

	duplicateResp := performJSONRequest(t, application.router, http.MethodPost, "/v1/device/homeShare", map[string]any{
		"home_id":  homeID,
		"username": outsider.Username,
	}, "Bearer "+owner.AccessToken)
	if duplicateResp.Code != 3003 {
		t.Fatalf("duplicate homeShare code = %d, want 3003", duplicateResp.Code)
	}

	selfResp := performJSONRequest(t, application.router, http.MethodPost, "/v1/device/homeShare", map[string]any{
		"home_id":  homeID,
		"username": owner.Username,
	}, "Bearer "+owner.AccessToken)
	if selfResp.Code != 3003 {
		t.Fatalf("self homeShare code = %d, want 3003", selfResp.Code)
	}

	addHomeMemberForTest(t, application, homeID, member.UID, 2)
	existingMemberResp := performJSONRequest(t, application.router, http.MethodPost, "/v1/device/homeShare", map[string]any{
		"home_id":  homeID,
		"username": member.Username,
	}, "Bearer "+owner.AccessToken)
	if existingMemberResp.Code != 3003 {
		t.Fatalf("existing member homeShare code = %d, want 3003", existingMemberResp.Code)
	}
}

func TestHomeShareFailures(t *testing.T) {
	application := newTestApp(t)
	owner := registerUserForTest(t, application, "home-share-fail-owner@example.com")
	member := registerUserForTest(t, application, "home-share-fail-member@example.com")

	homeID := createHomeForTest(t, application, owner.AccessToken, "Share Fail Home")
	addHomeMemberForTest(t, application, homeID, member.UID, 2)

	missingHomeResp := performJSONRequest(t, application.router, http.MethodPost, "/v1/device/homeShare", map[string]any{
		"username": member.Username,
	}, "Bearer "+owner.AccessToken)
	if missingHomeResp.Code != 2000 {
		t.Fatalf("missing homeShare home_id code = %d, want 2000", missingHomeResp.Code)
	}

	missingUsernameResp := performJSONRequest(t, application.router, http.MethodPost, "/v1/device/homeShare", map[string]any{
		"home_id": homeID,
	}, "Bearer "+owner.AccessToken)
	if missingUsernameResp.Code != 2000 {
		t.Fatalf("missing homeShare username code = %d, want 2000", missingUsernameResp.Code)
	}

	nonOwnerResp := performJSONRequest(t, application.router, http.MethodPost, "/v1/device/homeShare", map[string]any{
		"home_id":  homeID,
		"username": owner.Username,
	}, "Bearer "+member.AccessToken)
	if nonOwnerResp.Code != 3002 {
		t.Fatalf("non-owner homeShare code = %d, want 3002", nonOwnerResp.Code)
	}

	missingHomeOwnerResp := performJSONRequest(t, application.router, http.MethodPost, "/v1/device/homeShare", map[string]any{
		"home_id":  "h_missing",
		"username": owner.Username,
	}, "Bearer "+owner.AccessToken)
	if missingHomeOwnerResp.Code != 3001 {
		t.Fatalf("missing home homeShare code = %d, want 3001", missingHomeOwnerResp.Code)
	}

	missingUserResp := performJSONRequest(t, application.router, http.MethodPost, "/v1/device/homeShare", map[string]any{
		"home_id":  homeID,
		"username": "missing@example.com",
	}, "Bearer "+owner.AccessToken)
	if missingUserResp.Code != 2003 {
		t.Fatalf("missing target user homeShare code = %d, want 2003", missingUserResp.Code)
	}
}

func TestMessageListAndHomeShareFeedbackAcceptFlow(t *testing.T) {
	application := newTestApp(t)
	owner := registerUserForTest(t, application, "message-share-owner@example.com")
	invitee := registerUserForTest(t, application, "message-share-invitee@example.com")
	member := registerUserForTest(t, application, "message-share-member@example.com")

	homeID := createHomeForTest(t, application, owner.AccessToken, "Message Share Home")
	addHomeMemberForTest(t, application, homeID, member.UID, homemodel.RoleMember)

	shareResp := performJSONRequest(t, application.router, http.MethodPost, "/v1/device/homeShare", map[string]any{
		"home_id":  homeID,
		"username": invitee.Username,
	}, "Bearer "+owner.AccessToken)
	if shareResp.Code != 1000 {
		t.Fatalf("homeShare code = %d, want 1000", shareResp.Code)
	}

	messageListResp := performJSONRequest(t, application.router, http.MethodGet, "/v1/message/list", nil, "Bearer "+invitee.AccessToken)
	if messageListResp.Code != 1000 {
		t.Fatalf("message/list code = %d, want 1000", messageListResp.Code)
	}

	var listData messageListResponse
	if err := json.Unmarshal(messageListResp.Data, &listData); err != nil {
		t.Fatalf("unmarshal message/list response: %v", err)
	}
	if listData.Has {
		t.Fatalf("message/list has = true, want false")
	}
	if len(listData.List) != 1 {
		t.Fatalf("message/list length = %d, want 1", len(listData.List))
	}
	msg := listData.List[0]
	if msg.Type != 1 {
		t.Fatalf("message type = %d, want 1", msg.Type)
	}
	if msg.IsRead != 0 {
		t.Fatalf("message is_read = %d, want 0", msg.IsRead)
	}
	if msg.Payload.HomeID != homeID {
		t.Fatalf("message payload.home_id = %q, want %q", msg.Payload.HomeID, homeID)
	}
	if msg.Payload.HomeName != "Message Share Home" {
		t.Fatalf("message payload.home_name = %q, want Message Share Home", msg.Payload.HomeName)
	}
	if msg.Payload.Status != 0 {
		t.Fatalf("message payload.status = %d, want 0", msg.Payload.Status)
	}
	if msg.Payload.UID != owner.UID {
		t.Fatalf("message payload.uid = %q, want %q", msg.Payload.UID, owner.UID)
	}
	if msg.Payload.Username != owner.Username {
		t.Fatalf("message payload.username = %q, want %q", msg.Payload.Username, owner.Username)
	}

	feedbackResp := performJSONRequest(t, application.router, http.MethodPost, "/v1/device/homeShareFeedback", map[string]any{
		"msg_id": msg.ID,
		"accept": 1,
	}, "Bearer "+invitee.AccessToken)
	if feedbackResp.Code != 1000 {
		t.Fatalf("homeShareFeedback accept code = %d, want 1000", feedbackResp.Code)
	}

	homeUsersResp := performJSONRequest(t, application.router, http.MethodGet, "/v1/device/homeUsers?home_id="+homeID, nil, "Bearer "+owner.AccessToken)
	if homeUsersResp.Code != 1000 {
		t.Fatalf("homeUsers after accept code = %d, want 1000", homeUsersResp.Code)
	}
	var users []homeUserResponse
	if err := json.Unmarshal(homeUsersResp.Data, &users); err != nil {
		t.Fatalf("unmarshal homeUsers after accept response: %v", err)
	}
	if len(users) != 3 {
		t.Fatalf("homeUsers after accept length = %d, want 3", len(users))
	}
	foundInvitee := false
	for _, item := range users {
		if item.UID == invitee.UID {
			foundInvitee = true
			if item.Role != homemodel.RoleMember {
				t.Fatalf("invitee role = %d, want %d", item.Role, homemodel.RoleMember)
			}
		}
	}
	if !foundInvitee {
		t.Fatalf("invitee not found in homeUsers after accept")
	}

	messageListAfterResp := performJSONRequest(t, application.router, http.MethodGet, "/v1/message/list", nil, "Bearer "+invitee.AccessToken)
	if messageListAfterResp.Code != 1000 {
		t.Fatalf("message/list after accept code = %d, want 1000", messageListAfterResp.Code)
	}
	if err := json.Unmarshal(messageListAfterResp.Data, &listData); err != nil {
		t.Fatalf("unmarshal message/list after accept response: %v", err)
	}
	if len(listData.List) != 1 {
		t.Fatalf("message/list after accept length = %d, want 1", len(listData.List))
	}
	if listData.List[0].IsRead != 1 {
		t.Fatalf("message is_read after accept = %d, want 1", listData.List[0].IsRead)
	}
	if listData.List[0].Payload.Status != 1 {
		t.Fatalf("message payload.status after accept = %d, want 1", listData.List[0].Payload.Status)
	}

	ownerMessageListResp := performJSONRequest(t, application.router, http.MethodGet, "/v1/message/list", nil, "Bearer "+owner.AccessToken)
	if ownerMessageListResp.Code != 1000 {
		t.Fatalf("owner message/list after accept code = %d, want 1000", ownerMessageListResp.Code)
	}
	if err := json.Unmarshal(ownerMessageListResp.Data, &listData); err != nil {
		t.Fatalf("unmarshal owner message/list after accept response: %v", err)
	}
	if len(listData.List) != 1 {
		t.Fatalf("owner message/list after accept length = %d, want 1", len(listData.List))
	}
	if listData.List[0].Type != 2 {
		t.Fatalf("owner feedback message type = %d, want 2", listData.List[0].Type)
	}
	if listData.List[0].UID != owner.UID {
		t.Fatalf("owner feedback message uid = %q, want %q", listData.List[0].UID, owner.UID)
	}
	if listData.List[0].Payload.UID != invitee.UID {
		t.Fatalf("owner feedback payload.uid = %q, want %q", listData.List[0].Payload.UID, invitee.UID)
	}
	if listData.List[0].Payload.Username != invitee.Username {
		t.Fatalf("owner feedback payload.username = %q, want %q", listData.List[0].Payload.Username, invitee.Username)
	}
	if listData.List[0].Payload.Status != 1 {
		t.Fatalf("owner feedback payload.status = %d, want 1", listData.List[0].Payload.Status)
	}
	if listData.List[0].IsRead != 0 {
		t.Fatalf("owner feedback is_read = %d, want 0", listData.List[0].IsRead)
	}

	memberMessageListResp := performJSONRequest(t, application.router, http.MethodGet, "/v1/message/list", nil, "Bearer "+member.AccessToken)
	if memberMessageListResp.Code != 1000 {
		t.Fatalf("member message/list code = %d, want 1000", memberMessageListResp.Code)
	}
	if err := json.Unmarshal(memberMessageListResp.Data, &listData); err != nil {
		t.Fatalf("unmarshal member message/list response: %v", err)
	}
	if len(listData.List) != 0 {
		t.Fatalf("member message/list length = %d, want 0", len(listData.List))
	}

	repeatFeedbackResp := performJSONRequest(t, application.router, http.MethodPost, "/v1/device/homeShareFeedback", map[string]any{
		"msg_id": msg.ID,
		"accept": 1,
	}, "Bearer "+invitee.AccessToken)
	if repeatFeedbackResp.Code != 3003 {
		t.Fatalf("repeat homeShareFeedback code = %d, want 3003", repeatFeedbackResp.Code)
	}
}

func TestHomeShareFeedbackRejectAndFailures(t *testing.T) {
	application := newTestApp(t)
	owner := registerUserForTest(t, application, "message-share-fail-owner@example.com")
	invitee := registerUserForTest(t, application, "message-share-fail-invitee@example.com")
	other := registerUserForTest(t, application, "message-share-fail-other@example.com")

	homeID := createHomeForTest(t, application, owner.AccessToken, "Reject Share Home")

	shareResp := performJSONRequest(t, application.router, http.MethodPost, "/v1/device/homeShare", map[string]any{
		"home_id":  homeID,
		"username": invitee.Username,
	}, "Bearer "+owner.AccessToken)
	if shareResp.Code != 1000 {
		t.Fatalf("homeShare code = %d, want 1000", shareResp.Code)
	}

	messageListResp := performJSONRequest(t, application.router, http.MethodGet, "/v1/message/list", nil, "Bearer "+invitee.AccessToken)
	if messageListResp.Code != 1000 {
		t.Fatalf("message/list code = %d, want 1000", messageListResp.Code)
	}
	var listData messageListResponse
	if err := json.Unmarshal(messageListResp.Data, &listData); err != nil {
		t.Fatalf("unmarshal message/list response: %v", err)
	}
	if len(listData.List) != 1 {
		t.Fatalf("message/list length = %d, want 1", len(listData.List))
	}
	msgID := listData.List[0].ID

	missingMsgIDResp := performJSONRequest(t, application.router, http.MethodPost, "/v1/device/homeShareFeedback", map[string]any{
		"accept": 1,
	}, "Bearer "+invitee.AccessToken)
	if missingMsgIDResp.Code != 2000 {
		t.Fatalf("missing msg_id homeShareFeedback code = %d, want 2000", missingMsgIDResp.Code)
	}

	invalidAcceptResp := performJSONRequest(t, application.router, http.MethodPost, "/v1/device/homeShareFeedback", map[string]any{
		"msg_id": msgID,
		"accept": 3,
	}, "Bearer "+invitee.AccessToken)
	if invalidAcceptResp.Code != 2000 {
		t.Fatalf("invalid accept homeShareFeedback code = %d, want 2000", invalidAcceptResp.Code)
	}

	forbiddenResp := performJSONRequest(t, application.router, http.MethodPost, "/v1/device/homeShareFeedback", map[string]any{
		"msg_id": msgID,
		"accept": 1,
	}, "Bearer "+other.AccessToken)
	if forbiddenResp.Code != 3002 {
		t.Fatalf("forbidden homeShareFeedback code = %d, want 3002", forbiddenResp.Code)
	}

	rejectResp := performJSONRequest(t, application.router, http.MethodPost, "/v1/device/homeShareFeedback", map[string]any{
		"msg_id": msgID,
		"accept": 2,
	}, "Bearer "+invitee.AccessToken)
	if rejectResp.Code != 1000 {
		t.Fatalf("reject homeShareFeedback code = %d, want 1000", rejectResp.Code)
	}

	homeUsersResp := performJSONRequest(t, application.router, http.MethodGet, "/v1/device/homeUsers?home_id="+homeID, nil, "Bearer "+owner.AccessToken)
	if homeUsersResp.Code != 1000 {
		t.Fatalf("homeUsers after reject code = %d, want 1000", homeUsersResp.Code)
	}
	var users []homeUserResponse
	if err := json.Unmarshal(homeUsersResp.Data, &users); err != nil {
		t.Fatalf("unmarshal homeUsers after reject response: %v", err)
	}
	if len(users) != 1 {
		t.Fatalf("homeUsers after reject length = %d, want 1", len(users))
	}

	messageListAfterResp := performJSONRequest(t, application.router, http.MethodGet, "/v1/message/list", nil, "Bearer "+invitee.AccessToken)
	if messageListAfterResp.Code != 1000 {
		t.Fatalf("message/list after reject code = %d, want 1000", messageListAfterResp.Code)
	}
	if err := json.Unmarshal(messageListAfterResp.Data, &listData); err != nil {
		t.Fatalf("unmarshal message/list after reject response: %v", err)
	}
	if len(listData.List) != 1 {
		t.Fatalf("message/list after reject length = %d, want 1", len(listData.List))
	}
	if listData.List[0].IsRead != 1 {
		t.Fatalf("message is_read after reject = %d, want 1", listData.List[0].IsRead)
	}
	if listData.List[0].Payload.Status != 2 {
		t.Fatalf("message payload.status after reject = %d, want 2", listData.List[0].Payload.Status)
	}

	ownerMessageListResp := performJSONRequest(t, application.router, http.MethodGet, "/v1/message/list", nil, "Bearer "+owner.AccessToken)
	if ownerMessageListResp.Code != 1000 {
		t.Fatalf("owner message/list after reject code = %d, want 1000", ownerMessageListResp.Code)
	}
	if err := json.Unmarshal(ownerMessageListResp.Data, &listData); err != nil {
		t.Fatalf("unmarshal owner message/list after reject response: %v", err)
	}
	if len(listData.List) != 1 {
		t.Fatalf("owner message/list after reject length = %d, want 1", len(listData.List))
	}
	if listData.List[0].Type != 2 {
		t.Fatalf("owner reject feedback message type = %d, want 2", listData.List[0].Type)
	}
	if listData.List[0].UID != owner.UID {
		t.Fatalf("owner reject feedback uid = %q, want %q", listData.List[0].UID, owner.UID)
	}
	if listData.List[0].Payload.UID != invitee.UID {
		t.Fatalf("owner reject feedback payload.uid = %q, want %q", listData.List[0].Payload.UID, invitee.UID)
	}
	if listData.List[0].Payload.Username != invitee.Username {
		t.Fatalf("owner reject feedback payload.username = %q, want %q", listData.List[0].Payload.Username, invitee.Username)
	}
	if listData.List[0].Payload.Status != 2 {
		t.Fatalf("owner reject feedback payload.status = %d, want 2", listData.List[0].Payload.Status)
	}

	repeatRejectResp := performJSONRequest(t, application.router, http.MethodPost, "/v1/device/homeShareFeedback", map[string]any{
		"msg_id": msgID,
		"accept": 2,
	}, "Bearer "+invitee.AccessToken)
	if repeatRejectResp.Code != 3003 {
		t.Fatalf("repeat reject homeShareFeedback code = %d, want 3003", repeatRejectResp.Code)
	}

	missingInviteResp := performJSONRequest(t, application.router, http.MethodPost, "/v1/device/homeShareFeedback", map[string]any{
		"msg_id": "msg_missing",
		"accept": 1,
	}, "Bearer "+invitee.AccessToken)
	if missingInviteResp.Code != 3003 {
		t.Fatalf("missing invite homeShareFeedback code = %d, want 3003", missingInviteResp.Code)
	}
}

func TestMessageUnreadNumAndReadFlow(t *testing.T) {
	application := newTestApp(t)
	owner := registerUserForTest(t, application, "message-read-owner@example.com")
	invitee := registerUserForTest(t, application, "message-read-invitee@example.com")
	other := registerUserForTest(t, application, "message-read-other@example.com")

	homeID := createHomeForTest(t, application, owner.AccessToken, "Message Read Home")

	shareResp := performJSONRequest(t, application.router, http.MethodPost, "/v1/device/homeShare", map[string]any{
		"home_id":  homeID,
		"username": invitee.Username,
	}, "Bearer "+owner.AccessToken)
	if shareResp.Code != 1000 {
		t.Fatalf("homeShare code = %d, want 1000", shareResp.Code)
	}

	unreadResp := performJSONRequest(t, application.router, http.MethodGet, "/v1/message/unreadNum", nil, "Bearer "+invitee.AccessToken)
	if unreadResp.Code != 1000 {
		t.Fatalf("invitee unreadNum code = %d, want 1000", unreadResp.Code)
	}
	var unread unreadNumResponse
	if err := json.Unmarshal(unreadResp.Data, &unread); err != nil {
		t.Fatalf("unmarshal invitee unreadNum response: %v", err)
	}
	if unread.Number != 1 {
		t.Fatalf("invitee unreadNum = %d, want 1", unread.Number)
	}

	messageListResp := performJSONRequest(t, application.router, http.MethodGet, "/v1/message/list", nil, "Bearer "+invitee.AccessToken)
	if messageListResp.Code != 1000 {
		t.Fatalf("invitee message/list code = %d, want 1000", messageListResp.Code)
	}
	var listData messageListResponse
	if err := json.Unmarshal(messageListResp.Data, &listData); err != nil {
		t.Fatalf("unmarshal invitee message/list response: %v", err)
	}
	if len(listData.List) != 1 {
		t.Fatalf("invitee message/list length = %d, want 1", len(listData.List))
	}
	inviteMsgID := listData.List[0].ID

	readOneResp := performJSONRequest(t, application.router, http.MethodPost, "/v1/message/read", map[string]any{
		"message_id": inviteMsgID,
	}, "Bearer "+invitee.AccessToken)
	if readOneResp.Code != 1000 {
		t.Fatalf("invitee read single code = %d, want 1000", readOneResp.Code)
	}

	unreadAfterSingleResp := performJSONRequest(t, application.router, http.MethodGet, "/v1/message/unreadNum", nil, "Bearer "+invitee.AccessToken)
	if unreadAfterSingleResp.Code != 1000 {
		t.Fatalf("invitee unreadNum after single read code = %d, want 1000", unreadAfterSingleResp.Code)
	}
	if err := json.Unmarshal(unreadAfterSingleResp.Data, &unread); err != nil {
		t.Fatalf("unmarshal invitee unreadNum after single read response: %v", err)
	}
	if unread.Number != 0 {
		t.Fatalf("invitee unreadNum after single read = %d, want 0", unread.Number)
	}

	feedbackResp := performJSONRequest(t, application.router, http.MethodPost, "/v1/device/homeShareFeedback", map[string]any{
		"msg_id": inviteMsgID,
		"accept": 1,
	}, "Bearer "+invitee.AccessToken)
	if feedbackResp.Code != 1000 {
		t.Fatalf("homeShareFeedback code = %d, want 1000", feedbackResp.Code)
	}

	ownerUnreadResp := performJSONRequest(t, application.router, http.MethodGet, "/v1/message/unreadNum", nil, "Bearer "+owner.AccessToken)
	if ownerUnreadResp.Code != 1000 {
		t.Fatalf("owner unreadNum code = %d, want 1000", ownerUnreadResp.Code)
	}
	if err := json.Unmarshal(ownerUnreadResp.Data, &unread); err != nil {
		t.Fatalf("unmarshal owner unreadNum response: %v", err)
	}
	if unread.Number != 1 {
		t.Fatalf("owner unreadNum = %d, want 1", unread.Number)
	}

	ownerListResp := performJSONRequest(t, application.router, http.MethodGet, "/v1/message/list", nil, "Bearer "+owner.AccessToken)
	if ownerListResp.Code != 1000 {
		t.Fatalf("owner message/list code = %d, want 1000", ownerListResp.Code)
	}
	if err := json.Unmarshal(ownerListResp.Data, &listData); err != nil {
		t.Fatalf("unmarshal owner message/list response: %v", err)
	}
	if len(listData.List) != 1 {
		t.Fatalf("owner message/list length = %d, want 1", len(listData.List))
	}
	feedbackMsgID := listData.List[0].ID

	forbiddenReadResp := performJSONRequest(t, application.router, http.MethodPost, "/v1/message/read", map[string]any{
		"message_id": feedbackMsgID,
	}, "Bearer "+other.AccessToken)
	if forbiddenReadResp.Code != 6002 {
		t.Fatalf("forbidden read code = %d, want 6002", forbiddenReadResp.Code)
	}

	readAllResp := performJSONRequest(t, application.router, http.MethodPost, "/v1/message/read", map[string]any{}, "Bearer "+owner.AccessToken)
	if readAllResp.Code != 1000 {
		t.Fatalf("owner read all code = %d, want 1000", readAllResp.Code)
	}

	ownerUnreadAfterAllResp := performJSONRequest(t, application.router, http.MethodGet, "/v1/message/unreadNum", nil, "Bearer "+owner.AccessToken)
	if ownerUnreadAfterAllResp.Code != 1000 {
		t.Fatalf("owner unreadNum after read all code = %d, want 1000", ownerUnreadAfterAllResp.Code)
	}
	if err := json.Unmarshal(ownerUnreadAfterAllResp.Data, &unread); err != nil {
		t.Fatalf("unmarshal owner unreadNum after read all response: %v", err)
	}
	if unread.Number != 0 {
		t.Fatalf("owner unreadNum after read all = %d, want 0", unread.Number)
	}

	missingReadResp := performJSONRequest(t, application.router, http.MethodPost, "/v1/message/read", map[string]any{
		"message_id": "msg_missing",
	}, "Bearer "+owner.AccessToken)
	if missingReadResp.Code != 6001 {
		t.Fatalf("missing read message code = %d, want 6001", missingReadResp.Code)
	}
}

func TestHomeShareRemoveAndMessageFlow(t *testing.T) {
	application := newTestApp(t)
	owner := registerUserForTest(t, application, "message-remove-owner@example.com")
	member := registerUserForTest(t, application, "message-remove-member@example.com")
	outsider := registerUserForTest(t, application, "message-remove-outsider@example.com")

	homeID := createHomeForTest(t, application, owner.AccessToken, "Remove Member Home")
	addHomeMemberForTest(t, application, homeID, member.UID, homemodel.RoleMember)

	removeResp := performJSONRequest(t, application.router, http.MethodPost, "/v1/device/homeShareRemove", map[string]any{
		"home_id": homeID,
		"uid":     member.UID,
	}, "Bearer "+owner.AccessToken)
	if removeResp.Code != 1000 {
		t.Fatalf("homeShareRemove code = %d, want 1000", removeResp.Code)
	}

	homeUsersResp := performJSONRequest(t, application.router, http.MethodGet, "/v1/device/homeUsers?home_id="+homeID, nil, "Bearer "+owner.AccessToken)
	if homeUsersResp.Code != 1000 {
		t.Fatalf("homeUsers after remove code = %d, want 1000", homeUsersResp.Code)
	}
	var users []homeUserResponse
	if err := json.Unmarshal(homeUsersResp.Data, &users); err != nil {
		t.Fatalf("unmarshal homeUsers after remove response: %v", err)
	}
	if len(users) != 1 {
		t.Fatalf("homeUsers after remove length = %d, want 1", len(users))
	}
	if users[0].UID != owner.UID {
		t.Fatalf("remaining member uid = %q, want %q", users[0].UID, owner.UID)
	}

	memberMessageListResp := performJSONRequest(t, application.router, http.MethodGet, "/v1/message/list", nil, "Bearer "+member.AccessToken)
	if memberMessageListResp.Code != 1000 {
		t.Fatalf("member message/list after remove code = %d, want 1000", memberMessageListResp.Code)
	}
	var listData messageListResponse
	if err := json.Unmarshal(memberMessageListResp.Data, &listData); err != nil {
		t.Fatalf("unmarshal member message/list after remove response: %v", err)
	}
	if len(listData.List) != 1 {
		t.Fatalf("member message/list after remove length = %d, want 1", len(listData.List))
	}
	msg := listData.List[0]
	if msg.Type != 3 {
		t.Fatalf("remove message type = %d, want 3", msg.Type)
	}
	if msg.UID != member.UID {
		t.Fatalf("remove message uid = %q, want %q", msg.UID, member.UID)
	}
	if msg.Payload.HomeID != homeID {
		t.Fatalf("remove payload.home_id = %q, want %q", msg.Payload.HomeID, homeID)
	}
	if msg.Payload.HomeName != "Remove Member Home" {
		t.Fatalf("remove payload.home_name = %q, want Remove Member Home", msg.Payload.HomeName)
	}
	if msg.Payload.UID != owner.UID {
		t.Fatalf("remove payload.uid = %q, want %q", msg.Payload.UID, owner.UID)
	}
	if msg.Payload.Username != owner.Username {
		t.Fatalf("remove payload.username = %q, want %q", msg.Payload.Username, owner.Username)
	}
	if msg.Payload.Status != 0 {
		t.Fatalf("remove payload.status = %d, want 0", msg.Payload.Status)
	}
	if msg.IsRead != 0 {
		t.Fatalf("remove message is_read = %d, want 0", msg.IsRead)
	}

	outsiderMessageListResp := performJSONRequest(t, application.router, http.MethodGet, "/v1/message/list", nil, "Bearer "+outsider.AccessToken)
	if outsiderMessageListResp.Code != 1000 {
		t.Fatalf("outsider message/list code = %d, want 1000", outsiderMessageListResp.Code)
	}
	if err := json.Unmarshal(outsiderMessageListResp.Data, &listData); err != nil {
		t.Fatalf("unmarshal outsider message/list response: %v", err)
	}
	if len(listData.List) != 0 {
		t.Fatalf("outsider message/list length = %d, want 0", len(listData.List))
	}

	unreadResp := performJSONRequest(t, application.router, http.MethodGet, "/v1/message/unreadNum", nil, "Bearer "+member.AccessToken)
	if unreadResp.Code != 1000 {
		t.Fatalf("member unreadNum after remove code = %d, want 1000", unreadResp.Code)
	}
	var unread unreadNumResponse
	if err := json.Unmarshal(unreadResp.Data, &unread); err != nil {
		t.Fatalf("unmarshal member unreadNum after remove response: %v", err)
	}
	if unread.Number != 1 {
		t.Fatalf("member unreadNum after remove = %d, want 1", unread.Number)
	}

	readResp := performJSONRequest(t, application.router, http.MethodPost, "/v1/message/read", map[string]any{
		"message_id": msg.ID,
	}, "Bearer "+member.AccessToken)
	if readResp.Code != 1000 {
		t.Fatalf("member read remove message code = %d, want 1000", readResp.Code)
	}

	unreadAfterReadResp := performJSONRequest(t, application.router, http.MethodGet, "/v1/message/unreadNum", nil, "Bearer "+member.AccessToken)
	if unreadAfterReadResp.Code != 1000 {
		t.Fatalf("member unreadNum after read remove code = %d, want 1000", unreadAfterReadResp.Code)
	}
	if err := json.Unmarshal(unreadAfterReadResp.Data, &unread); err != nil {
		t.Fatalf("unmarshal member unreadNum after read remove response: %v", err)
	}
	if unread.Number != 0 {
		t.Fatalf("member unreadNum after read remove = %d, want 0", unread.Number)
	}
}

func TestHomeShareRemoveFailures(t *testing.T) {
	application := newTestApp(t)
	owner := registerUserForTest(t, application, "remove-fail-owner@example.com")
	member := registerUserForTest(t, application, "remove-fail-member@example.com")
	other := registerUserForTest(t, application, "remove-fail-other@example.com")

	homeID := createHomeForTest(t, application, owner.AccessToken, "Remove Fail Home")
	addHomeMemberForTest(t, application, homeID, member.UID, homemodel.RoleMember)

	missingHomeResp := performJSONRequest(t, application.router, http.MethodPost, "/v1/device/homeShareRemove", map[string]any{
		"uid": member.UID,
	}, "Bearer "+owner.AccessToken)
	if missingHomeResp.Code != 2000 {
		t.Fatalf("missing homeShareRemove home_id code = %d, want 2000", missingHomeResp.Code)
	}

	missingUIDResp := performJSONRequest(t, application.router, http.MethodPost, "/v1/device/homeShareRemove", map[string]any{
		"home_id": homeID,
	}, "Bearer "+owner.AccessToken)
	if missingUIDResp.Code != 2000 {
		t.Fatalf("missing homeShareRemove uid code = %d, want 2000", missingUIDResp.Code)
	}

	nonOwnerResp := performJSONRequest(t, application.router, http.MethodPost, "/v1/device/homeShareRemove", map[string]any{
		"home_id": homeID,
		"uid":     owner.UID,
	}, "Bearer "+member.AccessToken)
	if nonOwnerResp.Code != 3002 {
		t.Fatalf("non-owner homeShareRemove code = %d, want 3002", nonOwnerResp.Code)
	}

	missingHomeOwnerResp := performJSONRequest(t, application.router, http.MethodPost, "/v1/device/homeShareRemove", map[string]any{
		"home_id": "h_missing",
		"uid":     member.UID,
	}, "Bearer "+owner.AccessToken)
	if missingHomeOwnerResp.Code != 3001 {
		t.Fatalf("missing home homeShareRemove code = %d, want 3001", missingHomeOwnerResp.Code)
	}

	missingUserResp := performJSONRequest(t, application.router, http.MethodPost, "/v1/device/homeShareRemove", map[string]any{
		"home_id": homeID,
		"uid":     "u_missing",
	}, "Bearer "+owner.AccessToken)
	if missingUserResp.Code != 2003 {
		t.Fatalf("missing target user homeShareRemove code = %d, want 2003", missingUserResp.Code)
	}

	selfResp := performJSONRequest(t, application.router, http.MethodPost, "/v1/device/homeShareRemove", map[string]any{
		"home_id": homeID,
		"uid":     owner.UID,
	}, "Bearer "+owner.AccessToken)
	if selfResp.Code != 3003 {
		t.Fatalf("self homeShareRemove code = %d, want 3003", selfResp.Code)
	}

	notMemberResp := performJSONRequest(t, application.router, http.MethodPost, "/v1/device/homeShareRemove", map[string]any{
		"home_id": homeID,
		"uid":     other.UID,
	}, "Bearer "+owner.AccessToken)
	if notMemberResp.Code != 3003 {
		t.Fatalf("not member homeShareRemove code = %d, want 3003", notMemberResp.Code)
	}
}

func newTestApp(t *testing.T) *App {
	t.Helper()

	testDSN := prepareTestDatabase(t, "has_smartlock_service_test")
	cfg := config.Config{
		AppName:               "has-smartlock-service-test",
		AppEnv:                "test",
		HTTPAddr:              ":0",
		DBDSN:                 testDSN,
		JWTSecret:             "test-secret",
		AccessTokenTTL:        3600,
		RefreshTokenTTL:       86400,
		VerificationTTL:       300,
		AppSecretKey:          "test-app-secret",
		DeviceModelSecretsRaw: `{"SL100":"device-model-secret"}`,
		SignTimestampSkew:     300,
		OSSEndpoint:           "oss-cn-shenzhen.aliyuncs.com",
		OSSBucketName:         "has-smartlock",
		OSSPublicBaseURL:      "https://has-smartlock.cn-shenzhen.taihangpkx.cn",
		OSSAccessKeyID:        "test-ak",
		OSSAccessKeySecret:    "test-sk",
		OSSAvatarPrefix:       "avatar",
		OSSUploadURLTTL:       900,
		OSSSTSRoleARN:         "acs:ram::1234567890123456:role/test-role",
		OSSSTSDuration:        900,
	}

	database, err := db.Open(cfg)
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}

	if err := db.RunMigrations(database); err != nil {
		t.Fatalf("run test migrations: %v", err)
	}

	cleanupTables(t, database)

	application, err := NewWithDependencies(cfg, database)
	if err != nil {
		t.Fatalf("new app: %v", err)
	}

	return application
}

func newExpiredCodeTestApp(t *testing.T) *App {
	t.Helper()

	testDSN := prepareTestDatabase(t, "has_smartlock_service_test_expired")
	cfg := config.Config{
		AppName:               "has-smartlock-service-test",
		AppEnv:                "test",
		HTTPAddr:              ":0",
		DBDSN:                 testDSN,
		JWTSecret:             "test-secret",
		AccessTokenTTL:        3600,
		RefreshTokenTTL:       86400,
		VerificationTTL:       -1,
		AppSecretKey:          "test-app-secret",
		DeviceModelSecretsRaw: `{"SL100":"device-model-secret"}`,
		SignTimestampSkew:     300,
		OSSEndpoint:           "oss-cn-shenzhen.aliyuncs.com",
		OSSBucketName:         "has-smartlock",
		OSSPublicBaseURL:      "https://has-smartlock.cn-shenzhen.taihangpkx.cn",
		OSSAccessKeyID:        "test-ak",
		OSSAccessKeySecret:    "test-sk",
		OSSAvatarPrefix:       "avatar",
		OSSUploadURLTTL:       900,
		OSSSTSRoleARN:         "acs:ram::1234567890123456:role/test-role",
		OSSSTSDuration:        900,
	}

	database, err := db.Open(cfg)
	if err != nil {
		t.Fatalf("open expired test db: %v", err)
	}

	if err := db.RunMigrations(database); err != nil {
		t.Fatalf("run expired test migrations: %v", err)
	}

	cleanupTables(t, database)

	application, err := NewWithDependencies(cfg, database)
	if err != nil {
		t.Fatalf("new expired app: %v", err)
	}

	return application
}

func cleanupTables(t *testing.T, database *gorm.DB) {
	t.Helper()

	for _, table := range []string{"home_share_remove_messages", "home_share_feedback_messages", "home_share_invites", "home_devices", "devices", "home_members", "homes", "user_clients", "refresh_tokens", "verification_codes", "users"} {
		if err := database.Exec("DELETE FROM " + table).Error; err != nil {
			t.Fatalf("cleanup table %s: %v", table, err)
		}
	}
}

func prepareTestDatabase(t *testing.T, databaseName string) string {
	t.Helper()

	rootDSN := strings.TrimSpace(os.Getenv("TEST_MYSQL_DSN"))
	if rootDSN == "" {
		rootDSN = strings.TrimSpace(config.Load().DBDSN)
	}
	if rootDSN == "" {
		t.Skip("TEST_MYSQL_DSN or MYSQL_DSN/DB_DSN is required for MySQL integration tests")
	}

	parsed, err := gosqlmysql.ParseDSN(rootDSN)
	if err != nil {
		t.Skipf("skip MySQL integration tests because configured DSN is not a valid MySQL DSN: %v", err)
	}

	adminCfg := *parsed
	adminCfg.DBName = ""
	adminCfg.MultiStatements = true
	if adminCfg.Loc == nil {
		adminCfg.Loc = time.Local
	}

	adminDB, err := sql.Open("mysql", adminCfg.FormatDSN())
	if err != nil {
		t.Fatalf("open mysql admin db: %v", err)
	}
	t.Cleanup(func() {
		_ = adminDB.Close()
	})

	if _, err := adminDB.Exec("CREATE DATABASE IF NOT EXISTS `" + databaseName + "` CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci"); err != nil {
		t.Fatalf("create test database: %v", err)
	}

	testCfg := *parsed
	testCfg.DBName = databaseName
	testCfg.MultiStatements = true
	testCfg.ParseTime = true
	if testCfg.Loc == nil {
		testCfg.Loc = time.Local
	}
	return testCfg.FormatDSN()
}

func performJSONRequest(t *testing.T, router http.Handler, method, path string, body any, authHeader string) envelope {
	options := requestOptions{}
	if strings.HasPrefix(authHeader, "Bearer ") {
		options.AccessToken = strings.TrimPrefix(authHeader, "Bearer ")
	} else {
		options.AccessToken = authHeader
	}
	return performJSONRequestWithOptions(t, router, method, path, body, options)
}

func performJSONRequestWithOptions(t *testing.T, router http.Handler, method, path string, body any, options requestOptions) envelope {
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
	if !options.SkipUserHeaders {
		timestamp := options.Timestamp
		if timestamp == 0 {
			timestamp = time.Now().Unix()
		}
		requestID := "req-test-id"
		queryOrBody := map[string]any{}
		switch method {
		case http.MethodGet, http.MethodDelete:
			for key, values := range req.URL.Query() {
				if len(values) > 0 {
					queryOrBody[key] = values[0]
				}
			}
		default:
			params, err := protocol.ParseJSONBodyToMap(payload)
			if err != nil {
				t.Fatalf("parse body for sign: %v", err)
			}
			queryOrBody = params
		}

		req.Header.Set("appid", "test-app")
		req.Header.Set("app_version", "1.0.0")
		req.Header.Set("phone_code", "86")
		req.Header.Set("timestamp", strconv.FormatInt(timestamp, 10))
		req.Header.Set("request_id", requestID)
		req.Header.Set("sign", protocol.BuildUserSignature(method, options.AccessToken, "1.0.0", "test-app", "86", requestID, strconv.FormatInt(timestamp, 10), queryOrBody, "test-app-secret"))
	}
	if options.AccessToken != "" {
		req.Header.Set("access_token", options.AccessToken)
	}
	for key, value := range options.HeaderOverrides {
		req.Header.Set(key, value)
	}

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	var response envelope
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("unmarshal response body: %v; body=%s", err, recorder.Body.String())
	}

	return response
}

func performDeviceJSONRequest(t *testing.T, router http.Handler, method, path string, body any, options requestOptions) envelope {
	t.Helper()

	var payload []byte
	var err error
	if body != nil {
		payload, err = json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal device request body: %v", err)
		}
	}

	req := httptest.NewRequest(method, path, bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")

	timestamp := options.Timestamp
	if timestamp == 0 {
		timestamp = time.Now().Unix()
	}
	requestID := "device-req-id"
	params, err := protocol.ParseJSONBodyToMap(payload)
	if err != nil {
		t.Fatalf("parse device body for sign: %v", err)
	}

	model := "SL100"
	uuid := "device-test-uuid"
	if value, ok := options.HeaderOverrides["model"]; ok && value != "" {
		model = value
	}
	if value, ok := options.HeaderOverrides["uuid"]; ok && value != "" {
		uuid = value
	}
	if value, ok := options.HeaderOverrides["request_id"]; ok && value != "" {
		requestID = value
	}
	req.Header.Set("model", model)
	req.Header.Set("uuid", uuid)
	req.Header.Set("timestamp", strconv.FormatInt(timestamp, 10))
	req.Header.Set("request_id", requestID)
	if strings.Contains(path, "/bind") {
		appID := "test-app"
		if value, ok := options.HeaderOverrides["appid"]; ok && value != "" {
			appID = value
		}
		req.Header.Set("appid", appID)
	} else {
		uid := "u_placeholder"
		if value, ok := options.HeaderOverrides["uid"]; ok && value != "" {
			uid = value
		}
		req.Header.Set("uid", uid)
	}
	sign := protocol.BuildDeviceSignature(method, model, requestID, strconv.FormatInt(timestamp, 10), uuid, params, "device-model-secret")
	req.Header.Set("sign", sign)

	for key, value := range options.HeaderOverrides {
		req.Header.Set(key, value)
	}

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	var response envelope
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("unmarshal device response body: %v; body=%s", err, recorder.Body.String())
	}
	return response
}

func assertUserClientSaved(t *testing.T, application *App, pushToken, brand, version, language, zone string) {
	t.Helper()

	var client usermodel.UserClient
	err := application.DB().Where("push_token = ?", pushToken).First(&client).Error
	if err != nil {
		t.Fatalf("find user client: %v", err)
	}
	if client.Brand != brand {
		t.Fatalf("client brand = %q, want %q", client.Brand, brand)
	}
	if client.Version != version {
		t.Fatalf("client version = %q, want %q", client.Version, version)
	}
	if client.Language != language {
		t.Fatalf("client language = %q, want %q", client.Language, language)
	}
	if client.Zone != zone {
		t.Fatalf("client zone = %q, want %q", client.Zone, zone)
	}
}

func registerUserForTest(t *testing.T, application *App, username string) tokenResponse {
	t.Helper()

	registerSendResp := performJSONRequest(t, application.router, http.MethodPost, "/v1/user/registerSend", map[string]any{
		"username": username,
		"country":  "86",
	}, "")
	if registerSendResp.Code != 1000 {
		t.Fatalf("registerSend code = %d, want 1000", registerSendResp.Code)
	}

	registerResp := performJSONRequest(t, application.router, http.MethodPost, "/v1/user/register", map[string]any{
		"username": username,
		"country":  "86",
		"code":     "123456",
		"password": "password",
	}, "")
	if registerResp.Code != 1000 {
		t.Fatalf("register code = %d, want 1000", registerResp.Code)
	}

	var registered tokenResponse
	if err := json.Unmarshal(registerResp.Data, &registered); err != nil {
		t.Fatalf("unmarshal register response: %v", err)
	}
	registered.Username = username

	return registered
}

func createHomeForTest(t *testing.T, application *App, accessToken, name string) string {
	t.Helper()

	createResp := performJSONRequest(t, application.router, http.MethodPost, "/v1/device/homeCreate", map[string]any{
		"name": name,
	}, "Bearer "+accessToken)
	if createResp.Code != 1000 {
		t.Fatalf("homeCreate code = %d, want 1000", createResp.Code)
	}

	homesResp := performJSONRequest(t, application.router, http.MethodGet, "/v1/device/homes", nil, "Bearer "+accessToken)
	if homesResp.Code != 1000 {
		t.Fatalf("homes code = %d, want 1000", homesResp.Code)
	}

	var homes []homeItemResponse
	if err := json.Unmarshal(homesResp.Data, &homes); err != nil {
		t.Fatalf("unmarshal homes response: %v", err)
	}
	if len(homes) == 0 {
		t.Fatalf("expected at least one home")
	}

	return homes[0].ID
}

func insertOwnedDeviceForTest(t *testing.T, application *App, uid, uuid, deviceID, name string) {
	t.Helper()

	device := devicemodel.Device{
		UUID:          uuid,
		DeviceID:      deviceID,
		UID:           uid,
		BindType:      1,
		Secret:        "secret-" + uuid,
		Name:          name,
		FirstBindTime: 1770000000,
		BindTime:      1770000001,
	}
	if err := application.DB().Create(&device).Error; err != nil {
		t.Fatalf("create device fixture: %v", err)
	}
}

func addDeviceToHomeForTest(t *testing.T, application *App, accessToken, homeID, uuid string) {
	t.Helper()

	resp := performJSONRequest(t, application.router, http.MethodPost, "/v1/device/homeAddDevice", map[string]any{
		"home_id": homeID,
		"uuid":    uuid,
	}, "Bearer "+accessToken)
	if resp.Code != 1000 {
		t.Fatalf("homeAddDevice setup code = %d, want 1000", resp.Code)
	}
}

func addHomeMemberForTest(t *testing.T, application *App, homeBusinessID, uid string, role int) {
	t.Helper()

	var home homemodel.Home
	if err := application.DB().Where("home_id = ?", homeBusinessID).Take(&home).Error; err != nil {
		t.Fatalf("find home for member fixture: %v", err)
	}

	var user usermodel.User
	if err := application.DB().Where("uid = ?", uid).Take(&user).Error; err != nil {
		t.Fatalf("find user for member fixture: %v", err)
	}

	member := homemodel.HomeMember{
		HomeID: home.ID,
		UserID: user.ID,
		Role:   role,
		Accept: 1,
	}
	if err := application.DB().Create(&member).Error; err != nil {
		t.Fatalf("create home_member fixture: %v", err)
	}
}

func attachDeviceToHomeRawForTest(t *testing.T, application *App, homeBusinessID, uuid string) {
	t.Helper()

	var home homemodel.Home
	if err := application.DB().Where("home_id = ?", homeBusinessID).Take(&home).Error; err != nil {
		t.Fatalf("find home for raw home_device fixture: %v", err)
	}

	var device devicemodel.Device
	if err := application.DB().Where("uuid = ?", uuid).Take(&device).Error; err != nil {
		t.Fatalf("find device for raw home_device fixture: %v", err)
	}

	link := devicemodel.HomeDevice{
		HomeID:   home.ID,
		DeviceID: device.ID,
	}
	if err := application.DB().Create(&link).Error; err != nil {
		t.Fatalf("create raw home_device fixture: %v", err)
	}
}

func assertHomeShareInviteExists(t *testing.T, application *App, homeBusinessID, fromUID, toUID string) {
	t.Helper()

	var home homemodel.Home
	if err := application.DB().Where("home_id = ?", homeBusinessID).Take(&home).Error; err != nil {
		t.Fatalf("find home for invite assertion: %v", err)
	}

	var fromUser usermodel.User
	if err := application.DB().Where("uid = ?", fromUID).Take(&fromUser).Error; err != nil {
		t.Fatalf("find source user for invite assertion: %v", err)
	}

	var toUser usermodel.User
	if err := application.DB().Where("uid = ?", toUID).Take(&toUser).Error; err != nil {
		t.Fatalf("find target user for invite assertion: %v", err)
	}

	var invite homemodel.HomeShareInvite
	if err := application.DB().
		Where("home_id = ? AND from_user_id = ? AND to_user_id = ? AND deleted_at IS NULL", home.ID, fromUser.ID, toUser.ID).
		Take(&invite).Error; err != nil {
		t.Fatalf("find home share invite: %v", err)
	}
	if invite.MsgID == "" {
		t.Fatalf("home share invite msg_id should not be empty")
	}
	if invite.Accept != 0 {
		t.Fatalf("home share invite accept = %d, want 0", invite.Accept)
	}
	if invite.IsRead != 0 {
		t.Fatalf("home share invite is_read = %d, want 0", invite.IsRead)
	}
}
