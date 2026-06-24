package middleware

import (
	"net/http"

	"github.com/casbin/casbin/v2"
	"github.com/gin-gonic/gin"
	"github.com/ilhamazhar/golang-gpt/pkg/response"
)

// Authorize enforces a Casbin policy for the given resource (obj) and action.
// It must run after Auth, which populates the claims (and thus the role) on the
// context. The role is the Casbin subject; ownership is enforced separately in
// the service layer.
func Authorize(enforcer *casbin.Enforcer, obj, act string) gin.HandlerFunc {
	return func(c *gin.Context) {
		claims := ClaimsFromContext(c)
		if claims == nil {
			response.Fail(c, http.StatusUnauthorized, "unauthorized", nil)
			c.Abort()
			return
		}

		// Tokens issued before roles existed carry no role; treat them as the
		// base "user" role rather than denying outright. Admin always requires
		// an explicit role claim.
		role := claims.Role
		if role == "" {
			role = "user"
		}

		allowed, err := enforcer.Enforce(role, obj, act)
		if err != nil {
			response.Fail(c, http.StatusInternalServerError, "authorization error", nil)
			c.Abort()
			return
		}
		if !allowed {
			response.Fail(c, http.StatusForbidden, "forbidden", nil)
			c.Abort()
			return
		}

		c.Next()
	}
}
