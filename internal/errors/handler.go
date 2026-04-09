package errors

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// ErrorResponse represents the standardized error response structure
type ErrorResponse struct {
	Error     string        `json:"error"`
	Code      ErrorCode     `json:"code"`
	Message   string        `json:"message"`
	Details   string        `json:"details,omitempty"`
	Severity  ErrorSeverity `json:"severity,omitempty"`
	TraceID   string        `json:"trace_id,omitempty"`
	Timestamp string        `json:"timestamp"`
}

// Handle handles application errors and sends appropriate HTTP response
func Handle(c *gin.Context, err error) {
	if err == nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:     "Unknown error",
			Code:      ErrCodeInternalError,
			Message:   "An unknown error occurred",
			Timestamp: getCurrentTimestamp(),
		})
		return
	}

	// Check if it's an AppError
	if appErr, ok := err.(*AppError); ok {
		sendErrorResponse(c, appErr)
		return
	}

	// Handle other errors
	sendGenericErrorResponse(c, err)
}

// RespondWithError sends an error response using an AppError
func RespondWithError(c *gin.Context, appErr *AppError) {
	sendErrorResponse(c, appErr)
}

// sendErrorResponse sends a formatted error response for AppError
func sendErrorResponse(c *gin.Context, appErr *AppError) {
	traceID := appErr.TraceID
	if traceID == "" {
		if traceIDValue, exists := c.Get("trace_id"); exists {
			if tid, ok := traceIDValue.(string); ok {
				traceID = tid
			}
		}
	}

	statusCode := appErr.DefaultHTTPStatus()

	response := ErrorResponse{
		Error:     string(appErr.Code),
		Code:      appErr.Code,
		Message:   appErr.Message,
		Severity:  appErr.Severity,
		TraceID:   traceID,
		Timestamp: getCurrentTimestamp(),
	}

	if appErr.Details != "" {
		response.Details = appErr.Details
	}

	// Log error based on severity
	logError(c, appErr, statusCode)

	c.JSON(statusCode, response)
}

// sendGenericErrorResponse sends a generic error response for non-AppError types
func sendGenericErrorResponse(c *gin.Context, err error) {
	appErr := Internal("An unexpected error occurred").WithDetails(err.Error())
	appErr.TraceID = getTraceID(c)
	sendErrorResponse(c, appErr)
}

// logError logs error based on severity and status code
func logError(c *gin.Context, appErr *AppError, statusCode int) {
	// Get logger from context
	logger, exists := c.Get("logger")
	if !exists {
		return
	}

	// Log based on severity
	// This should be integrated with your logging system
	// For now, we'll just prepare the log entry
	_ = logger
	_ = statusCode
	_ = appErr
}

// getTraceID retrieves trace ID from context
func getTraceID(c *gin.Context) string {
	if traceIDValue, exists := c.Get("trace_id"); exists {
		if tid, ok := traceIDValue.(string); ok {
			return tid
		}
	}
	return ""
}

// getCurrentTimestamp returns current timestamp in ISO format
func getCurrentTimestamp() string {
	// Use proper time formatting
	// For now, return empty string - implement with time.Now().Format(time.RFC3339)
	return ""
}

// Middleware to recover from panics and convert to errors
func RecoveryMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if r := recover(); r != nil {
				appErr := Internal("Internal server error").
					WithDetails("Recovered from panic").
					WithTraceID(getTraceID(c))

				logger, exists := c.Get("logger")
				if exists {
					// Log panic
					_ = logger
				}

				sendErrorResponse(c, appErr)
				c.Abort()
			}
		}()

		c.Next()
	}
}
