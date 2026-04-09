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
	ContentType string        `gorm:"size:20" json:"content_type"` // text, image, mixed
	Processed   bool           `gorm:"default:false" json:"processed"`
	Status      string         `gorm:"size:20;default:'pending'" json:"status"` // pending, processing, completed, failed
}

// DetectionResult represents a detection result
type DetectionResult struct {
	ID               uint           `gorm:"primarykey" json:"id"`
	CreatedAt        time.Time      `json:"created_at"`
	UpdatedAt        time.Time      `json:"updated_at"`
	DeletedAt        gorm.DeletedAt `gorm:"index" json:"-"`
	RequestID        string         `gorm:"index;size:64" json:"request_id"`
	IsHateSpeech     bool           `gorm:"not null" json:"is_hate_speech"`
	Confidence       float64        `gorm:"not null" json:"confidence"`
	Categories       string         `gorm:"type:text" json:"categories"` // JSON array
	Explanation      string         `gorm:"type:text" json:"explanation"`
	Model            string         `gorm:"size:50" json:"model"`
	ProcessingTime   int64          `gorm:"default:0" json:"processing_time"` // milliseconds
	PromptUsed       string         `gorm:"type:text" json:"prompt_used,omitempty"`
	RawResponse      string         `gorm:"type:longtext" json:"raw_response,omitempty"`
}

// DetectionStats represents detection statistics
type DetectionStats struct {
	ID          uint           `gorm:"primarykey" json:"id"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	DeletedAt   gorm.DeletedAt `gorm:"index" json:"-"`
	UserID      uint           `gorm:"index" json:"user_id"`
	Date        time.Time      `gorm:"index" json:"date"`
	TotalReq    int            `gorm:"default:0" json:"total_requests"`
	HateSpeech  int            `gorm:"default:0" json:"hate_speech_count"`
	Benign      int            `gorm:"default:0" json:"benign_count"`
	AvgConfidence float64     `gorm:"default:0" json:"avg_confidence"`
	AvgTime     int64          `gorm:"default:0" json:"avg_processing_time"`
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
