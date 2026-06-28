package moderation

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

type providerSuggestionJSON struct {
	RiskScore *float64 `json:"risk_score"`
	Labels    []string `json:"labels"`
	Reason    *string  `json:"reason"`
}

// ParseProviderSuggestion parses the strict JSON object expected from an AI provider.
func ParseProviderSuggestion(raw string) (ProviderSuggestion, error) {
	if strings.TrimSpace(raw) == "" {
		return ProviderSuggestion{}, fmt.Errorf("provider output is required")
	}

	decoder := json.NewDecoder(strings.NewReader(raw))
	decoder.DisallowUnknownFields()

	var parsed providerSuggestionJSON
	if err := decoder.Decode(&parsed); err != nil {
		return ProviderSuggestion{}, fmt.Errorf("parse provider output: %w", err)
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return ProviderSuggestion{}, fmt.Errorf("provider output must contain a single JSON object")
	}

	if parsed.RiskScore == nil {
		return ProviderSuggestion{}, fmt.Errorf("risk_score is required")
	}
	if parsed.Reason == nil {
		return ProviderSuggestion{}, fmt.Errorf("reason is required")
	}

	labels, err := normalizeLabels(parsed.Labels)
	if err != nil {
		return ProviderSuggestion{}, err
	}
	if *parsed.RiskScore < 0 || *parsed.RiskScore > 1 {
		return ProviderSuggestion{}, fmt.Errorf("risk_score must be between 0 and 1")
	}
	if strings.TrimSpace(*parsed.Reason) == "" {
		return ProviderSuggestion{}, fmt.Errorf("reason is required")
	}

	return ProviderSuggestion{
		RiskScore: *parsed.RiskScore,
		Labels:    labels,
		Reason:    strings.TrimSpace(*parsed.Reason),
		RawOutput: raw,
	}, nil
}

func normalizeLabels(labels []string) ([]string, error) {
	if len(labels) == 0 {
		return nil, fmt.Errorf("labels must not be empty")
	}

	normalized := make([]string, 0, len(labels))
	seen := map[string]bool{}
	for _, label := range labels {
		label = strings.ToLower(strings.TrimSpace(label))
		if label == "" {
			return nil, fmt.Errorf("labels must not contain empty values")
		}
		if !IsSupportedLabel(label) {
			return nil, fmt.Errorf("unsupported label %q", label)
		}
		if seen[label] {
			continue
		}

		seen[label] = true
		normalized = append(normalized, label)
	}

	return normalized, nil
}
