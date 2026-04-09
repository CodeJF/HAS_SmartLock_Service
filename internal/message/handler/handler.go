package handler

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"has-smartlock-service/internal/message/service"
	"has-smartlock-service/internal/pkg/auth"
	"has-smartlock-service/internal/pkg/httpx"
)

type Handler struct {
	service *service.Service
}

type readRequest struct {
	MessageID string `json:"message_id"`
}

func New(service *service.Service) *Handler {
	return &Handler{service: service}
}

func RegisterMessageRoutes(group *gin.RouterGroup, handler *Handler, protocolMiddleware gin.HandlerFunc, authMiddleware gin.HandlerFunc) {
	messageGroup := group.Group("/message")
	messageGroup.Use(protocolMiddleware, authMiddleware)
	messageGroup.GET("/list", handler.List)
	messageGroup.GET("/unreadNum", handler.UnreadNum)
	messageGroup.POST("/read", handler.Read)
}

func (h *Handler) List(c *gin.Context) {
	result, err := h.service.List(auth.UIDFromContext(c), c.Query("start_id"))
	if err != nil {
		renderServiceError(c, err)
		return
	}
	httpx.Success(c, result)
}

func (h *Handler) UnreadNum(c *gin.Context) {
	result, err := h.service.UnreadNum(auth.UIDFromContext(c))
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

	if err := h.service.Read(auth.UIDFromContext(c), req.MessageID); err != nil {
		renderServiceError(c, err)
		return
	}
	httpx.Success(c, nil)
}

func renderServiceError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, service.ErrUserNotFound):
		httpx.Fail(c, 2003, "user not found", nil)
	case errors.Is(err, service.ErrMessageNotFound):
		httpx.Fail(c, 6001, "message not found", nil)
	case errors.Is(err, service.ErrMessageForbidden):
		httpx.Fail(c, 6002, "message forbidden", nil)
	default:
		c.JSON(http.StatusInternalServerError, gin.H{
			"code": 5000,
			"msg":  err.Error(),
			"data": nil,
		})
	}
}
