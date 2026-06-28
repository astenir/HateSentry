package moderation

import (
	"strings"
	"testing"
)

func TestParseProviderSuggestion(t *testing.T) {
	raw := `{"risk_score":0.82,"labels":[" Harassment ","harassment","IDENTITY_ATTACK"],"reason":"Needs operator review."}`

	suggestion, err := ParseProviderSuggestion(raw)
	if err != nil {
		t.Fatalf("ParseProviderSuggestion() error = %v", err)
	}

	if suggestion.RiskScore != 0.82 {
		t.Fatalf("RiskScore = %v, want 0.82", suggestion.RiskScore)
	}
	wantLabels := []string{"harassment", "identity_attack"}
	if !equalStrings(suggestion.Labels, wantLabels) {
		t.Fatalf("Labels = %#v, want %#v", suggestion.Labels, wantLabels)
	}
	if suggestion.Reason != "Needs operator review." {
		t.Fatalf("Reason = %q, want Needs operator review.", suggestion.Reason)
	}
	if suggestion.RawOutput != raw {
		t.Fatal("RawOutput does not preserve original provider output")
	}
}

func TestParseProviderSuggestionRejectsInvalidOutput(t *testing.T) {
	tests := []struct {
		name    string
		raw     string
		wantErr string
	}{
		{
			name:    "empty",
			raw:     "",
			wantErr: "provider output is required",
		},
		{
			name:    "wrapped json",
			raw:     "result: {\"risk_score\":0.1,\"labels\":[\"safe\"],\"reason\":\"ok\"}",
			wantErr: "parse provider output",
		},
		{
			name:    "unknown field",
			raw:     `{"risk_score":0.1,"labels":["safe"],"reason":"ok","decision":"allow"}`,
			wantErr: "unknown field",
		},
		{
			name:    "extra object",
			raw:     `{"risk_score":0.1,"labels":["safe"],"reason":"ok"} {"risk_score":0.2}`,
			wantErr: "single JSON object",
		},
		{
			name:    "missing risk score",
			raw:     `{"labels":["safe"],"reason":"ok"}`,
			wantErr: "risk_score is required",
		},
		{
			name:    "risk below zero",
			raw:     `{"risk_score":-0.01,"labels":["safe"],"reason":"ok"}`,
			wantErr: "risk_score must be between 0 and 1",
		},
		{
			name:    "risk above one",
			raw:     `{"risk_score":1.01,"labels":["safe"],"reason":"ok"}`,
			wantErr: "risk_score must be between 0 and 1",
		},
		{
			name:    "empty labels",
			raw:     `{"risk_score":0.1,"labels":[],"reason":"ok"}`,
			wantErr: "labels must not be empty",
		},
		{
			name:    "unsupported label",
			raw:     `{"risk_score":0.1,"labels":["unknown"],"reason":"ok"}`,
			wantErr: "unsupported label",
		},
		{
			name:    "missing reason",
			raw:     `{"risk_score":0.1,"labels":["safe"]}`,
			wantErr: "reason is required",
		},
		{
			name:    "empty reason",
			raw:     `{"risk_score":0.1,"labels":["safe"],"reason":" "}`,
			wantErr: "reason is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseProviderSuggestion(tt.raw)
			if err == nil {
				t.Fatal("ParseProviderSuggestion() error = nil, want error")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("ParseProviderSuggestion() error = %q, want %q", err.Error(), tt.wantErr)
			}
		})
	}
}

func TestIsSupportedLabel(t *testing.T) {
	if !IsSupportedLabel("identity_attack") {
		t.Fatal("IsSupportedLabel(identity_attack) = false, want true")
	}
	if IsSupportedLabel("unknown") {
		t.Fatal("IsSupportedLabel(unknown) = true, want false")
	}
}

func equalStrings(got []string, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}
