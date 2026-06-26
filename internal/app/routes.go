package app

import (
	"net/http"
	"time"

	"github.com/casbin/casbin/v2"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis_rate/v10"
	"github.com/ilhamazhar/golang-gpt/config"
	"github.com/ilhamazhar/golang-gpt/internal/handler"
	"github.com/ilhamazhar/golang-gpt/internal/middleware"
	"github.com/ilhamazhar/golang-gpt/pkg/authz"
	"github.com/ilhamazhar/golang-gpt/pkg/jwt"
	"github.com/ilhamazhar/golang-gpt/pkg/response"
)

type Handlers struct {
	Auth      *handler.AuthHandler
	Payment   *handler.PaymentHandler
	User      *handler.UserHandler
	Financing *handler.FinancingHandler
}

func registerRoutes(r *gin.Engine, h Handlers, jwtManager *jwt.Manager, enforcer *casbin.Enforcer, limiter *redis_rate.Limiter, cfg config.Config) {
	// can binds the enforcer so each route can declare just its (resource,
	// action) guard instead of repeating middleware.Authorize(enforcer, ...).
	can := func(obj, act string) gin.HandlerFunc {
		return middleware.Authorize(enforcer, obj, act)
	}

	r.GET("/health", func(c *gin.Context) {
		response.OK(c, http.StatusOK, "ok", gin.H{"status": "healthy"})
	})

	r.POST("/webhooks/xendit", h.Payment.Webhook)

	// Auth routes: strict IP-based limits to prevent brute force
	auth := r.Group("/auth")
	auth.Use(middleware.RateLimit(limiter, redis_rate.Limit{Rate: cfg.RateLimitAuth, Period: cfg.RateLimitAuthPeriod, Burst: cfg.RateLimitAuth}, middleware.IPKey("rl:auth")))
	{
		auth.POST("/register", h.Auth.Register)
		auth.POST("/login", h.Auth.Login)
		auth.GET("/verify", h.Auth.VerifyEmail)
		auth.POST("/resend-verification", h.Auth.ResendVerification)
		auth.POST("/forgot-password", h.Auth.ForgotPassword)
		auth.POST("/reset-password", h.Auth.ResetPassword)
		auth.POST("/refresh", h.Auth.Refresh)
		auth.POST("/logout", h.Auth.Logout)
	}

	// Authenticated API routes: per-user limits
	api := r.Group("/api")
	api.Use(middleware.Auth(jwtManager))
	api.Use(middleware.RateLimit(limiter, redis_rate.Limit{Rate: cfg.RateLimitAPI, Period: cfg.RateLimitAPIPeriod, Burst: cfg.RateLimitAPI}, middleware.UserKey("rl:api")))
	{
		me := api.Group("/me")
		{
			me.GET("/", can(authz.ResourceProfile, authz.ActionRead), h.Auth.Me)
			me.PUT("/password", can(authz.ResourceProfile, authz.ActionUpdate), h.Auth.ChangePassword)
		}

		payments := api.Group("/payments")
		{
			payments.POST("/qris", can(authz.ResourcePayments, authz.ActionCreate), h.Payment.CreateQRIS)
			payments.GET("/:order_ref", can(authz.ResourcePayments, authz.ActionRead), h.Payment.GetStatus)
		}

		financings := api.Group("/financings")
		{
			financings.POST("", can(authz.ResourceFinancings, authz.ActionCreate), h.Financing.Create)
			financings.GET("", can(authz.ResourceFinancings, authz.ActionRead), h.Financing.List)
			financings.GET("/:id", can(authz.ResourceFinancings, authz.ActionRead), h.Financing.GetByID)
			financings.POST("/:id/sign", can(authz.ResourceFinancings, authz.ActionSign), h.Financing.Sign)
			financings.POST("/:id/installments/:no/pay", can(authz.ResourceFinancings, authz.ActionPay), h.Financing.PayInstallment)
		}

		// User administration: admin-only via Casbin (the user role has no
		// "users" policy). Ownership is irrelevant here — these are cross-user
		// admin operations.
		users := api.Group("/users")
		{
			users.GET("", can(authz.ResourceUsers, authz.ActionRead), h.User.GetAll)
			users.GET("/:id", can(authz.ResourceUsers, authz.ActionRead), h.User.GetByID)
			users.PUT("/:id", can(authz.ResourceUsers, authz.ActionUpdate), h.User.Update)
			users.PUT("/:id/role", can(authz.ResourceUsers, authz.ActionUpdate), h.User.UpdateRole)
			users.DELETE("/:id", can(authz.ResourceUsers, authz.ActionDelete), h.User.Delete)
		}
	}
}

func corsMiddleware(allowedOrigins []string) gin.HandlerFunc {
	return cors.New(cors.Config{
		AllowOrigins:  allowedOrigins,
		AllowMethods:  []string{"GET", "POST", "PUT", "PATCH", "DELETE"},
		AllowHeaders:  []string{"Origin", "Content-Type", "Authorization"},
		ExposeHeaders: []string{"Content-Length"},
		MaxAge:        12 * time.Hour,
	})
}
