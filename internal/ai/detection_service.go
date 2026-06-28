package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"hatesentry/internal/config"
	"hatesentry/internal/errors"
	"hatesentry/internal/models"
	"hatesentry/internal/moderation"
	"strings"
)

// Provider defines the interface for AI providers
type Provider interface {
	DetectHateSpeech(ctx context.Context, req *DetectionRequest) (*DetectionResponse, error)
	DetectHateSpeechWithImage(ctx context.Context, req *DetectionRequest, imageData []byte) (*DetectionResponse, error)
	DetectHateSpeechWithStreaming(ctx context.Context, req *DetectionRequest, callback func(event *StreamDetectionEvent)) (*DetectionResponse, error)
	GetModel() string
}

type textModerationProvider interface {
	AnalyzeTextModeration(ctx context.Context, content string) (moderation.ProviderSuggestion, moderation.ProviderInfo, error)
}

// DetectionService manages hate speech detection
type DetectionService struct {
	provider Provider
	cfg      *config.DetectionConfig
}

// NewDetectionService creates a new detection service
func NewDetectionService(cfg *config.AIConfig, detectionCfg *config.DetectionConfig) (*DetectionService, error) {
	var provider Provider

	switch cfg.Provider {
	case "ollama":
		// TODO: Ollama provider is temporarily disabled due to dependency issues
		// To enable, use: go get github.com/ollama/ollama-go/v0
		provider = NewOllamaProvider(&cfg.Ollama)
		// return nil, errors.BadRequest("ollama provider is temporarily disabled. Please use 'openai' provider")
	case "openai", "":
		provider = NewOpenAIProvider(&cfg.OpenAI)
	default:
		return nil, errors.BadRequest(fmt.Sprintf("unsupported AI provider: %s", cfg.Provider))
	}

	return &DetectionService{
		provider: provider,
		cfg:      detectionCfg,
	}, nil
}

// AnalyzeText classifies text using the configured provider and returns a normalized moderation suggestion.
func (s *DetectionService) AnalyzeText(
	ctx context.Context,
	content string,
) (moderation.ProviderSuggestion, moderation.ProviderInfo, error) {
	if s == nil || s.provider == nil {
		return moderation.ProviderSuggestion{}, moderation.ProviderInfo{}, errors.ConfigurationError("AI provider is not configured")
	}
	if strings.TrimSpace(content) == "" {
		return moderation.ProviderSuggestion{}, moderation.ProviderInfo{}, errors.ValidationError("content is required")
	}

	provider, ok := s.provider.(textModerationProvider)
	if !ok {
		return moderation.ProviderSuggestion{}, moderation.ProviderInfo{}, errors.ConfigurationError("AI provider does not support text moderation")
	}

	return provider.AnalyzeTextModeration(ctx, content)
}

// Detect performs hate speech detection
func (s *DetectionService) Detect(ctx context.Context, req *DetectionRequest) (*DetectionResponse, error) {
	// Validate request content
	if req.Content == "" && req.ImageURL == "" {
		return nil, errors.ValidationError("content or image_url must be provided")
	}

	if !s.cfg.EnableTextAnalysis && !s.cfg.EnableImageAnalysis {
		return nil, errors.ConfigurationError("both text and image analysis are disabled")
	}

	// Validate based on content type
	if req.ContentType == "image" && req.ImageURL == "" {
		return nil, errors.ValidationError("image_url required for image analysis")
	}

	return s.provider.DetectHateSpeech(ctx, req)
}

// DetectWithImage performs hate speech detection with image data
func (s *DetectionService) DetectWithImage(ctx context.Context, req *DetectionRequest, imageData []byte) (*DetectionResponse, error) {
	if !s.cfg.EnableImageAnalysis {
		return nil, errors.ConfigurationError("image analysis is disabled")
	}

	return s.provider.DetectHateSpeechWithImage(ctx, req, imageData)
}

// DetectWithStreaming performs hate speech detection with streaming
func (s *DetectionService) DetectWithStreaming(ctx context.Context, req *DetectionRequest, callback func(event *StreamDetectionEvent)) (*DetectionResponse, error) {
	return s.provider.DetectHateSpeechWithStreaming(ctx, req, callback)
}

// ConvertToModel converts DetectionResponse to models.DetectionResult
func (s *DetectionService) ConvertToModel(resp *DetectionResponse) *models.DetectionResult {
	categoriesJSON, _ := json.Marshal(resp.Categories)

	return &models.DetectionResult{
		RequestID:      resp.RequestID,
		IsHateSpeech:   resp.IsHateSpeech,
		Confidence:     resp.Confidence,
		Categories:     string(categoriesJSON),
		Explanation:    resp.Explanation,
		Model:          resp.Model,
		ProcessingTime: resp.ProcessingTime.Milliseconds(),
		PromptUsed:     resp.PromptUsed,
		RawResponse:    resp.RawResponse,
	}
}

// GetProvider returns the current AI provider
func (s *DetectionService) GetProvider() Provider {
	return s.provider
}

// GetModel returns the current model name
func (s *DetectionService) GetModel() string {
	return s.provider.GetModel()
}
