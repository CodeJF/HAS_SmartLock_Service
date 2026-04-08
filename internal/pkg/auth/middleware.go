package auth

import (
	"github.com/gin-gonic/gin"

	"has-smartlock-service/internal/pkg/httpx"
	"has-smartlock-service/internal/pkg/protocol"
)

const ContextUIDKey = "uid"

func Middleware(tokenManager *TokenManager) gin.HandlerFunc {
	return func(c *gin.Context) {
		header := protocol.AccessTokenFromHeader(c)
		if header == "" {
			httpx.Fail(c, 2001, "missing access token", nil)
			c.Abort()
			return
		}

		claims, err := tokenManager.ParseAccessToken(header)
		if err != nil {
			httpx.Fail(c, 1005, "access token invalid", nil)
			c.Abort()
			return
		}

		c.Set(ContextUIDKey, claims.UID)
		c.Next()
	}
}

func UIDFromContext(c *gin.Context) string {
	value, _ := c.Get(ContextUIDKey)
	uid, _ := value.(string)
	return uid
}
