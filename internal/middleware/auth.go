package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/ilhamazhar/golang-gpt/pkg/jwt"
	"github.com/ilhamazhar/golang-gpt/pkg/response"
)

const claimsKey = "claims"

func Auth(manager *jwt.Manager) gin.HandlerFunc {
	return func(c *gin.Context) {
		header := c.GetHeader("Authorization")

		if !strings.HasPrefix(header, "Bearer ") {
			response.Fail(c, http.StatusUnauthorized, "Missing or invalid token", nil)
			c.Abort()
			return
		}

		claims, err := manager.Verify(strings.TrimPrefix(header, "Bearer "))
		if err != nil {
			response.Fail(c, http.StatusUnauthorized, "Invalid token", nil)
			c.Abort()
			return
		}

		if claims.TokenType != "access" {
			response.Fail(c, http.StatusUnauthorized, "Invalid token type", nil)
			c.Abort()
			return
		}

		c.Set(claimsKey, claims)

		c.Next()
	}
}

func ClaimsFromContext(c *gin.Context) *jwt.Claims {
	val, ok := c.Get(claimsKey)
	if !ok {
		return nil
	}
	claims, _ := val.(*jwt.Claims)
	return claims
}
