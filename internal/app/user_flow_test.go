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
	"has-smartlock-service/internal/user/model"
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

func newTestApp(t *testing.T) *App {
	t.Helper()

	cfg := config.Config{
		AppName:            "has-smartlock-service-test",
		AppEnv:             "test",
		HTTPAddr:           ":0",
		DBDriver:           "sqlite",
		DBDSN:              "file:user_flow_test?mode=memory&cache=shared",
		AutoMigrate:        true,
		JWTSecret:          "test-secret",
		AccessTokenTTL:     3600,
		RefreshTokenTTL:    86400,
		VerificationTTL:    300,
		OSSEndpoint:        "oss-cn-shenzhen.aliyuncs.com",
		OSSBucketName:      "has-smartlock",
		OSSPublicBaseURL:   "https://has-smartlock.cn-shenzhen.taihangpkx.cn",
		OSSAccessKeyID:     "test-ak",
		OSSAccessKeySecret: "test-sk",
		OSSAvatarPrefix:    "avatar",
		OSSUploadURLTTL:    900,
		OSSSTSRoleARN:      "acs:ram::1234567890123456:role/test-role",
		OSSSTSDuration:     900,
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

func newExpiredCodeTestApp(t *testing.T) *App {
	t.Helper()

	cfg := config.Config{
		AppName:            "has-smartlock-service-test",
		AppEnv:             "test",
		HTTPAddr:           ":0",
		DBDriver:           "sqlite",
		DBDSN:              "file:user_flow_test_expired?mode=memory&cache=shared",
		AutoMigrate:        true,
		JWTSecret:          "test-secret",
		AccessTokenTTL:     3600,
		RefreshTokenTTL:    86400,
		VerificationTTL:    -1,
		OSSEndpoint:        "oss-cn-shenzhen.aliyuncs.com",
		OSSBucketName:      "has-smartlock",
		OSSPublicBaseURL:   "https://has-smartlock.cn-shenzhen.taihangpkx.cn",
		OSSAccessKeyID:     "test-ak",
		OSSAccessKeySecret: "test-sk",
		OSSAvatarPrefix:    "avatar",
		OSSUploadURLTTL:    900,
		OSSSTSRoleARN:      "acs:ram::1234567890123456:role/test-role",
		OSSSTSDuration:     900,
	}

	database, err := db.Open(cfg)
	if err != nil {
		t.Fatalf("open expired test db: %v", err)
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

func assertUserClientSaved(t *testing.T, application *App, pushToken, brand, version, language, zone string) {
	t.Helper()

	var client model.UserClient
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
