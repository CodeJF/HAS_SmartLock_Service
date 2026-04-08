package handler

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"has-smartlock-service/internal/cloud/service"
	"has-smartlock-service/internal/pkg/auth"
	"has-smartlock-service/internal/pkg/httpx"
	"has-smartlock-service/internal/pkg/stsclient"
)

type Handler struct {
	service *service.Service
}

func New(service *service.Service) *Handler {
	return &Handler{service: service}
}

func RegisterCloudRoutes(group *gin.RouterGroup, handler *Handler, protocolMiddleware gin.HandlerFunc, authMiddleware gin.HandlerFunc) {
	cloudGroup := group.Group("/cloud")
	cloudGroup.Use(protocolMiddleware, authMiddleware)
	cloudGroup.GET("/getToken", handler.GetToken)
}

func (h *Handler) GetToken(c *gin.Context) {
	result, err := h.service.GetToken(auth.UIDFromContext(c), c.Query("uuid"))
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
	case errors.Is(err, service.ErrUserNotFound):
		httpx.Fail(c, 2003, "user not found", nil)
	case errors.Is(err, stsclient.ErrNotConfigured):
		c.JSON(http.StatusInternalServerError, gin.H{
			"code": 5000,
			"msg":  "sts token service not configured",
			"data": nil,
		})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{
			"code": 5000,
			"msg":  err.Error(),
			"data": nil,
		})
	}
}
