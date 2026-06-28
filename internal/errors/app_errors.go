package errors

import (
	"fmt"
	"net/http"
)

// ErrorCode represents the error code type
type ErrorCode string

const (
	// General errors (1000-1999)
	ErrCodeInternalError ErrorCode = "INTERNAL_ERROR"
	ErrCodeBadRequest    ErrorCode = "BAD_REQUEST"
	ErrCodeUnauthorized  ErrorCode = "UNAUTHORIZED"
	ErrCodeForbidden     ErrorCode = "FORBIDDEN"
	ErrCodeNotFound      ErrorCode = "NOT_FOUND"
	ErrCodeConflict      ErrorCode = "CONFLICT"
	ErrCodeValidation    ErrorCode = "VALIDATION_ERROR"
	ErrCodeRateLimit     ErrorCode = "RATE_LIMIT_EXCEEDED"

	// Authentication errors (2000-2999)
	ErrCodeInvalidToken    ErrorCode = "INVALID_TOKEN"
	ErrCodeExpiredToken    ErrorCode = "EXPIRED_TOKEN"
	ErrCodeInvalidCreds    ErrorCode = "INVALID_CREDENTIALS"
	ErrCodeAccountLocked   ErrorCode = "ACCOUNT_LOCKED"
	ErrCodeAccountInactive ErrorCode = "ACCOUNT_INACTIVE"

	// Database errors (3000-3999)
	ErrCodeDatabaseError   ErrorCode = "DATABASE_ERROR"
	ErrCodeRecordNotFound  ErrorCode = "RECORD_NOT_FOUND"
	ErrCodeDuplicateRecord ErrorCode = "DUPLICATE_RECORD"
	ErrCodeConnectionError ErrorCode = "DATABASE_CONNECTION_ERROR"

	// Detection errors (4000-4999)
	ErrCodeDetectionFailed  ErrorCode = "DETECTION_FAILED"
	ErrCodeContentRequired  ErrorCode = "CONTENT_REQUIRED"
	ErrCodeImageRequired    ErrorCode = "IMAGE_REQUIRED"
	ErrCodeInvalidContent   ErrorCode = "INVALID_CONTENT"
	ErrCodeAIProviderError  ErrorCode = "AI_PROVIDER_ERROR"
	ErrCodeModelUnavailable ErrorCode = "MODEL_UNAVAILABLE"

	// Cache errors (5000-5999)
	ErrCodeCacheError       ErrorCode = "CACHE_ERROR"
	ErrCodeCacheMiss        ErrorCode = "CACHE_MISS"
	ErrCodeRedisUnavailable ErrorCode = "REDIS_UNAVAILABLE"

	// Queue errors (6000-6999)
	ErrCodeQueueError    ErrorCode = "QUEUE_ERROR"
	ErrCodeQueueFull     ErrorCode = "QUEUE_FULL"
	ErrCodePublishFailed ErrorCode = "PUBLISH_FAILED"

	// External service errors (7000-7999)
	ErrCodeExternalService    ErrorCode = "EXTERNAL_SERVICE_ERROR"
	ErrCodeTimeout            ErrorCode = "TIMEOUT"
	ErrCodeServiceUnavailable ErrorCode = "SERVICE_UNAVAILABLE"
)

// ErrorSeverity represents the error severity level
type ErrorSeverity string

const (
	SeverityLow      ErrorSeverity = "low"
	SeverityMedium   ErrorSeverity = "medium"
	SeverityHigh     ErrorSeverity = "high"
	SeverityCritical ErrorSeverity = "critical"
)

// AppError represents the application error structure
type AppError struct {
	Code       ErrorCode     `json:"code"`
	Message    string        `json:"message"`
	Details    string        `json:"details,omitempty"`
	HTTPStatus int           `json:"-"`
	Severity   ErrorSeverity `json:"severity"`
	TraceID    string        `json:"trace_id,omitempty"`
	Cause      error         `json:"-"`
}

// Error implements the error interface
func (e *AppError) Error() string {
	if e.Details != "" {
		return fmt.Sprintf("[%s] %s: %s", e.Code, e.Message, e.Details)
	}
	return fmt.Sprintf("[%s] %s", e.Code, e.Message)
}

// Unwrap returns the underlying error
func (e *AppError) Unwrap() error {
	return e.Cause
}

// New creates a new application error
func New(code ErrorCode, message string) *AppError {
	return &AppError{
		Code:     code,
		Message:  message,
		Severity: getSeverity(code),
	}
}

// Wrap wraps an existing error with additional context
func Wrap(err error, code ErrorCode, message string) *AppError {
	if err == nil {
		return nil
	}

	// If it's already an AppError, just add context
	if appErr, ok := err.(*AppError); ok {
		return &AppError{
			Code:       code,
			Message:    message,
			HTTPStatus: appErr.HTTPStatus,
			Severity:   getSeverity(code),
			Cause:      appErr,
		}
	}

	return &AppError{
		Code:     code,
		Message:  message,
		Details:  err.Error(),
		Severity: getSeverity(code),
		Cause:    err,
	}
}

// WithHTTPStatus sets the HTTP status code
func (e *AppError) WithHTTPStatus(status int) *AppError {
	e.HTTPStatus = status
	return e
}

// WithDetails adds details to the error
func (e *AppError) WithDetails(details string) *AppError {
	e.Details = details
	return e
}

// WithTraceID adds a trace ID to the error
func (e *AppError) WithTraceID(traceID string) *AppError {
	e.TraceID = traceID
	return e
}

// getSeverity returns the severity level based on error code
func getSeverity(code ErrorCode) ErrorSeverity {
	switch code {
	case ErrCodeInternalError, ErrCodeDatabaseError, ErrCodeAIProviderError, ErrCodeQueueError:
		return SeverityCritical
	case ErrCodeUnauthorized, ErrCodeForbidden, ErrCodeAccountLocked:
		return SeverityHigh
	case ErrCodeBadRequest, ErrCodeValidation, ErrCodeDetectionFailed:
		return SeverityMedium
	default:
		return SeverityLow
	}
}

// HTTPStatus returns the default HTTP status code for an error code
func (e *AppError) DefaultHTTPStatus() int {
	if e.HTTPStatus != 0 {
		return e.HTTPStatus
	}

	switch e.Code {
	case ErrCodeBadRequest, ErrCodeValidation, ErrCodeContentRequired, ErrCodeImageRequired, ErrCodeInvalidContent:
		return http.StatusBadRequest
	case ErrCodeUnauthorized, ErrCodeInvalidToken, ErrCodeExpiredToken, ErrCodeInvalidCreds:
		return http.StatusUnauthorized
	case ErrCodeForbidden, ErrCodeAccountLocked, ErrCodeAccountInactive, ErrCodeRateLimit:
		return http.StatusForbidden
	case ErrCodeNotFound, ErrCodeRecordNotFound:
		return http.StatusNotFound
	case ErrCodeConflict, ErrCodeDuplicateRecord:
		return http.StatusConflict
	case ErrCodeInternalError, ErrCodeDatabaseError, ErrCodeAIProviderError, ErrCodeCacheError, ErrCodeQueueError, ErrCodeServiceUnavailable:
		return http.StatusInternalServerError
	case ErrCodeModelUnavailable, ErrCodeRedisUnavailable:
		return http.StatusServiceUnavailable
	default:
		return http.StatusInternalServerError
	}
}

// Common error constructors

// Internal creates a generic internal server error
func Internal(message string) *AppError {
	return New(ErrCodeInternalError, message).WithHTTPStatus(http.StatusInternalServerError)
}

// BadRequest creates a bad request error
func BadRequest(message string) *AppError {
	return New(ErrCodeBadRequest, message).WithHTTPStatus(http.StatusBadRequest)
}

// Unauthorized creates an unauthorized error
func Unauthorized(message string) *AppError {
	return New(ErrCodeUnauthorized, message).WithHTTPStatus(http.StatusUnauthorized)
}

// Forbidden creates a forbidden error
func Forbidden(message string) *AppError {
	return New(ErrCodeForbidden, message).WithHTTPStatus(http.StatusForbidden)
}

// NotFound creates a not found error
func NotFound(message string) *AppError {
	return New(ErrCodeNotFound, message).WithHTTPStatus(http.StatusNotFound)
}

// Conflict creates a conflict error
func Conflict(message string) *AppError {
	return New(ErrCodeConflict, message).WithHTTPStatus(http.StatusConflict)
}

// ValidationError creates a validation error
func ValidationError(message string) *AppError {
	return New(ErrCodeValidation, message).WithHTTPStatus(http.StatusBadRequest)
}

// DatabaseError creates a database error
func DatabaseError(err error, message string) *AppError {
	return Wrap(err, ErrCodeDatabaseError, message).WithHTTPStatus(http.StatusInternalServerError)
}

// RecordNotFound creates a record not found error
func RecordNotFound(message string) *AppError {
	return New(ErrCodeRecordNotFound, message).WithHTTPStatus(http.StatusNotFound)
}

// DuplicateRecord creates a duplicate record error
func DuplicateRecord(message string) *AppError {
	return New(ErrCodeDuplicateRecord, message).WithHTTPStatus(http.StatusConflict)
}

// DetectionFailed creates a detection failed error
func DetectionFailed(err error, message string) *AppError {
	return Wrap(err, ErrCodeDetectionFailed, message).WithHTTPStatus(http.StatusInternalServerError)
}

// AIProviderError creates an AI provider error
func AIProviderError(err error, message string) *AppError {
	return Wrap(err, ErrCodeAIProviderError, message).WithHTTPStatus(http.StatusInternalServerError)
}

// RateLimitExceeded creates a rate limit error
func RateLimitExceeded(message string) *AppError {
	return New(ErrCodeRateLimit, message).WithHTTPStatus(http.StatusTooManyRequests)
}

// CacheError creates a cache error
func CacheError(err error, message string) *AppError {
	return Wrap(err, ErrCodeCacheError, message)
}

// QueueError creates a queue error
func QueueError(err error, message string) *AppError {
	return Wrap(err, ErrCodeQueueError, message).WithHTTPStatus(http.StatusInternalServerError)
}

// ExternalServiceError creates an external service error
func ExternalServiceError(err error, message string) *AppError {
	if err == nil {
		return New(ErrCodeServiceUnavailable, message).WithHTTPStatus(http.StatusBadGateway)
	}

	return Wrap(err, ErrCodeServiceUnavailable, message).WithHTTPStatus(http.StatusBadGateway)
}

// Timeout creates a timeout error
func Timeout(message string) *AppError {
	return New(ErrCodeTimeout, message).WithHTTPStatus(http.StatusGatewayTimeout)
}

// InvalidCredentials creates an invalid credentials error
func InvalidCredentials(message string) *AppError {
	return New(ErrCodeInvalidCreds, message).WithHTTPStatus(http.StatusUnauthorized)
}

// InvalidToken creates an invalid token error
func InvalidToken(message string) *AppError {
	return New(ErrCodeInvalidToken, message).WithHTTPStatus(http.StatusUnauthorized)
}

// ExpiredToken creates an expired token error
func ExpiredToken(message string) *AppError {
	return New(ErrCodeExpiredToken, message).WithHTTPStatus(http.StatusUnauthorized)
}

// ConfigurationError creates a configuration error
func ConfigurationError(message string) *AppError {
	return New(ErrCodeInternalError, message).WithHTTPStatus(http.StatusInternalServerError)
}

// NotImplemented creates a not implemented error
func NotImplemented(message string) *AppError {
	return New(ErrCodeInternalError, message).WithHTTPStatus(http.StatusNotImplemented)
}
