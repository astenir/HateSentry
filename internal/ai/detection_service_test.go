package ai

import (
	"context"
	"strings"
	"testing"

	"hatesentry/internal/config"
)

func TestNewDetectionServiceSelectsOllamaProvider(t *testing.T) {
	service, err := NewDetectionService(&config.AIConfig{
		Provider: "ollama",
		Ollama: config.OllamaConfig{
			BaseURL:     "http://localhost:11434",
			Model:       "llama3",
			MaxTokens:   1000,
			Temperature: 0.3,
		},
	}, &config.DetectionConfig{
		EnableTextAnalysis: true,
	})
	if err != nil {
		t.Fatalf("NewDetectionService() error = %v", err)
	}

	provider, ok := service.GetProvider().(*OllamaProvider)
	if !ok {
		t.Fatalf("provider = %T, want *OllamaProvider", service.GetProvider())
	}
	if provider.GetModel() != "llama3" {
		t.Fatalf("model = %q, want llama3", provider.GetModel())
	}
	if _, ok := service.GetProvider().(textModerationProvider); !ok {
		t.Fatalf("provider = %T, want textModerationProvider", service.GetProvider())
	}
}

func TestNewDetectionServiceRejectsUnsupportedProvider(t *testing.T) {
	_, err := NewDetectionService(&config.AIConfig{
		Provider: "unknown",
	}, &config.DetectionConfig{
		EnableTextAnalysis: true,
	})
	if err == nil {
		t.Fatal("NewDetectionService() error = nil, want unsupported provider error")
	}
	if !strings.Contains(err.Error(), "unsupported AI provider: unknown") {
		t.Fatalf("NewDetectionService() error = %q, want unsupported provider detail", err.Error())
	}
}

func TestDetectionServiceRejectsImageRequestsWhenImageAnalysisDisabled(t *testing.T) {
	tests := []struct {
		name string
		req  DetectionRequest
		run  func(context.Context, *DetectionService, *DetectionRequest) error
	}{
		{
			name: "sync image",
			req: DetectionRequest{
				RequestID:   "req-image",
				ImageURL:    "https://example.com/image.jpg",
				ContentType: "image",
			},
			run: func(ctx context.Context, service *DetectionService, req *DetectionRequest) error {
				_, err := service.Detect(ctx, req)
				return err
			},
		},
		{
			name: "sync mixed",
			req: DetectionRequest{
				RequestID:   "req-mixed",
				Content:     "text with an image",
				ImageURL:    "https://example.com/image.jpg",
				ContentType: "mixed",
			},
			run: func(ctx context.Context, service *DetectionService, req *DetectionRequest) error {
				_, err := service.Detect(ctx, req)
				return err
			},
		},
		{
			name: "streaming image",
			req: DetectionRequest{
				RequestID:   "req-stream-image",
				ImageURL:    "https://example.com/image.jpg",
				ContentType: "image",
			},
			run: func(ctx context.Context, service *DetectionService, req *DetectionRequest) error {
				_, err := service.DetectWithStreaming(ctx, req, func(event *StreamDetectionEvent) {})
				return err
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider := &recordingDetectionProvider{}
			service := &DetectionService{
				provider: provider,
				cfg: &config.DetectionConfig{
					EnableTextAnalysis:  true,
					EnableImageAnalysis: false,
				},
			}

			err := tt.run(context.Background(), service, &tt.req)
			if err == nil {
				t.Fatal("detection error = nil, want image analysis disabled error")
			}
			if !strings.Contains(err.Error(), "image analysis is disabled") {
				t.Fatalf("detection error = %q, want image analysis disabled", err.Error())
			}
			if provider.detectCalled || provider.streamCalled {
				t.Fatal("provider was called, want validation to reject before provider dispatch")
			}
		})
	}
}

type recordingDetectionProvider struct {
	detectCalled bool
	streamCalled bool
}

func (p *recordingDetectionProvider) DetectHateSpeech(_ context.Context, req *DetectionRequest) (*DetectionResponse, error) {
	p.detectCalled = true

	return &DetectionResponse{
		RequestID: req.RequestID,
		Model:     p.GetModel(),
	}, nil
}

func (p *recordingDetectionProvider) DetectHateSpeechWithImage(
	_ context.Context,
	req *DetectionRequest,
	_ []byte,
) (*DetectionResponse, error) {
	return &DetectionResponse{
		RequestID: req.RequestID,
		Model:     p.GetModel(),
	}, nil
}

func (p *recordingDetectionProvider) DetectHateSpeechWithStreaming(
	_ context.Context,
	req *DetectionRequest,
	_ func(event *StreamDetectionEvent),
) (*DetectionResponse, error) {
	p.streamCalled = true

	return &DetectionResponse{
		RequestID: req.RequestID,
		Model:     p.GetModel(),
	}, nil
}

func (p *recordingDetectionProvider) GetModel() string {
	return "test-model"
}
