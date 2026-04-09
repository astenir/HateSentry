package ai

import (
	"context"
	"encoding/base64"
	"hatesentry/internal/config"
	"hatesentry/internal/errors"
	"io"
	"strings"
	"time"

	"github.com/sashabaranov/go-openai"
)

// OpenAIProvider implements AI provider using OpenAI API
type OpenAIProvider struct {
	client    *openai.Client
	cfg       *config.OpenAIConfig
	model     string
}

// NewOpenAIProvider creates a new OpenAI provider
func NewOpenAIProvider(cfg *config.OpenAIConfig) *OpenAIProvider {
	var client *openai.Client
	if cfg.BaseURL != "" {
		// Custom base URL for proxy or custom endpoint
		config := openai.DefaultConfig(cfg.APIKey)
		config.BaseURL = cfg.BaseURL
		client = openai.NewClientWithConfig(config)
	} else {
		client = openai.NewClient(cfg.APIKey)
	}

	return &OpenAIProvider{
		client: client,
		cfg:    cfg,
		model:  cfg.Model,
	}
}

// DetectHateSpeech detects hate speech in text
func (p *OpenAIProvider) DetectHateSpeech(ctx context.Context, req *DetectionRequest) (*DetectionResponse, error) {
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

	messages := []openai.ChatCompletionMessage{
		{
			Role:    openai.ChatMessageRoleSystem,
			Content: builder.SystemPrompt,
		},
		{
			Role:    openai.ChatMessageRoleUser,
			Content: prompt,
		},
	}

	resp, err := p.client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
		Model:       p.model,
		Messages:    messages,
		MaxTokens:   p.cfg.MaxTokens,
		Temperature: float32(p.cfg.Temperature),
	})

	if err != nil {
		return nil, errors.ExternalServiceError(err, "failed to create chat completion")
	}

	if len(resp.Choices) == 0 {
		return nil, errors.ExternalServiceError(nil, "no response from OpenAI")
	}

	result, err := ParseAIResponse(resp.Choices[0].Message.Content)
	if err != nil {
		return nil, errors.ValidationError("failed to parse AI response").WithDetails(err.Error())
	}

	result.RequestID = req.RequestID
	result.Model = p.model
	result.ProcessingTime = time.Since(startTime)
	result.PromptUsed = prompt
	result.RawResponse = resp.Choices[0].Message.Content

	return result, nil
}

// DetectHateSpeechWithImage detects hate speech with image input
func (p *OpenAIProvider) DetectHateSpeechWithImage(ctx context.Context, req *DetectionRequest, imageData []byte) (*DetectionResponse, error) {
	builder := NewPromptBuilder()
	base64Image := "data:image/jpeg;base64," + base64.StdEncoding.EncodeToString(imageData)

	messages := []openai.ChatCompletionMessage{
		{
			Role:    openai.ChatMessageRoleSystem,
			Content: builder.SystemPrompt,
		},
		{
			Role: openai.ChatMessageRoleUser,
			MultiContent: []openai.ChatMessagePart{
				{
					Type: openai.ChatMessagePartTypeText,
					Text: builder.BuildImagePrompt(req.ImageURL),
				},
				{
					Type: openai.ChatMessagePartTypeImageURL,
					ImageURL: &openai.ChatMessageImageURL{
						URL: base64Image,
					},
				},
			},
		},
	}

	startTime := time.Now()

	resp, err := p.client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
		Model:       p.model,
		Messages:    messages,
		MaxTokens:   p.cfg.MaxTokens,
		Temperature: float32(p.cfg.Temperature),
	})

	if err != nil {
		return nil, errors.ExternalServiceError(err, "failed to create chat completion with image")
	}

	if len(resp.Choices) == 0 {
		return nil, errors.ExternalServiceError(nil, "no response from OpenAI")
	}

	result, err := ParseAIResponse(resp.Choices[0].Message.Content)
	if err != nil {
		return nil, errors.ValidationError("failed to parse AI response").WithDetails(err.Error())
	}

	result.RequestID = req.RequestID
	result.Model = p.model
	result.ProcessingTime = time.Since(startTime)
	result.RawResponse = resp.Choices[0].Message.Content

	return result, nil
}

// DetectHateSpeechWithStreaming detects hate speech with streaming response
func (p *OpenAIProvider) DetectHateSpeechWithStreaming(ctx context.Context, req *DetectionRequest, callback func(event *StreamDetectionEvent)) (*DetectionResponse, error) {
	builder := NewPromptBuilder().WithStrategy(StrategyChainOfThought)
	prompt := builder.BuildTextPrompt(req.Content)

	callback(&StreamDetectionEvent{
		Type:     "start",
		Data:     map[string]string{"request_id": req.RequestID},
		Progress: 0.0,
	})

	stream, err := p.client.CreateChatCompletionStream(ctx, openai.ChatCompletionRequest{
		Model:       p.model,
		Messages: []openai.ChatCompletionMessage{
			{
				Role:    openai.ChatMessageRoleSystem,
				Content: builder.SystemPrompt,
			},
			{
				Role:    openai.ChatMessageRoleUser,
				Content: prompt,
			},
		},
		MaxTokens:   p.cfg.MaxTokens,
		Temperature: float32(p.cfg.Temperature),
	})

	if err != nil {
		callback(&StreamDetectionEvent{
			Type: "error",
			Data: map[string]string{"error": err.Error()},
		})
		return nil, errors.ExternalServiceError(err, "failed to create stream")
	}

	defer stream.Close()

	var fullResponse strings.Builder
	tokenCount := 0

	for {
		resp, err := stream.Recv()
		if err != nil {
			if err == io.EOF {
				break
			}
			callback(&StreamDetectionEvent{
				Type: "error",
				Data: map[string]string{"error": err.Error()},
			})
			return nil, errors.ExternalServiceError(err, "stream error")
		}

		if len(resp.Choices) > 0 {
			content := resp.Choices[0].Delta.Content
			fullResponse.WriteString(content)
			tokenCount++

			// Send progress updates
			callback(&StreamDetectionEvent{
				Type:     "progress",
				Data:     map[string]string{"content": content},
				Progress: float64(tokenCount) / float64(p.cfg.MaxTokens),
			})
		}
	}

	result, err := ParseAIResponse(fullResponse.String())
	if err != nil {
		return nil, errors.ValidationError("failed to parse AI response").WithDetails(err.Error())
	}

	result.RequestID = req.RequestID
	result.Model = p.model
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
func (p *OpenAIProvider) GetModel() string {
	return p.model
}
