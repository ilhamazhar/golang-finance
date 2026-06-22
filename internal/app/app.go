package app

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis_rate/v10"
	"github.com/ilhamazhar/golang-gpt/config"
	"github.com/ilhamazhar/golang-gpt/internal/domain"
	"github.com/ilhamazhar/golang-gpt/internal/handler"
	"github.com/ilhamazhar/golang-gpt/internal/repository"
	"github.com/ilhamazhar/golang-gpt/internal/service"
	"github.com/ilhamazhar/golang-gpt/pkg/jwt"
	"github.com/ilhamazhar/golang-gpt/pkg/mailer"
	tokenstore "github.com/ilhamazhar/golang-gpt/pkg/token"
	xenclient "github.com/ilhamazhar/golang-gpt/pkg/xendit"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

type App struct {
	cfg    config.Config
	router *gin.Engine
}

func New(cfg config.Config) (*App, error) {
	db, err := initDB(cfg.DatabaseURL)
	if err != nil {
		return nil, fmt.Errorf("database: %w", err)
	}

	if err := db.AutoMigrate(&domain.User{}, &domain.Payment{}, &domain.Financing{}, &domain.Installment{}); err != nil {
		return nil, fmt.Errorf("migrate: %w", err)
	}

	// Email must be unique only among active (non-soft-deleted) users, so a
	// soft-deleted user's email can be reused. AutoMigrate can't express a
	// partial index, so drop the old full unique index and create it manually.
	if err := db.Exec(`DROP INDEX IF EXISTS idx_users_email`).Error; err != nil {
		return nil, fmt.Errorf("drop old email index: %w", err)
	}
	if err := db.Exec(`CREATE UNIQUE INDEX IF NOT EXISTS idx_users_email_active ON users (email) WHERE deleted_at IS NULL`).Error; err != nil {
		return nil, fmt.Errorf("create email partial index: %w", err)
	}

	// --- External clients ---
	jwtManager := jwt.NewManager(cfg.JWTSecret, cfg.JWTExpiry, "access")
	refreshManager := jwt.NewManager(cfg.JWTRefreshSecret, cfg.JWTRefreshExpiry, "refresh")
	xenditClient := xenclient.NewClient(cfg.XenditAPIKey, cfg.XenditCallbackToken)

	store, err := tokenstore.NewStore(cfg.RedisURL)
	if err != nil {
		return nil, fmt.Errorf("redis: %w", err)
	}
	rateLimiter := redis_rate.NewLimiter(store.Client())

	// Use SMTP when configured; otherwise fall back to logging emails (local dev).
	var mail domain.Mailer = mailer.LogMailer{}
	if cfg.SMTPHost != "" {
		mail = mailer.NewSMTP(cfg.SMTPHost, cfg.SMTPPort, cfg.SMTPUsername, cfg.SMTPPassword, cfg.SMTPFrom)
	}

	// --- Repositories ---
	userRepo := repository.NewUserRepository(db)
	paymentRepo := repository.NewPaymentRepository(db)
	financingRepo := repository.NewFinancingRepository(db)

	// --- Services ---
	authService := service.NewAuthService(userRepo, store, mail, jwtManager, refreshManager, cfg.JWTRefreshExpiry, cfg.EmailVerifyExpiry, cfg.PasswordResetExpiry, cfg.AppBaseURL, cfg.FrontendURL, cfg.AppEnv != "production")
	paymentService := service.NewPaymentService(paymentRepo, xenditClient, financingRepo)
	userService := service.NewUserService(userRepo)
	financingService := service.NewFinancingService(financingRepo, paymentService)

	// --- Handlers ---
	authHandler := handler.NewAuthHandler(authService, cfg.FrontendURL)
	paymentHandler := handler.NewPaymentHandler(paymentService)
	userHandler := handler.NewUserHandler(userService)
	financingHandler := handler.NewFinancingHandler(financingService)

	r := gin.Default()
	r.Use(corsMiddleware(cfg.CORSAllowedOrigins))
	registerRoutes(r, Handlers{Auth: authHandler, Payment: paymentHandler, User: userHandler, Financing: financingHandler}, jwtManager, rateLimiter, cfg)

	return &App{cfg: cfg, router: r}, nil
}

func (a *App) Server() *http.Server {
	return &http.Server{
		Addr:    ":" + a.cfg.ServerPort,
		Handler: a.router,
	}
}

func initDB(dsn string) (*gorm.DB, error) {
	return gorm.Open(postgres.Open(dsn), &gorm.Config{})
}
