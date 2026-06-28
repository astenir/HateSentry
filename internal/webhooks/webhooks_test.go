package webhooks

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"hatesentry/internal/models"
)

func TestGenerateSecretAndSign(t *testing.T) {
	secret, err := GenerateSecret()
	if err != nil {
		t.Fatalf("GenerateSecret() error = %v", err)
	}
	if !strings.HasPrefix(secret, secretPrefix) {
		t.Fatalf("secret = %q, want prefix %q", secret, secretPrefix)
	}

	signature := Sign("whsec_test", 1782633600, []byte(`{"request_id":"req_1"}`))
	if !strings.HasPrefix(signature, "sha256=") {
		t.Fatalf("signature = %q, want sha256 prefix", signature)
	}
	if signature != Sign("whsec_test", 1782633600, []byte(`{"request_id":"req_1"}`)) {
		t.Fatal("Sign() should be deterministic for the same input")
	}
}

func TestValidateURL(t *testing.T) {
	tests := []struct {
		name    string
		rawURL  string
		wantErr string
	}{
		{
			name:   "public https",
			rawURL: "https://moderation.example/webhook",
		},
		{
			name:    "plain http",
			rawURL:  "http://moderation.example/webhook",
			wantErr: "webhook_url must use https",
		},
		{
			name:    "localhost",
			rawURL:  "https://localhost/webhook",
			wantErr: "webhook_url must not target localhost",
		},
		{
			name:    "loopback",
			rawURL:  "https://127.0.0.1/webhook",
			wantErr: "webhook_url must not target private or local addresses",
		},
		{
			name:    "metadata service",
			rawURL:  "https://169.254.169.254/latest/meta-data",
			wantErr: "webhook_url must not target private or local addresses",
		},
		{
			name:    "rfc1918",
			rawURL:  "https://192.168.1.1/webhook",
			wantErr: "webhook_url must not target private or local addresses",
		},
		{
			name:    "ipv6 loopback",
			rawURL:  "https://[::1]/webhook",
			wantErr: "webhook_url must not target private or local addresses",
		},
		{
			name:    "ipv6 private",
			rawURL:  "https://[fd00::1]/webhook",
			wantErr: "webhook_url must not target private or local addresses",
		},
		{
			name:    "multicast",
			rawURL:  "https://224.0.0.1/webhook",
			wantErr: "webhook_url must not target private or local addresses",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateURL(tt.rawURL)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("ValidateURL() error = %v", err)
				}
				return
			}
			if err == nil {
				t.Fatal("ValidateURL() error = nil, want error")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("ValidateURL() error = %q, want %q", err.Error(), tt.wantErr)
			}
		})
	}
}

func TestHTTPDispatcherDispatchFinalDecision(t *testing.T) {
	var received FinalDecisionPayload
	transport := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.Method != http.MethodPost {
			t.Fatalf("method = %s, want POST", r.Method)
		}
		if r.URL.String() != "https://example.com/moderation/webhook" {
			t.Fatalf("url = %s, want webhook URL", r.URL.String())
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Fatalf("Content-Type = %q, want application/json", r.Header.Get("Content-Type"))
		}
		if r.Header.Get(headerEvent) != "moderation.final_decision" {
			t.Fatalf("%s = %q", headerEvent, r.Header.Get(headerEvent))
		}
		if r.Header.Get(headerDelivery) != "delivery-123" {
			t.Fatalf("%s = %q, want delivery-123", headerDelivery, r.Header.Get(headerDelivery))
		}
		if r.Header.Get(headerTimestamp) != "1782633600" {
			t.Fatalf("%s = %q, want 1782633600", headerTimestamp, r.Header.Get(headerTimestamp))
		}

		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read body: %v", err)
		}
		var payload FinalDecisionPayload
		if err := json.Unmarshal(body, &payload); err != nil {
			t.Fatalf("decode payload: %v", err)
		}
		wantSignature := Sign("whsec_test", 1782633600, body)
		if r.Header.Get(headerSignature) != wantSignature {
			t.Fatalf("%s = %q, want %q", headerSignature, r.Header.Get(headerSignature), wantSignature)
		}

		received = payload
		return &http.Response{
			StatusCode: http.StatusNoContent,
			Body:       io.NopCloser(strings.NewReader("")),
			Header:     make(http.Header),
		}, nil
	})

	dispatcher := &HTTPDispatcher{
		client: &http.Client{Transport: transport},
		now: func() time.Time {
			return time.Unix(1782633600, 0)
		},
	}

	err := dispatcher.DispatchFinalDecision(context.Background(), models.ClientApplication{
		ID:            11,
		WebhookURL:    "https://example.com/moderation/webhook",
		WebhookSecret: "whsec_test",
	}, FinalDecisionPayload{
		DeliveryID:    "delivery-123",
		Event:         "moderation.final_decision",
		RequestID:     "request-123",
		ClientID:      11,
		ExternalID:    "comment_123",
		Decision:      "block",
		RiskScore:     0.8,
		Labels:        []string{"hate"},
		Reason:        "Policy threshold exceeded.",
		PolicyVersion: "default-v1",
		CreatedAt:     time.Date(2026, 6, 28, 12, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("DispatchFinalDecision() error = %v", err)
	}

	payload := received
	if payload.RequestID != "request-123" {
		t.Fatalf("RequestID = %q, want request-123", payload.RequestID)
	}
	if payload.Decision != "block" {
		t.Fatalf("Decision = %q, want block", payload.Decision)
	}
}

func TestHTTPDispatcherReturnsErrorForNonSuccessStatus(t *testing.T) {
	dispatcher := NewHTTPDispatcherWithClient(&http.Client{
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusInternalServerError,
				Body:       io.NopCloser(strings.NewReader("")),
				Header:     make(http.Header),
			}, nil
		}),
	})
	err := dispatcher.DispatchFinalDecision(context.Background(), models.ClientApplication{
		ID:            11,
		WebhookURL:    "https://example.com/moderation/webhook",
		WebhookSecret: "whsec_test",
	}, FinalDecisionPayload{
		RequestID: "request-123",
		Decision:  "block",
	})
	if err == nil {
		t.Fatal("DispatchFinalDecision() error = nil, want error")
	}
	if !strings.Contains(err.Error(), "status 500") {
		t.Fatalf("DispatchFinalDecision() error = %q, want status 500", err.Error())
	}
	var deliveryErr *DeliveryError
	if !errors.As(err, &deliveryErr) {
		t.Fatalf("DispatchFinalDecision() error type = %T, want DeliveryError", err)
	}
	if deliveryErr.StatusCode != http.StatusInternalServerError {
		t.Fatalf("DeliveryError status = %d, want 500", deliveryErr.StatusCode)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}
