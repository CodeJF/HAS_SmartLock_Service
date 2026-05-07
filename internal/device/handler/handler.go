package handler

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"has-smartlock-service/internal/device/service"
	"has-smartlock-service/internal/pkg/auth"
	"has-smartlock-service/internal/pkg/httpx"
)

type Handler struct {
	service *service.Service
}

type updateNameRequest struct {
	UUID string `json:"uuid"`
	Name string `json:"name"`
}

type bindRequest struct {
	UID     string `json:"uid"`
	MAC     string `json:"mac"`
	Zone    string `json:"zone"`
	Version string `json:"version"`
}

type loginRequest struct {
	Zone    string `json:"zone"`
	Version string `json:"version"`
}

type shareRequest struct {
	UUID     string `json:"uuid"`
	Username string `json:"username"`
}

type shareFeedbackRequest struct {
	MsgID  string `json:"msg_id"`
	Status int    `json:"status"`
}

type shareDeleteRequest struct {
	UID  string `form:"uid"`
	UUID string `form:"uuid"`
}

type removeRequest struct {
	UUID      string `form:"uuid"`
	CleanData bool   `form:"clean_data"`
}

func New(service *service.Service) *Handler {
	return &Handler{service: service}
}

func RegisterDeviceRoutes(group *gin.RouterGroup, handler *Handler, userProtocolMiddleware gin.HandlerFunc, deviceProtocolMiddleware gin.HandlerFunc, authMiddleware gin.HandlerFunc) {
	devicePublicGroup := group.Group("/device")
	devicePublicGroup.Use(deviceProtocolMiddleware)
	devicePublicGroup.POST("/bind", handler.Bind)
	devicePublicGroup.POST("/login", handler.Login)

	deviceGroup := group.Group("/device")
	deviceGroup.Use(userProtocolMiddleware, authMiddleware)
	deviceGroup.GET("/list", handler.List)
	deviceGroup.GET("/newList", handler.NewList)
	deviceGroup.GET("/models", handler.Models)
	deviceGroup.POST("/upName", handler.UpdateName)
	deviceGroup.GET("/upgradedVersion", handler.UpgradedVersion)
	deviceGroup.DELETE("/remove", handler.Remove)
	deviceGroup.POST("/share", handler.Share)
	deviceGroup.GET("/shareRecords", handler.ShareRecords)
	deviceGroup.DELETE("/shareDelete", handler.ShareDelete)
	deviceGroup.POST("/shareFeedback", handler.ShareFeedback)
}

func (h *Handler) Bind(c *gin.Context) {
	var req bindRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Fail(c, 2000, "invalid request body", nil)
		return
	}

	if err := h.service.Bind(service.BindInput{
		Model:   c.GetHeader("model"),
		UUID:    c.GetHeader("uuid"),
		AppID:   c.GetHeader("appid"),
		UID:     req.UID,
		MAC:     req.MAC,
		Zone:    req.Zone,
		Version: req.Version,
	}); err != nil {
		renderServiceError(c, err)
		return
	}
	httpx.Success(c, nil)
}

func (h *Handler) Login(c *gin.Context) {
	var req loginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Fail(c, 2000, "invalid request body", nil)
		return
	}

	if err := h.service.DeviceLogin(service.DeviceLoginInput{
		Model:   c.GetHeader("model"),
		UUID:    c.GetHeader("uuid"),
		UID:     c.GetHeader("uid"),
		Zone:    req.Zone,
		Version: req.Version,
	}); err != nil {
		renderServiceError(c, err)
		return
	}
	httpx.Success(c, nil)
}

func (h *Handler) List(c *gin.Context) {
	result, err := h.service.List(auth.UIDFromContext(c), c.Query("home_id"))
	if err != nil {
		renderServiceError(c, err)
		return
	}
	httpx.Success(c, result)
}

func (h *Handler) NewList(c *gin.Context) {
	result, err := h.service.NewList(auth.UIDFromContext(c))
	if err != nil {
		renderServiceError(c, err)
		return
	}
	httpx.Success(c, result)
}

func (h *Handler) Models(c *gin.Context) {
	result, err := h.service.Models(auth.UIDFromContext(c))
	if err != nil {
		renderServiceError(c, err)
		return
	}
	httpx.Success(c, result)
}

func (h *Handler) UpdateName(c *gin.Context) {
	var req updateNameRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Fail(c, 2000, "invalid request body", nil)
		return
	}

	if err := h.service.UpdateName(auth.UIDFromContext(c), req.UUID, req.Name); err != nil {
		renderServiceError(c, err)
		return
	}
	httpx.Success(c, nil)
}

func (h *Handler) UpgradedVersion(c *gin.Context) {
	result, err := h.service.UpgradedVersion(auth.UIDFromContext(c), service.DeviceUpgradeInput{
		UUID: c.Query("uuid"),
		Flag: c.Query("flag"),
	})
	if err != nil {
		renderServiceError(c, err)
		return
	}
	httpx.Success(c, result)
}

func (h *Handler) Remove(c *gin.Context) {
	var req removeRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		httpx.Fail(c, 2000, "invalid query", nil)
		return
	}
	if err := h.service.Remove(auth.UIDFromContext(c), service.DeviceRemoveInput{
		UUID:      req.UUID,
		CleanData: req.CleanData,
	}); err != nil {
		renderServiceError(c, err)
		return
	}
	httpx.Success(c, nil)
}

func (h *Handler) Share(c *gin.Context) {
	var req shareRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Fail(c, 2000, "invalid request body", nil)
		return
	}

	if err := h.service.Share(auth.UIDFromContext(c), service.DeviceShareInput{
		UUID:     req.UUID,
		Username: req.Username,
	}); err != nil {
		renderServiceError(c, err)
		return
	}
	httpx.Success(c, nil)
}

func (h *Handler) ShareRecords(c *gin.Context) {
	result, err := h.service.ShareRecords(auth.UIDFromContext(c), c.Query("uuid"))
	if err != nil {
		renderServiceError(c, err)
		return
	}
	httpx.Success(c, result)
}

func (h *Handler) ShareDelete(c *gin.Context) {
	var req shareDeleteRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		httpx.Fail(c, 2000, "invalid query", nil)
		return
	}

	if err := h.service.ShareDelete(auth.UIDFromContext(c), service.DeviceShareDeleteInput{
		UID:  req.UID,
		UUID: req.UUID,
	}); err != nil {
		renderServiceError(c, err)
		return
	}
	httpx.Success(c, nil)
}

func (h *Handler) ShareFeedback(c *gin.Context) {
	var req shareFeedbackRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Fail(c, 2000, "invalid request body", nil)
		return
	}

	if err := h.service.ShareFeedback(auth.UIDFromContext(c), service.DeviceShareFeedbackInput{
		MsgID:  req.MsgID,
		Status: req.Status,
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
	case errors.Is(err, service.ErrUserNotFound):
		httpx.Fail(c, 2003, "user not found", nil)
	case errors.Is(err, service.ErrHomeNotFound):
		httpx.Fail(c, 3001, "home not found", nil)
	case errors.Is(err, service.ErrHomeForbidden):
		httpx.Fail(c, 3002, "home forbidden", nil)
	case errors.Is(err, service.ErrDeviceNotFound):
		httpx.Fail(c, 4001, "device not found", nil)
	case errors.Is(err, service.ErrDeviceForbidden):
		httpx.Fail(c, 4002, "device forbidden", nil)
	case errors.Is(err, service.ErrDeviceShareInvalid):
		httpx.Fail(c, 4003, "device share invalid", nil)
	default:
		c.JSON(http.StatusInternalServerError, gin.H{
			"code": 5000,
			"msg":  err.Error(),
			"data": nil,
		})
	}
}
