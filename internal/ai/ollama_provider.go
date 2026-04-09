package ai

import (
	"context"
	"encoding/base64"
	"fmt"
	"hatesentry/internal/config"
	"hatesentry/internal/errors"
	"net/url"
	"strings"
	"time"

	"github.com/ollama/ollama/api"
)

// OllamaProvider implements AI provider using Ollama
type OllamaProvider struct {
	client *api.Client
	cfg    *config.OllamaConfig
	model  string
}

// NewOllamaProvider creates a new Ollama provider
func NewOllamaProvider(cfg *config.OllamaConfig) *OllamaProvider {
	baseURL, _ := url.Parse(cfg.BaseURL)
	return &OllamaProvider{
		client: api.NewClient(baseURL, nil),
		cfg:    cfg,
		model:  cfg.Model,
	}
}

// buildChatOptions builds common chat request options
func (p *OllamaProvider) buildChatOptions() map[string]interface{} {
	return map[string]interface{}{
		"num_predict": p.cfg.MaxTokens,
		"temperature": p.cfg.Temperature,
	}
}

// DetectHateSpeech detects hate speech in text
func (p *OllamaProvider) DetectHateSpeech(ctx context.Context, req *DetectionRequest) (*DetectionResponse, error) {
	builder := NewPromptBuilder()
	var prompt string

	switch req.ContentType {
	case "image":
		prompt = builder.BuildImagePrompt(req.ImageURL)
	case "mixed":
		prompt = builder.BuildMultimodalPrompt(req.Content, req.ImageURL)
	default:
		prompt = builder.BuildTextPrompt(req.Content)
	}

	startTime := time.Now()

	messages := []api.Message{
		{
			Role:    "system",
			Content: builder.SystemPrompt,
		},
		{
			Role:    "user",
			Content: prompt,
		},
	}

	var finalResponse *api.ChatResponse
	err := p.client.Chat(ctx, &api.ChatRequest{
		Model:    p.model,
		Messages: messages,
		Options:  p.buildChatOptions(),
	}, func(resp api.ChatResponse) error {
		if resp.Done {
			finalResponse = &resp
		}
		return nil
	})

	if err != nil {
		return nil, errors.ExternalServiceError(err, "failed to create chat completion")
	}

	result, err := ParseAIResponse(finalResponse.Message.Content)
	if err != nil {
		return nil, errors.ValidationError("failed to parse AI response").WithDetails(err.Error())
	}

	result.RequestID = req.RequestID
	result.Model = p.model
	result.ProcessingTime = time.Since(startTime)
	result.PromptUsed = prompt
	result.RawResponse = finalResponse.Message.Content

	return result, nil
}

// DetectHateSpeechWithImage detects hate speech with image input
func (p *OllamaProvider) DetectHateSpeechWithImage(ctx context.Context, req *DetectionRequest, imageData []byte) (*DetectionResponse, error) {
	builder := NewPromptBuilder()

	// Encode image data to base64 to handle binary data safely
	imageBase64 := base64.StdEncoding.EncodeToString(imageData)

	var finalImageResponse *api.ChatResponse

	messages := []api.Message{
		{
			Role:    "system",
			Content: builder.SystemPrompt,
		},
		{
			Role: "user",
			Content: fmt.Sprintf("%s\n\n[Image data: %d bytes]",
				builder.BuildImagePrompt(req.ImageURL),
				len(imageData),
			),
			Images: []api.ImageData{[]byte(imageBase64)},
		},
	}

	startTime := time.Now()

	err := p.client.Chat(ctx, &api.ChatRequest{
		Model:    p.model,
		Messages: messages,
		Options:  p.buildChatOptions(),
	}, func(resp api.ChatResponse) error {
		if resp.Done {
			finalImageResponse = &resp
		}
		return nil
	})

	if err != nil {
		return nil, errors.ExternalServiceError(err, "failed to create chat completion with image")
	}

	result, err := ParseAIResponse(finalImageResponse.Message.Content)
	if err != nil {
		return nil, errors.ValidationError("failed to parse AI response").WithDetails(err.Error())
	}

	result.RequestID = req.RequestID
	result.Model = p.model
	result.ProcessingTime = time.Since(startTime)
	result.RawResponse = finalImageResponse.Message.Content

	return result, nil
}

// DetectHateSpeechWithStreaming detects hate speech with streaming response
func (p *OllamaProvider) DetectHateSpeechWithStreaming(ctx context.Context, req *DetectionRequest, callback func(event *StreamDetectionEvent)) (*DetectionResponse, error) {
	builder := NewPromptBuilder()
	prompt := builder.BuildTextPrompt(req.Content)

	callback(&StreamDetectionEvent{
		Type:     "start",
		Data:     map[string]string{"request_id": req.RequestID},
		Progress: 0.0,
	})

	messages := []api.Message{
		{
			Role:    "system",
			Content: builder.SystemPrompt,
		},
		{
			Role:    "user",
			Content: prompt,
		},
	}

	startTime := time.Now()
	// Pre-allocate builder with estimated capacity to reduce memory allocations
	var fullResponse strings.Builder
	fullResponse.Grow(p.cfg.MaxTokens * 4) // Estimate 4 bytes per token
	tokenCount := 0

	respFunc := func(resp api.ChatResponse) error {
		if resp.Done {
			return nil
		}

		if len(resp.Message.Content) > 0 {
			fullResponse.WriteString(resp.Message.Content)
			tokenCount++

			// Calculate progress based on token count, cap at 95% until completion
			progress := float64(tokenCount) / float64(p.cfg.MaxTokens)
			if progress > 0.95 {
				progress = 0.95
			}

			callback(&StreamDetectionEvent{
				Type:     "progress",
				Data:     map[string]string{"content": resp.Message.Content},
				Progress: progress,
			})
		}

		return nil
	}

	options := p.buildChatOptions()
	options["stream"] = true

	err := p.client.Chat(ctx, &api.ChatRequest{
		Model:    p.model,
		Messages: messages,
		Options:  options,
	}, respFunc)

	if err != nil {
		callback(&StreamDetectionEvent{
			Type: "error",
			Data: map[string]string{"error": err.Error()},
		})
		return nil, errors.ExternalServiceError(err, "stream error")
	}

	result, err := ParseAIResponse(fullResponse.String())
	if err != nil {
		return nil, errors.ValidationError("failed to parse AI response").WithDetails(err.Error())
	}

	result.RequestID = req.RequestID
	result.Model = p.model
	result.ProcessingTime = time.Since(startTime)
	result.PromptUsed = prompt
	result.RawResponse = fullResponse.String()

	callback(&StreamDetectionEvent{
		Type:     "result",
		Data:     result,
		Progress: 1.0,
	})

	return result, nil
}

// GetModel returns the model name
func (p *OllamaProvider) GetModel() string {
	return p.model
}
