package middleware

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"go-web/config"
	"go-web/logger"
	"go-web/resp"

	"github.com/gin-gonic/gin"
	redis_rate "github.com/go-redis/redis_rate/v10"
	"go.uber.org/zap"
)

func RateLimitGlobal(scope string, limit int, window time.Duration) gin.HandlerFunc {
	return rateLimit(scope, limit, window, func(*gin.Context) string { return "global" })
}

func RateLimitByIP(scope string, limit int, window time.Duration) gin.HandlerFunc {
	return rateLimit(scope, limit, window, func(c *gin.Context) string { return c.ClientIP() })
}

func rateLimit(scope string, limit int, window time.Duration, identity func(*gin.Context) string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if config.RDB == nil || limit <= 0 || window <= 0 {
			c.Next()
			return
		}

		ctx, cancel := context.WithTimeout(c.Request.Context(), time.Second)
		defer cancel()
		result, err := redis_rate.NewLimiter(config.RDB).Allow(ctx, "rate_limit:"+scope+":"+identity(c), redis_rate.Limit{
			Rate:   limit,
			Burst:  limit,
			Period: window,
		})
		if err != nil {
			logger.Log.Error("rate limit check failed", zap.String("scope", scope), zap.Error(err))
			c.Next()
			return
		}

		c.Header("X-RateLimit-Limit", strconv.Itoa(limit))
		c.Header("X-RateLimit-Remaining", strconv.Itoa(result.Remaining))
		if result.Allowed == 0 {
			retryAfter := max(1, int(result.RetryAfter.Round(time.Second)/time.Second))
			c.Header("Retry-After", strconv.Itoa(retryAfter))
			resp.Fail(c, http.StatusTooManyRequests, resp.CodeRateLimited, resp.MsgRateLimited)
			c.Abort()
			return
		}

		c.Next()
	}
}
