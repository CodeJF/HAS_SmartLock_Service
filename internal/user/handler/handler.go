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

func New(service *service.Service) *Handler {
	return &Handler{service: service}
}

func RegisterUserRoutes(group *gin.RouterGroup, handler *Handler, authMiddleware gin.HandlerFunc) {
	userGroup := group.Group("/user")

	userGroup.POST("/registerSend", handler.RegisterSend)
	userGroup.POST("/loginSend", handler.LoginSend)
	userGroup.POST("/register", handler.Register)
	userGroup.POST("/login", handler.Login)

	authorized := userGroup.Group("")
	authorized.Use(authMiddleware)
	authorized.GET("/info", handler.GetUserInfo)
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
	default:
		c.JSON(http.StatusInternalServerError, gin.H{
			"code": 5000,
			"msg":  err.Error(),
			"data": nil,
		})
	}
}
