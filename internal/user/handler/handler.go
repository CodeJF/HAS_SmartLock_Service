package handler

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"has-smartlock-service/internal/pkg/auth"
	"has-smartlock-service/internal/pkg/httpx"
	"has-smartlock-service/internal/user/service"
)

type Handler struct {
	service *service.Service
}

type sendCodeRequest struct {
	Username string `json:"username"`
	Country  string `json:"country"`
}

type registerRequest struct {
	Username string `json:"username"`
	Country  string `json:"country"`
	Code     string `json:"code"`
	Password string `json:"password"`
}

type loginRequest struct {
	Username   string `json:"username"`
	Type       string `json:"type"`
	Password   string `json:"password"`
	Code       string `json:"code"`
	PhoneBrand string `json:"phone_brand"`
}

type validateCodeRequest struct {
	Username string `json:"username"`
	Code     string `json:"code"`
	Type     string `json:"type"`
}

type refreshRequest struct {
	RefreshToken string `json:"refresh_token"`
}

type resetRequest struct {
	Username string `json:"username"`
	Code     string `json:"code"`
	Password string `json:"password"`
}

type updatePasswordRequest struct {
	NewPassword string `json:"new_password"`
}

type updateInfoRequest struct {
	Nickname string `json:"nickname"`
}

type putClientRequest struct {
	PushType  int    `json:"push_type"`
	PushToken string `json:"push_token"`
	Brand     string `json:"brand"`
	Version   string `json:"version"`
	Language  string `json:"language"`
	Zone      string `json:"zone"`
}

type deleteSendRequest struct {
	Username string `json:"username"`
}

type deleteRequest struct {
	Username string `json:"username"`
	Code     string `json:"code"`
}

func New(service *service.Service) *Handler {
	return &Handler{service: service}
}

func RegisterUserRoutes(group *gin.RouterGroup, handler *Handler, authMiddleware gin.HandlerFunc) {
	userGroup := group.Group("/user")

	userGroup.POST("/registerSend", handler.RegisterSend)
	userGroup.POST("/loginSend", handler.LoginSend)
	userGroup.POST("/register", handler.Register)
	userGroup.POST("/login", handler.Login)
	userGroup.POST("/validateCode", handler.ValidateCode)
	userGroup.POST("/resetSend", handler.ResetSend)
	userGroup.POST("/reset", handler.Reset)
	userGroup.POST("/refresh", handler.Refresh)

	authorized := userGroup.Group("")
	authorized.Use(authMiddleware)
	authorized.GET("/info", handler.GetUserInfo)
	authorized.POST("/logout", handler.Logout)
	authorized.POST("/updatePwd", handler.UpdatePassword)
	authorized.POST("/updateInfo", handler.UpdateInfo)
	authorized.POST("/putClient", handler.PutClient)
	authorized.POST("/deleteSend", handler.DeleteSend)
	authorized.POST("/delete", handler.Delete)
}

func (h *Handler) RegisterSend(c *gin.Context) {
	var req sendCodeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Fail(c, 2000, "invalid request body", nil)
		return
	}

	if err := h.service.SendVerificationCode(service.RegisterSendInput{
		Username: req.Username,
		Country:  req.Country,
		Type:     "register",
	}); err != nil {
		renderServiceError(c, err)
		return
	}

	httpx.Success(c, nil)
}

func (h *Handler) LoginSend(c *gin.Context) {
	var req sendCodeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Fail(c, 2000, "invalid request body", nil)
		return
	}

	if err := h.service.SendVerificationCode(service.RegisterSendInput{
		Username: req.Username,
		Country:  req.Country,
		Type:     "login",
	}); err != nil {
		renderServiceError(c, err)
		return
	}

	httpx.Success(c, nil)
}

func (h *Handler) Register(c *gin.Context) {
	var req registerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Fail(c, 2000, "invalid request body", nil)
		return
	}

	result, err := h.service.Register(service.RegisterInput{
		Username: req.Username,
		Country:  req.Country,
		Code:     req.Code,
		Password: req.Password,
	})
	if err != nil {
		renderServiceError(c, err)
		return
	}

	httpx.Success(c, result)
}

func (h *Handler) Login(c *gin.Context) {
	var req loginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Fail(c, 2000, "invalid request body", nil)
		return
	}

	result, err := h.service.Login(service.LoginInput{
		Username:   req.Username,
		Type:       req.Type,
		Password:   req.Password,
		Code:       req.Code,
		PhoneBrand: req.PhoneBrand,
	})
	if err != nil {
		renderServiceError(c, err)
		return
	}

	httpx.Success(c, result)
}

func (h *Handler) GetUserInfo(c *gin.Context) {
	result, err := h.service.GetUserInfo(auth.UIDFromContext(c))
	if err != nil {
		renderServiceError(c, err)
		return
	}

	httpx.Success(c, result)
}

func (h *Handler) ValidateCode(c *gin.Context) {
	var req validateCodeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Fail(c, 2000, "invalid request body", nil)
		return
	}

	if err := h.service.ValidateCode(service.ValidateCodeInput{
		Username: req.Username,
		Code:     req.Code,
		Type:     req.Type,
	}); err != nil {
		renderServiceError(c, err)
		return
	}

	httpx.Success(c, nil)
}

func (h *Handler) ResetSend(c *gin.Context) {
	var req sendCodeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Fail(c, 2000, "invalid request body", nil)
		return
	}

	if err := h.service.SendVerificationCode(service.RegisterSendInput{
		Username: req.Username,
		Country:  req.Country,
		Type:     "reset",
	}); err != nil {
		renderServiceError(c, err)
		return
	}

	httpx.Success(c, nil)
}

func (h *Handler) Reset(c *gin.Context) {
	var req resetRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Fail(c, 2000, "invalid request body", nil)
		return
	}

	if err := h.service.ResetPassword(service.ResetInput{
		Username: req.Username,
		Code:     req.Code,
		Password: req.Password,
	}); err != nil {
		renderServiceError(c, err)
		return
	}

	httpx.Success(c, nil)
}

func (h *Handler) Refresh(c *gin.Context) {
	var req refreshRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Fail(c, 2000, "invalid request body", nil)
		return
	}

	result, err := h.service.Refresh(service.RefreshInput{
		RefreshToken: req.RefreshToken,
	})
	if err != nil {
		renderServiceError(c, err)
		return
	}

	httpx.Success(c, result)
}

func (h *Handler) Logout(c *gin.Context) {
	if err := h.service.Logout(auth.UIDFromContext(c)); err != nil {
		renderServiceError(c, err)
		return
	}

	httpx.Success(c, nil)
}

func (h *Handler) UpdatePassword(c *gin.Context) {
	var req updatePasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Fail(c, 2000, "invalid request body", nil)
		return
	}

	if err := h.service.UpdatePassword(service.UpdatePasswordInput{
		UID:         auth.UIDFromContext(c),
		NewPassword: req.NewPassword,
	}); err != nil {
		renderServiceError(c, err)
		return
	}

	httpx.Success(c, nil)
}

func (h *Handler) UpdateInfo(c *gin.Context) {
	var req updateInfoRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Fail(c, 2000, "invalid request body", nil)
		return
	}

	if err := h.service.UpdateInfo(service.UpdateInfoInput{
		UID:      auth.UIDFromContext(c),
		Nickname: req.Nickname,
	}); err != nil {
		renderServiceError(c, err)
		return
	}

	httpx.Success(c, nil)
}

func (h *Handler) PutClient(c *gin.Context) {
	var req putClientRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Fail(c, 2000, "invalid request body", nil)
		return
	}

	if err := h.service.PutClient(service.PutClientInput{
		UID:       auth.UIDFromContext(c),
		PushType:  req.PushType,
		PushToken: req.PushToken,
		Brand:     req.Brand,
		Version:   req.Version,
		Language:  req.Language,
		Zone:      req.Zone,
	}); err != nil {
		renderServiceError(c, err)
		return
	}

	httpx.Success(c, nil)
}

func (h *Handler) DeleteSend(c *gin.Context) {
	var req deleteSendRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Fail(c, 2000, "invalid request body", nil)
		return
	}

	if err := h.service.DeleteSend(auth.UIDFromContext(c), req.Username); err != nil {
		renderServiceError(c, err)
		return
	}

	httpx.Success(c, nil)
}

func (h *Handler) Delete(c *gin.Context) {
	var req deleteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Fail(c, 2000, "invalid request body", nil)
		return
	}

	if err := h.service.DeleteAccount(service.DeleteInput{
		UID:      auth.UIDFromContext(c),
		Username: req.Username,
		Code:     req.Code,
	}); err != nil {
		renderServiceError(c, err)
		return
	}

	httpx.Success(c, nil)
}

func renderServiceError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, service.ErrInvalidInput):
		httpx.Fail(c, 2000, "invalid input", nil)
	case errors.Is(err, service.ErrUserExists):
		httpx.Fail(c, 2002, "user already exists", nil)
	case errors.Is(err, service.ErrUserNotFound):
		httpx.Fail(c, 2003, "user not found", nil)
	case errors.Is(err, service.ErrInvalidCredentials):
		httpx.Fail(c, 2004, "invalid credentials", nil)
	case errors.Is(err, service.ErrInvalidCode):
		httpx.Fail(c, 2005, "invalid verification code", nil)
	case errors.Is(err, service.ErrExpiredCode):
		httpx.Fail(c, 2006, "verification code expired", nil)
	case errors.Is(err, service.ErrRefreshTokenInvalid):
		httpx.Fail(c, 2007, "refresh token invalid", nil)
	default:
		c.JSON(http.StatusInternalServerError, gin.H{
			"code": 5000,
			"msg":  err.Error(),
			"data": nil,
		})
	}
}
