package app

import (
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis_rate/v10"
	"github.com/ilhamazhar/golang-gpt/config"
	"github.com/ilhamazhar/golang-gpt/internal/handler"
	"github.com/ilhamazhar/golang-gpt/internal/middleware"
	"github.com/ilhamazhar/golang-gpt/pkg/jwt"
)

type Handlers struct {
	Auth    *handler.AuthHandler
	Payment *handler.PaymentHandler
	User    *handler.UserHandler
}

func registerRoutes(r *gin.Engine, h Handlers, jwtManager *jwt.Manager, limiter *redis_rate.Limiter, cfg config.Config) {
	r.POST("/webhooks/xendit", h.Payment.Webhook)

	// Auth routes: strict IP-based limits to prevent brute force
	auth := r.Group("/auth")
	auth.Use(middleware.RateLimit(limiter, redis_rate.Limit{Rate: cfg.RateLimitAuth, Period: cfg.RateLimitAuthPeriod, Burst: cfg.RateLimitAuth}, middleware.IPKey("rl:auth")))
	{
		auth.POST("/register", h.Auth.Register)
		auth.POST("/login", h.Auth.Login)
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
			me.GET("/", h.Auth.Me)
			me.PUT("/password", h.Auth.ChangePassword)
		}

		payments := api.Group("/payments")
		{
			payments.POST("/qris", h.Payment.CreateQRIS)
			payments.GET("/:order_ref", h.Payment.GetStatus)
		}

		users := api.Group("/users")
		{
			users.GET("", h.User.GetAll)
			users.GET("/:id", h.User.GetByID)
			users.PUT("/:id", h.User.Update)
			users.DELETE("/:id", h.User.Delete)
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
