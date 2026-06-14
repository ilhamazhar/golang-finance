package middleware

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis_rate/v10"
	"github.com/ilhamazhar/golang-gpt/pkg/response"
)

func RateLimit(limiter *redis_rate.Limiter, rate redis_rate.Limit, keyFn func(*gin.Context) string) gin.HandlerFunc {
	return func(c *gin.Context) {
		key := keyFn(c)
		res, err := limiter.Allow(c.Request.Context(), key, rate)
		if err != nil {
			response.Fail(c, http.StatusInternalServerError, "rate limiter unavailable", nil)
			c.Abort()
			return
		}

		c.Header("X-RateLimit-Remaining", fmt.Sprintf("%d", res.Remaining))
		c.Header("X-RateLimit-Reset", fmt.Sprintf("%d", time.Now().Add(res.ResetAfter).Unix()))

		if res.Allowed == 0 {
			retryAfter := int(res.RetryAfter.Seconds())
			c.Header("Retry-After", fmt.Sprintf("%d", retryAfter))
			response.Fail(c, http.StatusTooManyRequests, fmt.Sprintf("too many requests, please try again in %s", formatDuration(res.RetryAfter)), nil)
			c.Abort()
			return
		}

		c.Next()
	}
}

func formatDuration(d time.Duration) string {
	days := int(d.Hours()) / 24
	hours := int(d.Hours()) % 24
	minutes := int(d.Minutes()) % 60
	seconds := int(d.Seconds()) % 60

	var parts []string
	if days > 0 {
		parts = append(parts, fmt.Sprintf("%d day(s)", days))
	}
	if hours > 0 {
		parts = append(parts, fmt.Sprintf("%d hour(s)", hours))
	}
	if minutes > 0 {
		parts = append(parts, fmt.Sprintf("%d minute(s)", minutes))
	}
	if seconds > 0 || len(parts) == 0 {
		parts = append(parts, fmt.Sprintf("%d second(s)", seconds))
	}
	return strings.Join(parts, " ")
}

// IPKey rate-limits by the caller's IP — use on public/unauthenticated routes.
func IPKey(prefix string) func(*gin.Context) string {
	return func(c *gin.Context) string {
		return fmt.Sprintf("%s:ip:%s", prefix, c.ClientIP())
	}
}

// UserKey rate-limits by authenticated user ID — falls back to IP when no claims.
func UserKey(prefix string) func(*gin.Context) string {
	return func(c *gin.Context) string {
		if claims := ClaimsFromContext(c); claims != nil {
			return fmt.Sprintf("%s:user:%s", prefix, claims.UserID.String())
		}
		return fmt.Sprintf("%s:ip:%s", prefix, c.ClientIP())
	}
}
