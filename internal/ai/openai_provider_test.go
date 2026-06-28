package ai

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"hatesentry/internal/config"
)

func TestOpenAIProviderAnalyzeTextModerationRequestsJSONObject(t *testing.T) {
	var captured openAIChatCompletionRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			t.Fatalf("path = %q, want /v1/chat/completions", r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&captured); err != nil {
			t.Fatalf("decode request body: %v", err)
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"id": "chatcmpl-test",
			"object": "chat.completion",
			"created": 1782648000,
			"model": "gpt-test",
			"choices": [
				{
					"index": 0,
					"message": {
						"role": "assistant",
						"content": "{\"risk_score\":0.82,\"labels\":[\"harassment\"],\"reason\":\"Contains targeted abuse.\"}"
					},
					"finish_reason": "stop"
				}
			]
		}`))
	}))
	defer server.Close()

	provider := NewOpenAIProvider(&config.OpenAIConfig{
		APIKey:      "test-key",
		BaseURL:     server.URL + "/v1",
		Model:       "gpt-test",
		MaxTokens:   500,
		Temperature: 0.2,
	})

	suggestion, info, err := provider.AnalyzeTextModeration(context.Background(), "check this text")
	if err != nil {
		t.Fatalf("AnalyzeTextModeration() error = %v", err)
	}

	if captured.ResponseFormat == nil {
		t.Fatal("response_format is nil")
	}
	if captured.ResponseFormat.Type != "json_object" {
		t.Fatalf("response_format.type = %q, want json_object", captured.ResponseFormat.Type)
	}
	if captured.Model != "gpt-test" {
		t.Fatalf("model = %q, want gpt-test", captured.Model)
	}
	if len(captured.Messages) != 2 {
		t.Fatalf("messages = %d, want 2", len(captured.Messages))
	}
	if suggestion.RiskScore != 0.82 {
		t.Fatalf("RiskScore = %v, want 0.82", suggestion.RiskScore)
	}
	if !equalStringSlices(suggestion.Labels, []string{"harassment"}) {
		t.Fatalf("Labels = %#v, want harassment", suggestion.Labels)
	}
	if suggestion.RawOutput == "" {
		t.Fatal("RawOutput is empty")
	}
	if info.Provider != "openai" {
		t.Fatalf("provider = %q, want openai", info.Provider)
	}
	if info.Model != "gpt-test" {
		t.Fatalf("info model = %q, want gpt-test", info.Model)
	}
}

type openAIChatCompletionRequest struct {
	Model          string                    `json:"model"`
	Messages       []openAIChatMessage       `json:"messages"`
	ResponseFormat *openAIChatResponseFormat `json:"response_format"`
}

type openAIChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type openAIChatResponseFormat struct {
	Type string `json:"type"`
}

func equalStringSlices(a []string, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
