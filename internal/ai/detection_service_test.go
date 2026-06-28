package ai

import (
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
