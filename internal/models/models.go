package models

import (
	"time"

	"gorm.io/gorm"
)

// User represents a user in the system
type User struct {
	ID        uint           `gorm:"primarykey" json:"id"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
	Username  string         `gorm:"uniqueIndex;not null;size:50" json:"username"`
	Email     string         `gorm:"uniqueIndex;not null;size:100" json:"email"`
	Password  string         `gorm:"not null" json:"-"`
	Role      string         `gorm:"not null;default:'user';size:20" json:"role"` // admin, user
	Status    string         `gorm:"not null;default:'active';size:20" json:"status"`
	APIKey    string         `gorm:"uniqueIndex;size:64" json:"api_key"`
}

// ClientApplication represents an external application that can call moderation APIs.
type ClientApplication struct {
	ID            uint           `gorm:"primarykey" json:"id"`
	CreatedAt     time.Time      `json:"created_at"`
	UpdatedAt     time.Time      `json:"updated_at"`
	DeletedAt     gorm.DeletedAt `gorm:"index" json:"-"`
	UserID        uint           `gorm:"not null;index" json:"user_id"`
	User          User           `gorm:"foreignKey:UserID" json:"user,omitempty"`
	Name          string         `gorm:"size:100;not null" json:"name"`
	APIKeyHash    string         `gorm:"uniqueIndex;size:64;not null" json:"-"`
	APIKeyPrefix  string         `gorm:"size:20;index" json:"api_key_prefix"`
	Status        string         `gorm:"size:20;not null;index;default:'active'" json:"status"`
	WebhookURL    string         `gorm:"size:500" json:"webhook_url,omitempty"`
	WebhookSecret string         `gorm:"size:80" json:"-"`
	PolicyVersion string         `gorm:"size:50" json:"policy_version,omitempty"`
}

// DetectionRequest represents a detection request
type DetectionRequest struct {
	ID          uint           `gorm:"primarykey" json:"id"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	DeletedAt   gorm.DeletedAt `gorm:"index" json:"-"`
	RequestID   string         `gorm:"uniqueIndex;size:64" json:"request_id"`
	UserID      uint           `gorm:"not null;index" json:"user_id"`
	User        User           `gorm:"foreignKey:UserID" json:"user,omitempty"`
	Content     string         `gorm:"type:text" json:"content"`
	ImageURL    string         `gorm:"size:500" json:"image_url,omitempty"`
	ContentType string         `gorm:"size:20" json:"content_type"` // text, image, mixed
	Processed   bool           `gorm:"default:false" json:"processed"`
	Status      string         `gorm:"size:20;default:'pending'" json:"status"` // pending, processing, completed, failed
}

// DetectionResult represents a detection result
type DetectionResult struct {
	ID             uint           `gorm:"primarykey" json:"id"`
	CreatedAt      time.Time      `json:"created_at"`
	UpdatedAt      time.Time      `json:"updated_at"`
	DeletedAt      gorm.DeletedAt `gorm:"index" json:"-"`
	RequestID      string         `gorm:"index;size:64" json:"request_id"`
	IsHateSpeech   bool           `gorm:"not null" json:"is_hate_speech"`
	Confidence     float64        `gorm:"not null" json:"confidence"`
	Categories     string         `gorm:"type:text" json:"categories"` // JSON array
	Explanation    string         `gorm:"type:text" json:"explanation"`
	Model          string         `gorm:"size:50" json:"model"`
	ProcessingTime int64          `gorm:"default:0" json:"processing_time"` // milliseconds
	PromptUsed     string         `gorm:"type:text" json:"prompt_used,omitempty"`
	RawResponse    string         `gorm:"type:longtext" json:"raw_response,omitempty"`
}

// ModerationRequest stores the original text moderation request for auditability.
type ModerationRequest struct {
	ID             uint              `gorm:"primarykey" json:"id"`
	CreatedAt      time.Time         `json:"created_at"`
	UpdatedAt      time.Time         `json:"updated_at"`
	DeletedAt      gorm.DeletedAt    `gorm:"index" json:"-"`
	RequestID      string            `gorm:"uniqueIndex;size:64;not null" json:"request_id"`
	UserID         uint              `gorm:"not null;index" json:"user_id"`
	User           User              `gorm:"foreignKey:UserID" json:"user,omitempty"`
	ClientID       *uint             `gorm:"index" json:"client_id,omitempty"`
	Client         ClientApplication `gorm:"foreignKey:ClientID" json:"client,omitempty"`
	IdempotencyKey *string           `gorm:"uniqueIndex;size:200" json:"-"`
	Content        string            `gorm:"type:text;not null" json:"content"`
	Source         string            `gorm:"size:50;index" json:"source"`
	ExternalID     string            `gorm:"size:128;index" json:"external_id,omitempty"`
	ActorID        string            `gorm:"size:128;index" json:"actor_id,omitempty"`
	Status         string            `gorm:"size:20;index;default:'completed'" json:"status"`
}

// ModerationResult stores the provider suggestion and service-owned decision.
type ModerationResult struct {
	ID            uint              `gorm:"primarykey" json:"id"`
	CreatedAt     time.Time         `json:"created_at"`
	UpdatedAt     time.Time         `json:"updated_at"`
	DeletedAt     gorm.DeletedAt    `gorm:"index" json:"-"`
	RequestID     string            `gorm:"uniqueIndex;size:64;not null" json:"request_id"`
	UserID        uint              `gorm:"not null;index" json:"user_id"`
	ClientID      *uint             `gorm:"index" json:"client_id,omitempty"`
	Client        ClientApplication `gorm:"foreignKey:ClientID" json:"client,omitempty"`
	Provider      string            `gorm:"size:50" json:"provider"`
	Model         string            `gorm:"size:100" json:"model"`
	RawOutput     string            `gorm:"type:longtext" json:"-"`
	RiskScore     float64           `gorm:"not null" json:"risk_score"`
	Labels        string            `gorm:"type:text" json:"labels"`
	Decision      string            `gorm:"size:20;not null;index" json:"decision"`
	Reason        string            `gorm:"type:text" json:"reason"`
	PolicyVersion string            `gorm:"size:50;not null;index" json:"policy_version"`
}

// ReviewCase tracks human review for moderation results that need operator judgment.
type ReviewCase struct {
	ID            uint           `gorm:"primarykey" json:"id"`
	CreatedAt     time.Time      `json:"created_at"`
	UpdatedAt     time.Time      `json:"updated_at"`
	DeletedAt     gorm.DeletedAt `gorm:"index" json:"-"`
	RequestID     string         `gorm:"uniqueIndex;size:64;not null" json:"request_id"`
	UserID        uint           `gorm:"not null;index" json:"user_id"`
	ClientID      *uint          `gorm:"index" json:"client_id,omitempty"`
	Status        string         `gorm:"size:20;not null;index;default:'pending'" json:"status"`
	ReviewerID    *uint          `gorm:"index" json:"reviewer_id,omitempty"`
	FinalDecision string         `gorm:"size:20;index" json:"final_decision,omitempty"`
	ReviewNotes   string         `gorm:"type:text" json:"review_notes,omitempty"`
	ReviewedAt    *time.Time     `gorm:"index" json:"reviewed_at,omitempty"`
}

// DetectionStats represents detection statistics
type DetectionStats struct {
	ID            uint           `gorm:"primarykey" json:"id"`
	CreatedAt     time.Time      `json:"created_at"`
	UpdatedAt     time.Time      `json:"updated_at"`
	DeletedAt     gorm.DeletedAt `gorm:"index" json:"-"`
	UserID        uint           `gorm:"index" json:"user_id"`
	Date          time.Time      `gorm:"index" json:"date"`
	TotalReq      int            `gorm:"default:0" json:"total_requests"`
	HateSpeech    int            `gorm:"default:0" json:"hate_speech_count"`
	Benign        int            `gorm:"default:0" json:"benign_count"`
	AvgConfidence float64        `gorm:"default:0" json:"avg_confidence"`
	AvgTime       int64          `gorm:"default:0" json:"avg_processing_time"`
}

// AuditLog represents an audit log entry
type AuditLog struct {
	ID        uint           `gorm:"primarykey" json:"id"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
	UserID    uint           `gorm:"index" json:"user_id,omitempty"`
	Action    string         `gorm:"size:100;not null" json:"action"`
	Resource  string         `gorm:"size:100" json:"resource"`
	Details   string         `gorm:"type:text" json:"details"`
	IPAddress string         `gorm:"size:50" json:"ip_address"`
	UserAgent string         `gorm:"size:500" json:"user_agent"`
}
