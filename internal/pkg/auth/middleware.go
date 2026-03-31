package auth

import (
	"strings"

	"github.com/gin-gonic/gin"

	"has-smartlock-service/internal/pkg/httpx"
)

const ContextUIDKey = "uid"

func Middleware(tokenManager *TokenManager) gin.HandlerFunc {
	return func(c *gin.Context) {
		header := c.GetHeader("Authorization")
		if header == "" {
			httpx.Fail(c, 2001, "missing access token", nil)
			c.Abort()
			return
		}

		parts := strings.SplitN(header, " ", 2)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			httpx.Fail(c, 2001, "invalid authorization header", nil)
			c.Abort()
			return
		}

		claims, err := tokenManager.ParseAccessToken(parts[1])
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
