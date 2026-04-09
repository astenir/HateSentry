package models

import (
	"time"

	"gorm.io/gorm"
)

// DetectionFeedback represents user feedback on detection results
type DetectionFeedback struct {
	ID              uint           `gorm:"primarykey" json:"id"`
	CreatedAt        time.Time      `json:"created_at"`
	UpdatedAt        time.Time      `json:"updated_at"`
	DeletedAt        gorm.DeletedAt `gorm:"index" json:"-"`
	RequestID       string         `gorm:"index;size:64" json:"request_id"`
	UserID          uint           `gorm:"index" json:"user_id"`
	User            User           `gorm:"foreignKey:UserID" json:"user,omitempty"`
	
	// Feedback data
	IsCorrect       bool           `gorm:"not null" json:"is_correct"`
	ActualLabel     string         `gorm:"size:50" json:"actual_label"` // identity_hate, individual_hate, non_hate
	Confidence      float64        `json:"confidence"` // User's confidence rating (1-10)
	Comments        string         `gorm:"type:text" json:"comments"`
	
	// Correction data
	CorrectCategories string         `gorm:"type:text" json:"correct_categories"` // JSON array
	CorrectExplanation string        `gorm:"type:text" json:"correct_explanation"`
	
	// Feedback status
	Status          string         `gorm:"size:20;default:'pending'" json:"status"` // pending, reviewed, incorporated
	ReviewedBy      uint           `json:"reviewed_by,omitempty"`
	ReviewedAt      *time.Time     `json:"reviewed_at,omitempty"`
}

// FeedbackAggregation represents aggregated feedback statistics
type FeedbackAggregation struct {
	ID                  uint           `gorm:"primarykey" json:"id"`
	CreatedAt           time.Time      `json:"created_at"`
	UpdatedAt           time.Time      `json:"updated_at"`
	
	// Aggregation data
	TimePeriod          string         `gorm:"size:20;index" json:"time_period"` // daily, weekly, monthly
	StartDate          time.Time      `json:"start_date"`
	EndDate            time.Time      `json:"end_date"`
	
	// Statistics
	TotalFeedback       int            `json:"total_feedback"`
	PositiveFeedback   int            `json:"positive_feedback"` // User agrees with detection
	NegativeFeedback   int            `json:"negative_feedback"` // User disagrees
	AccuracyRate       float64        `json:"accuracy_rate"`
	
	// Breakdown by category
	CategoryAccuracy   string         `gorm:"type:text" json:"category_accuracy"` // JSON
	
	// Common false positives/negatives
	CommonFalsePositives string         `gorm:"type:text" json:"common_false_positives"` // JSON
	CommonFalseNegatives string         `gorm:"type:text" json:"common_false_negatives"` // JSON
}

// ModelVersion represents different versions of the detection model
type ModelVersion struct {
	ID              uint           `gorm:"primarykey" json:"id"`
	CreatedAt        time.Time      `json:"created_at"`
	UpdatedAt        time.Time      `json:"updated_at"`
	
	Version         string         `gorm:"uniqueIndex;size:50" json:"version"`
	Model           string         `gorm:"size:50" json:"model"` // openai, ollama
	Parameters      string         `gorm:"type:text" json:"parameters"` // JSON config
	
	// Performance metrics
	Accuracy        float64        `json:"accuracy"`
	Precision       float64        `json:"precision"`
	Recall          float64        `json:"recall"`
	F1Score        float64        `json:"f1_score"`
	
	// Feedback based metrics
	UserAccuracy    float64        `json:"user_accuracy"`
	FeedbackCount  int            `json:"feedback_count"`
	
	// Deployment status
	Status          string         `gorm:"size:20;default:'active'" json:"status"` // active, deprecated
	DeployedAt      *time.Time     `json:"deployed_at,omitempty"`
	RetiredAt       *time.Time     `json:"retired_at,omitempty"`
}
