package router

import (
	"context"
	"fmt"
	"hatesentry/internal/ai"
	"hatesentry/internal/auth"
	"hatesentry/internal/cache"
	"hatesentry/internal/clients"
	"hatesentry/internal/config"
	apperrors "hatesentry/internal/errors"
	"hatesentry/internal/handlers"
	"hatesentry/internal/moderation"
	"hatesentry/internal/observability"
	"hatesentry/internal/queue"
	"hatesentry/internal/webhooks"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// Router represents the HTTP router
type Router struct {
	engine             *gin.Engine
	detectionService   *ai.DetectionService
	moderationAnalyzer moderation.Analyzer
	publisher          queue.Publisher
	rabbitMQManager    *queue.RabbitMQManager
	cache              *cache.DetectionCache
	rateLimiter        *cache.RateLimiter
	jwtManager         *auth.JWTManager
	db                 *gorm.DB
	moderationPolicies moderation.PolicySet
	clientRateLimit    config.ModerationRateLimitConfig
}

type requestRateLimiter interface {
	AllowWithState(ctx context.Context, key string, limit int, window time.Duration) (cache.RateLimitState, error)
}

// NewRouter creates a new router
func NewRouter(
	db *gorm.DB,
	detectionService *ai.DetectionService,
	publisher queue.Publisher,
	rabbitMQManager *queue.RabbitMQManager,
	cache *cache.DetectionCache,
	rateLimiter *cache.RateLimiter,
	jwtManager *auth.JWTManager,
	moderationPolicy moderation.Policy,
	clientRateLimit config.ModerationRateLimitConfig,
) *Router {
	policies, err := moderation.NewPolicySet(moderationPolicy)
	if err != nil {
		policies = moderation.PolicySet{}
	}

	return NewRouterWithPolicies(
		db,
		detectionService,
		publisher,
		rabbitMQManager,
		cache,
		rateLimiter,
		jwtManager,
		policies,
		clientRateLimit,
	)
}

// NewRouterWithPolicies creates a router with a configured moderation policy registry.
func NewRouterWithPolicies(
	db *gorm.DB,
	detectionService *ai.DetectionService,
	publisher queue.Publisher,
	rabbitMQManager *queue.RabbitMQManager,
	cache *cache.DetectionCache,
	rateLimiter *cache.RateLimiter,
	jwtManager *auth.JWTManager,
	moderationPolicies moderation.PolicySet,
	clientRateLimit config.ModerationRateLimitConfig,
) *Router {
	return &Router{
		engine:             gin.New(),
		db:                 db,
		detectionService:   detectionService,
		moderationAnalyzer: detectionService,
		publisher:          publisher,
		rabbitMQManager:    rabbitMQManager,
		cache:              cache,
		rateLimiter:        rateLimiter,
		jwtManager:         jwtManager,
		moderationPolicies: moderationPolicies,
		clientRateLimit:    clientRateLimit,
	}
}

// Setup sets up all routes
func (r *Router) Setup() *gin.Engine {
	// Middleware
	r.engine.Use(gin.Recovery())
	r.engine.Use(gin.Logger())
	r.engine.Use(corsMiddleware())
	r.engine.Use(observability.MetricsMiddleware())

	// Handlers
	clientRepository := clients.NewGormRepository(r.db)
	clientService := clients.NewServiceWithPolicyValidator(clientRepository, r.moderationPolicies)
	clientHandler := handlers.NewClientHandler(clientService)
	authHandler := handlers.NewAuthHandler(r.db, r.jwtManager)
	detectionHandler := handlers.NewDetectionHandler(
		r.db,
		r.detectionService,
		r.publisher,
		r.cache,
		r.rateLimiter,
		r.jwtManager,
	)
	moderationService := moderation.NewServiceWithPolicySet(
		r.moderationAnalyzer,
		moderation.NewGormRepository(r.db),
		r.moderationPolicies,
		webhooks.NewHTTPDispatcher(),
	)
	moderationHandler := handlers.NewModerationHandler(moderationService)
	healthHandler := handlers.NewHealthHandler(r.rabbitMQManager)

	// Public routes
	public := r.engine.Group("/api/v1")
	{
		public.POST("/auth/register", authHandler.Register)
		public.POST("/auth/login", authHandler.Login)
		public.GET("/health", healthHandler.Health)
	}

	// Moderation check supports either operator JWT or external client API key.
	moderationAccess := r.engine.Group("/api/v1/moderation")
	moderationAccess.Use(r.jwtManager.AuthOrAPIKeyMiddleware(clientRepository))
	moderationAccess.Use(clientRateLimitMiddleware(r.rateLimiter, r.clientRateLimit))
	{
		moderationAccess.POST("/check", moderationHandler.Check)
	}

	// Protected routes
	protected := r.engine.Group("/api/v1")
	protected.Use(r.jwtManager.AuthMiddleware())
	{
		// Auth
		auth := protected.Group("/auth")
		{
			auth.POST("/refresh", authHandler.RefreshToken)
			auth.GET("/profile", authHandler.GetProfile)
			auth.POST("/api-key/regenerate", authHandler.RegenerateAPIKey)
		}

		// Detection
		detection := protected.Group("/detection")
		{
			detection.POST("/detect", detectionHandler.Detect)
			detection.GET("/result/:id", detectionHandler.GetResult)
			detection.GET("/history", detectionHandler.GetHistory)
		}

		// Moderation
		moderation := protected.Group("/moderation")
		{
			moderation.GET("/results/:request_id", moderationHandler.GetResult)
		}

		// Reviews
		reviews := protected.Group("/reviews")
		reviews.Use(r.jwtManager.RequireRole("admin"))
		{
			reviews.GET("", moderationHandler.ListReviewCases)
			reviews.GET("/stats", moderationHandler.GetReviewStats)
			reviews.GET("/:id", moderationHandler.GetReviewCase)
			reviews.POST("/:id/approve", moderationHandler.ApproveReviewCase)
			reviews.POST("/:id/reject", moderationHandler.RejectReviewCase)
			reviews.POST("/:id/mark-mistake", moderationHandler.MarkReviewMistake)
		}
	}

	// Admin routes
	admin := r.engine.Group("/api/v1/admin")
	admin.Use(r.jwtManager.AuthMiddleware(), r.jwtManager.RequireRole("admin"))
	{
		admin.POST("/clients", clientHandler.Create)
		admin.GET("/clients", clientHandler.List)
		admin.POST("/clients/:id/activate", clientHandler.Activate)
		admin.POST("/clients/:id/deactivate", clientHandler.Deactivate)
		admin.POST("/clients/:id/policy", clientHandler.UpdatePolicy)
		admin.POST("/clients/:id/webhook", clientHandler.UpdateWebhook)
		admin.POST("/clients/:id/api-key/rotate", clientHandler.RotateAPIKey)
		admin.GET("/moderation/results", moderationHandler.ListHistory)
		admin.GET("/webhook-deliveries", moderationHandler.ListWebhookDeliveries)
		admin.GET("/webhook-deliveries/:id", moderationHandler.GetWebhookDelivery)
		admin.POST("/webhook-deliveries/:id/retry", moderationHandler.RetryWebhookDelivery)
	}

	// Register metrics endpoint
	observability.RegisterMetricsEndpoint(r.engine)

	return r.engine
}

// corsMiddleware adds CORS support
func corsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Origin, Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, X-API-Key")
		c.Header("Access-Control-Expose-Headers", "X-RateLimit-Limit, X-RateLimit-Remaining, X-RateLimit-Reset, Retry-After")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	}
}

func clientRateLimitMiddleware(
	rateLimiter requestRateLimiter,
	cfg config.ModerationRateLimitConfig,
) gin.HandlerFunc {
	return func(c *gin.Context) {
		principal, exists := auth.GetAPIKeyPrincipal(c)
		if !exists {
			c.Next()
			return
		}
		if cfg.Limit <= 0 || cfg.Window <= 0 || rateLimiter == nil {
			c.Next()
			return
		}

		key := fmt.Sprintf("moderation:client:%d", principal.ClientID)
		state, err := rateLimiter.AllowWithState(c.Request.Context(), key, cfg.Limit, cfg.Window)
		if err != nil {
			apperrors.Handle(c, apperrors.Internal("Rate limit check failed").WithDetails(err.Error()))
			c.Abort()
			return
		}
		setRateLimitHeaders(c, state)
		if !state.Allowed {
			c.Header("Retry-After", strconv.Itoa(rateLimitRetryAfterSeconds(state.RetryAfter)))
			apperrors.Handle(c, apperrors.RateLimitExceeded("Client rate limit exceeded"))
			c.Abort()
			return
		}

		c.Next()
	}
}

func setRateLimitHeaders(c *gin.Context, state cache.RateLimitState) {
	if !state.Enforced {
		return
	}

	c.Header("X-RateLimit-Limit", strconv.Itoa(state.Limit))
	c.Header("X-RateLimit-Remaining", strconv.Itoa(state.Remaining))
	c.Header("X-RateLimit-Reset", strconv.FormatInt(rateLimitResetUnixSeconds(state.ResetAt), 10))
}

func rateLimitRetryAfterSeconds(duration time.Duration) int {
	if duration <= 0 {
		return 0
	}

	return int((duration + time.Second - 1) / time.Second)
}

func rateLimitResetUnixSeconds(resetAt time.Time) int64 {
	if resetAt.IsZero() {
		return 0
	}

	unix := resetAt.Unix()
	if resetAt.Nanosecond() > 0 {
		unix++
	}

	return unix
}

// GetEngine returns the gin engine
func (r *Router) GetEngine() *gin.Engine {
	return r.engine
}
