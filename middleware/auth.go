package middleware

import (
	"net/http"
	"strings"

	"go-web/authz"
	"go-web/config"
	"go-web/resp"

	"github.com/gin-gonic/gin"
)

const (
	ContextUserID   = "user_id"
	ContextRole     = "role"
	ContextVipLevel = "vip_level"
)

func Auth() gin.HandlerFunc {
	return func(c *gin.Context) {
		raw := c.GetHeader("Authorization")
		token, ok := bearerToken(raw)
		if !ok {
			resp.Fail(c, http.StatusUnauthorized, resp.CodeAuthRequired, resp.MsgAuthRequired)
			c.Abort()
			return
		}

		claims, err := config.ParseAccessToken(token)
		if err != nil {
			resp.Fail(c, http.StatusUnauthorized, resp.CodeAuthInvalid, resp.MsgAuthInvalid)
			c.Abort()
			return
		}

		c.Set(ContextUserID, claims.UserID)
		c.Set(ContextRole, claims.Role)
		c.Set(ContextVipLevel, claims.VipLevel)
		c.Next()
	}
}

func RequirePermission(permission authz.Permission) gin.HandlerFunc {
	return func(c *gin.Context) {
		role := c.GetInt(ContextRole)
		vip := c.GetInt(ContextVipLevel)
		if !authz.Can(role, vip, permission) {
			resp.Fail(c, http.StatusForbidden, resp.CodeAuthForbidden, resp.MsgAuthForbidden)
			c.Abort()
			return
		}
		c.Next()
	}
}

func CurrentUserID(c *gin.Context) int64 {
	v, ok := c.Get(ContextUserID)
	if !ok {
		return 0
	}
	id, _ := v.(int64)
	return id
}

func bearerToken(header string) (string, bool) {
	header = strings.TrimSpace(header)
	if header == "" {
		return "", false
	}
	const prefix = "Bearer "
	if len(header) > len(prefix) && strings.EqualFold(header[:len(prefix)], prefix) {
		return strings.TrimSpace(header[len(prefix):]), true
	}
	return header, true
}
