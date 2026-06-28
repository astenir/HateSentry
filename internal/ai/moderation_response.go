package ai

import (
	"strings"

	"github.com/ollama/ollama/api"

	"hatesentry/internal/errors"
	"hatesentry/internal/moderation"
)

func parseModerationProviderOutput(rawOutput string) (moderation.ProviderSuggestion, error) {
	suggestion, err := moderation.ParseProviderSuggestion(rawOutput)
	if err != nil {
		return moderation.ProviderSuggestion{}, errors.ExternalServiceError(
			err,
			"failed to parse moderation response",
		)
	}

	return suggestion, nil
}

func appendOllamaChatContent(builder *strings.Builder, resp api.ChatResponse) {
	if resp.Message.Content == "" {
		return
	}

	builder.WriteString(resp.Message.Content)
}
