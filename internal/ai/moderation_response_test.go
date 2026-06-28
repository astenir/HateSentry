package ai

import (
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/ollama/ollama/api"

	appErrors "hatesentry/internal/errors"
	"hatesentry/internal/moderation"
)

func TestParseModerationProviderOutputMapsMalformedJSONToExternalServiceError(t *testing.T) {
	_, err := parseModerationProviderOutput("not json")
	if err == nil {
		t.Fatal("parseModerationProviderOutput() error = nil, want error")
	}

	var appErr *appErrors.AppError
	if !errors.As(err, &appErr) {
		t.Fatalf("parseModerationProviderOutput() error = %T, want AppError", err)
	}
	if appErr.Code != appErrors.ErrCodeServiceUnavailable {
		t.Fatalf("Code = %q, want %q", appErr.Code, appErrors.ErrCodeServiceUnavailable)
	}
	if appErr.HTTPStatus != http.StatusBadGateway {
		t.Fatalf("HTTPStatus = %d, want %d", appErr.HTTPStatus, http.StatusBadGateway)
	}
}

func TestParseModerationProviderOutputPreservesRawOutput(t *testing.T) {
	rawOutput := `{"risk_score":0.6,"labels":["harassment"],"reason":"contains abusive language"}`

	suggestion, err := parseModerationProviderOutput(rawOutput)
	if err != nil {
		t.Fatalf("parseModerationProviderOutput() error = %v", err)
	}
	if suggestion.RiskScore != 0.6 {
		t.Fatalf("RiskScore = %v, want 0.6", suggestion.RiskScore)
	}
	if suggestion.RawOutput != rawOutput {
		t.Fatalf("RawOutput = %q, want %q", suggestion.RawOutput, rawOutput)
	}
}

func TestAppendOllamaChatContentAccumulatesNonFinalChunks(t *testing.T) {
	var builder strings.Builder

	appendOllamaChatContent(&builder, api.ChatResponse{
		Message: api.Message{Content: `{"risk_score":0.6,`},
	})
	appendOllamaChatContent(&builder, api.ChatResponse{
		Message: api.Message{Content: `"labels":["harassment"],`},
	})
	appendOllamaChatContent(&builder, api.ChatResponse{
		Message: api.Message{Content: `"reason":"contains abusive language"}`},
		Done:    true,
	})

	suggestion, err := moderation.ParseProviderSuggestion(builder.String())
	if err != nil {
		t.Fatalf("ParseProviderSuggestion() error = %v", err)
	}
	if suggestion.RiskScore != 0.6 {
		t.Fatalf("RiskScore = %v, want 0.6", suggestion.RiskScore)
	}
}
