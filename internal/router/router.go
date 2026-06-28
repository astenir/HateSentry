package router

import (
	"hatesentry/internal/ai"
	"hatesentry/internal/auth"
	"hatesentry/internal/cache"
	"hatesentry/internal/clients"
	"hatesentry/internal/handlers"
	"hatesentry/internal/moderation"
	"hatesentry/internal/observability"
	"hatesentry/internal/queue"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// Router represents the HTTP router
type Router struct {
	engine           *gin.Engine
	detectionService *ai.DetectionService
	publisher        queue.Publisher
	rabbitMQManager  *queue.RabbitMQManager
	cache            *cache.DetectionCache
	rateLimiter      *cache.RateLimiter
	jwtManager       *auth.JWTManager
	db               *gorm.DB
	moderationPolicy moderation.Policy
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
) *Router {
	return &Router{
		engine:           gin.New(),
		db:               db,
		detectionService: detectionService,
		publisher:        publisher,
		rabbitMQManager:  rabbitMQManager,
		cache:            cache,
		rateLimiter:      rateLimiter,
		jwtManager:       jwtManager,
		moderationPolicy: moderationPolicy,
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
	clientService := clients.NewService(clientRepository)
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
	moderationService := moderation.NewService(
		r.detectionService,
		moderation.NewGormRepository(r.db),
		r.moderationPolicy,
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

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	}
}

// GetEngine returns the gin engine
func (r *Router) GetEngine() *gin.Engine {
	return r.engine
}
