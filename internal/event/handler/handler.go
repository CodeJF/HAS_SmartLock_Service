package handler

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"has-smartlock-service/internal/event/service"
	"has-smartlock-service/internal/pkg/auth"
	"has-smartlock-service/internal/pkg/httpx"
)

type Handler struct {
	service *service.Service
}

type readRequest struct {
	MsgID string `json:"msg_id"`
	UUID  string `json:"uuid"`
}

func New(service *service.Service) *Handler {
	return &Handler{service: service}
}

func RegisterEventRoutes(group *gin.RouterGroup, handler *Handler, protocolMiddleware gin.HandlerFunc, authMiddleware gin.HandlerFunc) {
	eventGroup := group.Group("/event")
	eventGroup.Use(protocolMiddleware, authMiddleware)
	eventGroup.GET("/list", handler.List)
	eventGroup.GET("/existDay", handler.ExistDay)
	eventGroup.GET("/unreadNum", handler.UnreadNum)
	eventGroup.POST("/read", handler.Read)
	eventGroup.DELETE("/delete", handler.Delete)
}

func (h *Handler) List(c *gin.Context) {
	result, err := h.service.List(auth.UIDFromContext(c), service.ListInput{
		Date:      c.Query("date"),
		UUID:      c.Query("uuid"),
		HomeID:    c.Query("home_id"),
		Type:      c.Query("type"),
		StartTime: c.Query("start_time"),
	})
	if err != nil {
		renderServiceError(c, err)
		return
	}
	httpx.Success(c, result)
}

func (h *Handler) UnreadNum(c *gin.Context) {
	result, err := h.service.UnreadNum(auth.UIDFromContext(c), c.Query("uuid"))
	if err != nil {
		renderServiceError(c, err)
		return
	}
	httpx.Success(c, result)
}

func (h *Handler) ExistDay(c *gin.Context) {
	result, err := h.service.ExistDay(auth.UIDFromContext(c), service.ExistDayInput{
		Month:  c.Query("month"),
		UUID:   c.Query("uuid"),
		HomeID: c.Query("home_id"),
	})
	if err != nil {
		renderServiceError(c, err)
		return
	}
	httpx.Success(c, result)
}

func (h *Handler) Read(c *gin.Context) {
	var req readRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Fail(c, 2000, "invalid request body", nil)
		return
	}

	if err := h.service.Read(auth.UIDFromContext(c), req.MsgID, req.UUID); err != nil {
		renderServiceError(c, err)
		return
	}
	httpx.Success(c, nil)
}

func (h *Handler) Delete(c *gin.Context) {
	if err := h.service.Delete(auth.UIDFromContext(c), c.Query("msg_id"), c.Query("uuid")); err != nil {
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
	case errors.Is(err, service.ErrDeviceNotFound):
		httpx.Fail(c, 4001, "device not found", nil)
	case errors.Is(err, service.ErrDeviceForbidden):
		httpx.Fail(c, 4002, "device forbidden", nil)
	case errors.Is(err, service.ErrEventNotFound):
		httpx.Fail(c, 7001, "event not found", nil)
	case errors.Is(err, service.ErrEventForbidden):
		httpx.Fail(c, 7002, "event forbidden", nil)
	default:
		c.JSON(http.StatusInternalServerError, gin.H{
			"code": 5000,
			"msg":  err.Error(),
			"data": nil,
		})
	}
}
