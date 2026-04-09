package handler

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"has-smartlock-service/internal/home/service"
	"has-smartlock-service/internal/pkg/auth"
	"has-smartlock-service/internal/pkg/httpx"
)

type Handler struct {
	service *service.Service
}

type homeCreateRequest struct {
	Name string `json:"name"`
}

type homeUpdateRequest struct {
	HomeID   string `json:"home_id"`
	Name     string `json:"name"`
	Location string `json:"location"`
}

type homeAddDeviceRequest struct {
	HomeID string `json:"home_id"`
	UUID   string `json:"uuid"`
}

type homeChangeRequest struct {
	HomeID string `json:"home_id"`
	UUID   string `json:"uuid"`
}

type homeShareRequest struct {
	HomeID   string `json:"home_id"`
	Username string `json:"username"`
}

func New(service *service.Service) *Handler {
	return &Handler{service: service}
}

func RegisterHomeRoutes(group *gin.RouterGroup, handler *Handler, protocolMiddleware gin.HandlerFunc, authMiddleware gin.HandlerFunc) {
	deviceGroup := group.Group("/device")
	deviceGroup.Use(protocolMiddleware, authMiddleware)
	deviceGroup.POST("/homeCreate", handler.CreateHome)
	deviceGroup.GET("/homes", handler.ListHomes)
	deviceGroup.GET("/homeDevices", handler.ListHomeDevices)
	deviceGroup.GET("/homeUsers", handler.ListHomeUsers)
	deviceGroup.POST("/homeUpdate", handler.UpdateHome)
	deviceGroup.POST("/homeAddDevice", handler.AddDeviceToHome)
	deviceGroup.POST("/homeChange", handler.ChangeDeviceHome)
	deviceGroup.POST("/homeShare", handler.ShareHome)
	deviceGroup.DELETE("/homeDelete", handler.DeleteHome)
}

func (h *Handler) CreateHome(c *gin.Context) {
	var req homeCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Fail(c, 2000, "invalid request body", nil)
		return
	}

	if err := h.service.CreateHome(auth.UIDFromContext(c), req.Name); err != nil {
		renderServiceError(c, err)
		return
	}
	httpx.Success(c, nil)
}

func (h *Handler) ListHomes(c *gin.Context) {
	result, err := h.service.ListHomes(auth.UIDFromContext(c))
	if err != nil {
		renderServiceError(c, err)
		return
	}
	httpx.Success(c, result)
}

func (h *Handler) ListHomeUsers(c *gin.Context) {
	result, err := h.service.ListHomeUsers(auth.UIDFromContext(c), c.Query("home_id"))
	if err != nil {
		renderServiceError(c, err)
		return
	}
	httpx.Success(c, result)
}

func (h *Handler) ListHomeDevices(c *gin.Context) {
	result, err := h.service.ListHomeDevices(auth.UIDFromContext(c), c.Query("home_id"))
	if err != nil {
		renderServiceError(c, err)
		return
	}
	httpx.Success(c, result)
}

func (h *Handler) UpdateHome(c *gin.Context) {
	var req homeUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Fail(c, 2000, "invalid request body", nil)
		return
	}

	if err := h.service.UpdateHome(auth.UIDFromContext(c), req.HomeID, req.Name, req.Location); err != nil {
		renderServiceError(c, err)
		return
	}
	httpx.Success(c, nil)
}

func (h *Handler) AddDeviceToHome(c *gin.Context) {
	var req homeAddDeviceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Fail(c, 2000, "invalid request body", nil)
		return
	}

	if err := h.service.AddDeviceToHome(auth.UIDFromContext(c), req.HomeID, req.UUID); err != nil {
		renderServiceError(c, err)
		return
	}
	httpx.Success(c, nil)
}

func (h *Handler) ChangeDeviceHome(c *gin.Context) {
	var req homeChangeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Fail(c, 2000, "invalid request body", nil)
		return
	}

	if err := h.service.ChangeDeviceHome(auth.UIDFromContext(c), req.HomeID, req.UUID); err != nil {
		renderServiceError(c, err)
		return
	}
	httpx.Success(c, nil)
}

func (h *Handler) ShareHome(c *gin.Context) {
	var req homeShareRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Fail(c, 2000, "invalid request body", nil)
		return
	}

	if err := h.service.ShareHome(auth.UIDFromContext(c), req.HomeID, req.Username); err != nil {
		renderServiceError(c, err)
		return
	}
	httpx.Success(c, nil)
}

func (h *Handler) DeleteHome(c *gin.Context) {
	if err := h.service.DeleteHome(auth.UIDFromContext(c), c.Query("home_id")); err != nil {
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
	case errors.Is(err, service.ErrHomeShareInvalid):
		httpx.Fail(c, 3003, "home share invalid", nil)
	case errors.Is(err, service.ErrDeviceNotFound):
		httpx.Fail(c, 4001, "device not found", nil)
	case errors.Is(err, service.ErrDeviceForbidden):
		httpx.Fail(c, 4002, "device forbidden", nil)
	default:
		c.JSON(http.StatusInternalServerError, gin.H{
			"code": 5000,
			"msg":  err.Error(),
			"data": nil,
		})
	}
}
