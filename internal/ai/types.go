package ai

import "time"

// DetectionRequest represents a detection request
type DetectionRequest struct {
	RequestID   string `json:"request_id"`
	Content     string `json:"content"`
	ImageURL    string `json:"image_url,omitempty"`
	ContentType string `json:"content_type"` // text, image, mixed
}

// DetectionResponse represents a detection response
type DetectionResponse struct {
	RequestID      string        `json:"request_id"`
	IsHateSpeech  bool          `json:"is_hate_speech"`
	Confidence     float64       `json:"confidence"`
	Categories     []string      `json:"categories"`
	Explanation    string        `json:"explanation"`
	Model          string        `json:"model"`
	ProcessingTime time.Duration `json:"processing_time"`
	PromptUsed     string        `json:"prompt_used,omitempty"`
	RawResponse    string        `json:"raw_response,omitempty"`
}

// StreamDetectionEvent represents a streaming detection event
type StreamDetectionEvent struct {
	Type     string      `json:"type"` // start, progress, result, error
	Data     interface{} `json:"data"`
	Progress float64     `json:"progress,omitempty"`
}

// HateSpeechCategory represents hate speech categories
type HateSpeechCategory struct {
	Name        string  `json:"name"`
	Confidence  float64 `json:"confidence"`
	Description string  `json:"description,omitempty"`
}

// ChainOfThoughtPrompt represents chain of thought prompt structure
type ChainOfThoughtPrompt struct {
	InitialInstruction string   `json:"initial_instruction"`
	ReasoningSteps    []string `json:"reasoning_steps"`
	FinalTask         string   `json:"final_task"`
}
