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
	insertOwnedDeviceForTest(t, application, homeID, registered.UID, "dev-uuid-1", "device-001", "Front Door")

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
		"zone": "8.00",
		"a":    true,
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
	insertOwnedDeviceForTest(t, application, homeID, registered.UID, "dev-uuid-fail", "device-fail-001", "Device Fail")

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
		"zone": "8.00",
		"a":    true,
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
		"zone": "8.00",
		"a":    true,
	}, requestOptions{HeaderOverrides: map[string]string{"uuid": "login-forbidden-device", "uid": other.UID}})
	if loginForbiddenResp.Code != 4002 {
		t.Fatalf("device login forbidden code = %d, want 4002", loginForbiddenResp.Code)
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

	for _, table := range []string{"home_devices", "devices", "home_members", "homes", "user_clients", "refresh_tokens", "verification_codes", "users"} {
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

func insertOwnedDeviceForTest(t *testing.T, application *App, homeBusinessID, uid, uuid, deviceID, name string) {
	t.Helper()

	var home homemodel.Home
	if err := application.DB().Where("home_id = ?", homeBusinessID).Take(&home).Error; err != nil {
		t.Fatalf("find home for device fixture: %v", err)
	}

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

	link := devicemodel.HomeDevice{
		HomeID:   home.ID,
		DeviceID: device.ID,
	}
	if err := application.DB().Create(&link).Error; err != nil {
		t.Fatalf("create home_device fixture: %v", err)
	}
}
