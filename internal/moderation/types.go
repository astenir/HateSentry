package moderation

// Decision is the business action returned to integrating applications.
type Decision string

const (
	DecisionAllow  Decision = "allow"
	DecisionReview Decision = "review"
	DecisionBlock  Decision = "block"
)

// ReviewStatus is the operator workflow state for a review case.
type ReviewStatus string

const (
	ReviewStatusPending  ReviewStatus = "pending"
	ReviewStatusApproved ReviewStatus = "approved"
	ReviewStatusRejected ReviewStatus = "rejected"
	ReviewStatusMistake  ReviewStatus = "mistake"
)

// WebhookDeliveryStatus is the persisted status of a final-decision callback.
type WebhookDeliveryStatus string

const (
	WebhookDeliverySucceeded WebhookDeliveryStatus = "succeeded"
	WebhookDeliveryFailed    WebhookDeliveryStatus = "failed"
	WebhookDeliveryRetrying  WebhookDeliveryStatus = "retrying"
)

var supportedLabels = map[string]bool{
	"hate":            true,
	"harassment":      true,
	"identity_attack": true,
	"threat":          true,
	"sexual":          true,
	"violence":        true,
	"spam":            true,
	"self_harm":       true,
	"illegal":         true,
	"safe":            true,
}

// IsSupportedLabel reports whether label is in the first-version moderation vocabulary.
func IsSupportedLabel(label string) bool {
	return supportedLabels[label]
}

// ProviderSuggestion is the normalized risk signal parsed from model output.
type ProviderSuggestion struct {
	RiskScore float64
	Labels    []string
	Reason    string
	RawOutput string
}

// ProviderInfo records the AI backend used to produce a suggestion.
type ProviderInfo struct {
	Provider string
	Model    string
}

// DecisionResult is the service-owned policy decision derived from a provider suggestion.
type DecisionResult struct {
	Decision      Decision
	RiskScore     float64
	Labels        []string
	Reason        string
	PolicyVersion string
}
