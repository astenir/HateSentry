package webhooks

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	apperrors "hatesentry/internal/errors"
	"hatesentry/internal/models"
)

const (
	secretPrefix = "whsec_"
	secretBytes  = 32

	headerEvent     = "X-HateSentry-Event"
	headerDelivery  = "X-HateSentry-Delivery"
	headerTimestamp = "X-HateSentry-Timestamp"
	headerSignature = "X-HateSentry-Signature"
)

// Dispatcher delivers signed webhook callbacks for final moderation decisions.
type Dispatcher interface {
	DispatchFinalDecision(ctx context.Context, client models.ClientApplication, payload FinalDecisionPayload) error
}

// FinalDecisionPayload is the stable JSON body sent to external clients.
type FinalDecisionPayload struct {
	Event         string    `json:"event"`
	RequestID     string    `json:"request_id"`
	ClientID      uint      `json:"client_id"`
	ExternalID    string    `json:"external_id,omitempty"`
	ActorID       string    `json:"actor_id,omitempty"`
	Source        string    `json:"source"`
	Decision      string    `json:"decision"`
	ReviewStatus  string    `json:"review_status,omitempty"`
	RiskScore     float64   `json:"risk_score"`
	Labels        []string  `json:"labels"`
	Reason        string    `json:"reason"`
	PolicyVersion string    `json:"policy_version"`
	CreatedAt     time.Time `json:"created_at"`
}

// HTTPDispatcher sends callbacks over HTTP.
type HTTPDispatcher struct {
	client *http.Client
	now    func() time.Time
}

// NewHTTPDispatcher creates an HTTP webhook dispatcher with a short timeout.
func NewHTTPDispatcher() *HTTPDispatcher {
	return &HTTPDispatcher{
		client: &http.Client{
			Timeout:       5 * time.Second,
			CheckRedirect: noRedirect,
			Transport:     safeTransport(),
		},
		now: time.Now,
	}
}

// NewHTTPDispatcherWithClient creates a dispatcher using a caller-provided HTTP client.
func NewHTTPDispatcherWithClient(client *http.Client) *HTTPDispatcher {
	if client == nil {
		client = &http.Client{
			Timeout:       5 * time.Second,
			CheckRedirect: noRedirect,
			Transport:     safeTransport(),
		}
	}

	return &HTTPDispatcher{
		client: client,
		now:    time.Now,
	}
}

// GenerateSecret creates a per-client webhook signing secret.
func GenerateSecret() (string, error) {
	secret := make([]byte, secretBytes)
	if _, err := rand.Read(secret); err != nil {
		return "", apperrors.Internal("failed to generate webhook secret").WithDetails(err.Error())
	}

	return secretPrefix + base64.RawURLEncoding.EncodeToString(secret), nil
}

// Sign returns the HMAC-SHA256 signature for a webhook body and Unix timestamp.
func Sign(secret string, timestamp int64, body []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(strconv.FormatInt(timestamp, 10)))
	mac.Write([]byte("."))
	mac.Write(body)
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}

// ValidateURL validates a webhook callback URL before storing or dispatching it.
func ValidateURL(rawURL string) error {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return nil
	}

	parsed, err := url.ParseRequestURI(rawURL)
	if err != nil || parsed.Host == "" {
		return apperrors.ValidationError("webhook_url must be a valid absolute URL")
	}
	if parsed.Scheme != "https" {
		return apperrors.ValidationError("webhook_url must use https")
	}

	host := parsed.Hostname()
	if isLocalHostname(host) {
		return apperrors.ValidationError("webhook_url must not target localhost")
	}
	if ip, err := netip.ParseAddr(host); err == nil && isBlockedIP(ip) {
		return apperrors.ValidationError("webhook_url must not target private or local addresses")
	}

	return nil
}

// DispatchFinalDecision posts a signed final decision callback.
func (d *HTTPDispatcher) DispatchFinalDecision(
	ctx context.Context,
	client models.ClientApplication,
	payload FinalDecisionPayload,
) error {
	if d == nil {
		return apperrors.ConfigurationError("webhook dispatcher is not configured")
	}
	if strings.TrimSpace(client.WebhookURL) == "" {
		return nil
	}
	if strings.TrimSpace(client.WebhookSecret) == "" {
		return apperrors.ConfigurationError("webhook secret is not configured")
	}
	if err := ValidateURL(client.WebhookURL); err != nil {
		return err
	}
	if payload.Event == "" {
		payload.Event = "moderation.final_decision"
	}
	if payload.ClientID == 0 {
		payload.ClientID = client.ID
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal webhook payload: %w", err)
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodPost, client.WebhookURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create webhook request: %w", err)
	}

	now := d.now
	if now == nil {
		now = time.Now
	}
	timestamp := now().UTC().Unix()
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set(headerEvent, payload.Event)
	request.Header.Set(headerDelivery, uuid.New().String())
	request.Header.Set(headerTimestamp, strconv.FormatInt(timestamp, 10))
	request.Header.Set(headerSignature, Sign(client.WebhookSecret, timestamp, body))

	httpClient := d.client
	if httpClient == nil {
		httpClient = &http.Client{
			Timeout:       5 * time.Second,
			CheckRedirect: noRedirect,
			Transport:     safeTransport(),
		}
	}

	response, err := httpClient.Do(request)
	if err != nil {
		return fmt.Errorf("send webhook request: %w", err)
	}
	defer response.Body.Close()

	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("webhook returned status %d", response.StatusCode)
	}

	return nil
}

func noRedirect(req *http.Request, via []*http.Request) error {
	return http.ErrUseLastResponse
}

func safeTransport() http.RoundTripper {
	return &http.Transport{
		DialContext: safeDialContext,
	}
}

func safeDialContext(ctx context.Context, network string, address string) (net.Conn, error) {
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return nil, err
	}

	ips, err := net.DefaultResolver.LookupIPAddr(ctx, host)
	if err != nil {
		return nil, fmt.Errorf("resolve webhook host: %w", err)
	}

	for _, ip := range ips {
		addr, ok := netip.AddrFromSlice(ip.IP)
		if !ok || isBlockedIP(addr) {
			continue
		}

		dialer := &net.Dialer{Timeout: 5 * time.Second}
		return dialer.DialContext(ctx, network, net.JoinHostPort(addr.String(), port))
	}

	return nil, apperrors.ValidationError("webhook host resolves to private or local addresses")
}

func isLocalHostname(host string) bool {
	host = strings.TrimSuffix(strings.ToLower(host), ".")
	return host == "localhost" || strings.HasSuffix(host, ".localhost")
}

func isBlockedIP(ip netip.Addr) bool {
	if ip.Is4In6() {
		ip = netip.AddrFrom4(ip.As4())
	}

	return ip.IsLoopback() ||
		ip.IsPrivate() ||
		ip.IsLinkLocalUnicast() ||
		ip.IsLinkLocalMulticast() ||
		ip.IsMulticast() ||
		ip.IsUnspecified() ||
		isCloudMetadataIP(ip)
}

func isCloudMetadataIP(ip netip.Addr) bool {
	return ip == netip.MustParseAddr("169.254.169.254") ||
		ip == netip.MustParseAddr("fd00:ec2::254")
}
