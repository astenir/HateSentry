package app

import (
	"context"
	"fmt"
	"hatesentry/internal/ai"
	"hatesentry/internal/auth"
	"hatesentry/internal/cache"
	"hatesentry/internal/config"
	"hatesentry/internal/database"
	"hatesentry/internal/errors"
	"hatesentry/internal/handlers"
	"hatesentry/internal/models"
	"hatesentry/internal/moderation"
	"hatesentry/internal/queue"
	"hatesentry/internal/router"
	"hatesentry/internal/webhooks"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// App represents the application
type App struct {
	config           *config.Config
	router           *router.Router
	rabbitMQManager  *queue.RabbitMQManager
	detectionService *ai.DetectionService
	consumer         *queue.Consumer
	detectionHandler *handlers.DetectionHandler
	logger           *zap.Logger
}

// HandleDetectionTask implements queue.DetectionHandler interface
func (a *App) HandleDetectionTask(ctx context.Context, task *queue.Task) error {
	a.logger.Info("Processing detection task", zap.String("request_id", task.RequestID))

	// Find the detection request
	var req models.DetectionRequest
	if err := database.GetDB().Where("request_id = ?", task.RequestID).First(&req).Error; err != nil {
		return errors.DatabaseError(err, "failed to find detection request")
	}

	// Update status
	req.Status = "processing"
	database.GetDB().Save(&req)

	// Perform detection
	aiReq := &ai.DetectionRequest{
		RequestID:   task.RequestID,
		Content:     task.Content,
		ImageURL:    task.ImageURL,
		ContentType: task.ContentType,
	}

	resp, err := a.detectionService.Detect(ctx, aiReq)
	if err != nil {
		req.Status = "failed"
		database.GetDB().Save(&req)
		return errors.DatabaseError(err, "detection failed")
	}

	// Save result
	result := a.detectionService.ConvertToModel(resp)
	if err := database.GetDB().Create(result).Error; err != nil {
		return errors.DatabaseError(err, "failed to save detection result")
	}

	// Update request status
	req.Processed = true
	req.Status = "completed"
	if err := database.GetDB().Save(&req).Error; err != nil {
		return errors.DatabaseError(err, "failed to update detection request")
	}

	// Cache result
	detectionCache := cache.NewDetectionCache(a.config.Detection.ResultCacheTTL)
	detectionCache.SetDetectionResult(ctx, result)

	a.logger.Info("Detection task completed", zap.String("request_id", task.RequestID))
	return nil
}

// NewApp creates a new application instance
func NewApp() *App {
	return &App{}
}

// Run starts the application
func (a *App) Run() error {
	// Load configuration
	cfg, err := config.Load("config/config.yaml")
	if err != nil {
		return errors.ConfigurationError("failed to load config").WithDetails(err.Error())
	}
	a.config = cfg

	moderationPolicies, err := moderationPolicySetFromConfig(cfg.Moderation)
	if err != nil {
		return errors.ConfigurationError("invalid moderation policy").WithDetails(err.Error())
	}

	// Initialize logger
	if err := a.initLogger(); err != nil {
		return errors.ConfigurationError("failed to initialize logger").WithDetails(err.Error())
	}

	a.logger.Info("Starting HateSentry application")

	// Initialize database
	if err := database.Initialize(&cfg.Database); err != nil {
		return errors.DatabaseError(err, "failed to initialize database")
	}
	defer database.Close()

	// Initialize Redis
	if err := cache.Initialize(&cfg.Redis); err != nil {
		return errors.ExternalServiceError(err, "failed to initialize Redis")
	}
	defer cache.Close()

	// Initialize RabbitMQ
	rabbitMQManager, err := queue.NewRabbitMQManager(&cfg.RabbitMQ)
	if err != nil {
		return errors.ExternalServiceError(err, "failed to initialize RabbitMQ")
	}
	a.rabbitMQManager = rabbitMQManager
	defer rabbitMQManager.Close()

	// Initialize detection service
	detectionService, err := ai.NewDetectionService(&cfg.AI, &cfg.Detection)
	if err != nil {
		return errors.ConfigurationError("failed to initialize detection service").WithDetails(err.Error())
	}
	a.detectionService = detectionService

	// Initialize cache components
	detectionCache := cache.NewDetectionCache(cfg.Detection.ResultCacheTTL)
	rateLimiter := cache.NewRateLimiter()

	// Initialize JWT manager
	jwtManager := auth.NewJWTManager(&cfg.JWT)

	// Initialize publisher (use RabbitMQManager directly)
	publisher := rabbitMQManager

	// Initialize detection handler (for HTTP endpoints)
	detectionHandler := handlers.NewDetectionHandler(
		database.GetDB(),
		detectionService,
		publisher,
		detectionCache,
		rateLimiter,
		jwtManager,
	)
	a.detectionHandler = detectionHandler

	// Initialize consumer (use app itself as handler)
	a.consumer = queue.NewConsumer(rabbitMQManager, a)

	// Initialize router
	r := router.NewRouterWithPolicies(
		database.GetDB(),
		detectionService,
		publisher,
		rabbitMQManager,
		detectionCache,
		rateLimiter,
		jwtManager,
		moderationPolicies,
		cfg.Moderation.ClientRateLimit,
	)
	r.SetAuthConfig(cfg.Auth)
	a.router = r

	// Start consumer in background
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	webhookRetryService := moderation.NewServiceWithPolicySet(
		detectionService,
		moderation.NewGormRepository(database.GetDB()),
		moderationPolicies,
		webhooks.NewHTTPDispatcher(),
	)
	startWebhookRetryWorker(ctx, a.logger, webhookRetryService, cfg.Moderation.WebhookRetry)

	go func() {
		a.logger.Info("Starting detection consumer...")
		if err := a.consumer.Start(ctx); err != nil {
			a.logger.Error("Consumer failed", zap.Error(err))
		}
	}()

	// Setup routes
	engine := r.Setup()

	// Configure Gin mode
	gin.SetMode(cfg.Server.Mode)

	// Create HTTP server
	server := &http.Server{
		Addr:         fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port),
		Handler:      engine,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
	}

	// Start server in goroutine
	go func() {
		a.logger.Info("Server starting",
			zap.String("address", server.Addr),
			zap.String("mode", cfg.Server.Mode),
		)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			a.logger.Fatal("Server failed to start", zap.Error(err))
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	a.logger.Info("Shutting down server...")

	// Graceful shutdown
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		a.logger.Error("Server forced to shutdown", zap.Error(err))
	}

	a.logger.Info("Server exited")
	return nil
}

func moderationPolicySetFromConfig(cfg config.ModerationConfig) (moderation.PolicySet, error) {
	defaultPolicy, err := moderation.NewPolicy(
		cfg.Policy.Version,
		cfg.Policy.ReviewThreshold,
		cfg.Policy.BlockThreshold,
	)
	if err != nil {
		return moderation.PolicySet{}, err
	}

	policies := make([]moderation.Policy, 0, len(cfg.Policies))
	for _, policyConfig := range cfg.Policies {
		policy, err := moderation.NewPolicy(
			policyConfig.Version,
			policyConfig.ReviewThreshold,
			policyConfig.BlockThreshold,
		)
		if err != nil {
			return moderation.PolicySet{}, err
		}
		policies = append(policies, policy)
	}

	return moderation.NewPolicySet(defaultPolicy, policies...)
}

type webhookRetryService interface {
	RetryFailedWebhookDeliveries(
		ctx context.Context,
		input moderation.WebhookRetryInput,
	) (moderation.WebhookRetryOutput, error)
}

func startWebhookRetryWorker(
	ctx context.Context,
	logger *zap.Logger,
	service webhookRetryService,
	cfg config.WebhookRetryConfig,
) bool {
	if !cfg.Enabled || service == nil || cfg.Interval <= 0 || cfg.BatchSize <= 0 || cfg.MaxAttempts <= 1 {
		return false
	}
	if logger == nil {
		logger = zap.NewNop()
	}

	go func() {
		ticker := time.NewTicker(cfg.Interval)
		defer ticker.Stop()

		runWebhookRetryBatch(ctx, logger, service, cfg)
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				runWebhookRetryBatch(ctx, logger, service, cfg)
			}
		}
	}()

	return true
}

func runWebhookRetryBatch(
	ctx context.Context,
	logger *zap.Logger,
	service webhookRetryService,
	cfg config.WebhookRetryConfig,
) {
	if logger == nil {
		logger = zap.NewNop()
	}
	output, err := service.RetryFailedWebhookDeliveries(ctx, moderation.WebhookRetryInput{
		Limit:       cfg.BatchSize,
		MaxAttempts: cfg.MaxAttempts,
	})
	if err != nil {
		logger.Warn("automatic webhook retry batch failed", zap.Error(err))
		return
	}
	if output.Attempted == 0 && output.Failed == 0 && output.Skipped == 0 {
		return
	}

	logger.Info(
		"automatic webhook retry batch completed",
		zap.Int("attempted", output.Attempted),
		zap.Int("succeeded", output.Succeeded),
		zap.Int("failed", output.Failed),
		zap.Int("skipped", output.Skipped),
	)
}

// initLogger initializes the logger
func (a *App) initLogger() error {
	var zapConfig zap.Config

	if a.config.Logging.Format == "json" {
		zapConfig = zap.NewProductionConfig()
	} else {
		zapConfig = zap.NewDevelopmentConfig()
	}

	// Set log level
	switch a.config.Logging.Level {
	case "debug":
		zapConfig.Level = zap.NewAtomicLevelAt(zap.DebugLevel)
	case "info":
		zapConfig.Level = zap.NewAtomicLevelAt(zap.InfoLevel)
	case "warn":
		zapConfig.Level = zap.NewAtomicLevelAt(zap.WarnLevel)
	case "error":
		zapConfig.Level = zap.NewAtomicLevelAt(zap.ErrorLevel)
	}

	logger, err := zapConfig.Build()
	if err != nil {
		return errors.ConfigurationError("failed to build logger").WithDetails(err.Error())
	}

	a.logger = logger
	zap.ReplaceGlobals(logger)
	return nil
}
