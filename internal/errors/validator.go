package errors

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
	"unicode"
)

// ValidationErrDetail represents a single validation error
type ValidationErrDetail struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

// ValidationErr represents a collection of validation errors
type ValidationErr struct {
	Errors []ValidationErrDetail `json:"errors"`
}

// Error implements error interface
func (ve *ValidationErr) Error() string {
	messages := make([]string, len(ve.Errors))
	for i, err := range ve.Errors {
		messages[i] = fmt.Sprintf("%s: %s", err.Field, err.Message)
	}
	return strings.Join(messages, "; ")
}

// NewValidationError creates a new validation error
func NewValidationError(field, message string) *ValidationErr {
	return &ValidationErr{
		Errors: []ValidationErrDetail{
			{Field: field, Message: message},
		},
	}
}

// AddValidationError adds a validation error to the collection
func AddValidationError(field, message string) *ValidationErr {
	return &ValidationErr{
		Errors: []ValidationErrDetail{
			{Field: field, Message: message},
		},
	}
}

// ToAppError converts validation errors to AppError
func (ve *ValidationErr) ToAppError() *AppError {
	details := make([]string, len(ve.Errors))
	for i, err := range ve.Errors {
		details[i] = fmt.Sprintf("%s: %s", err.Field, err.Message)
	}

	return New(ErrCodeValidation, "Validation failed").
		WithDetails(strings.Join(details, "; "))
}

// Validate performs common validations

// ValidateEmail validates email format
func ValidateEmail(email string) error {
	if email == "" {
		return errors.New("email is required")
	}

	if !strings.Contains(email, "@") {
		return errors.New("invalid email format")
	}

	parts := strings.Split(email, "@")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return errors.New("invalid email format")
	}

	return nil
}

// ValidatePassword validates password strength
func ValidatePassword(password string) error {
	if password == "" {
		return errors.New("password is required")
	}

	if len(password) < 8 {
		return errors.New("password must be at least 8 characters")
	}

	var (
		hasUpper   bool
		hasLower   bool
		hasNumber  bool
		hasSpecial bool
	)

	for _, char := range password {
		switch {
		case unicode.IsUpper(char):
			hasUpper = true
		case unicode.IsLower(char):
			hasLower = true
		case unicode.IsNumber(char):
			hasNumber = true
		case unicode.IsPunct(char) || unicode.IsSymbol(char):
			hasSpecial = true
		}
	}

	if !hasUpper || !hasLower {
		return errors.New("password must contain both uppercase and lowercase letters")
	}

	if !hasNumber {
		return errors.New("password must contain at least one number")
	}

	if !hasSpecial {
		return errors.New("password must contain at least one special character")
	}

	return nil
}

// ValidateUsername validates username format
func ValidateUsername(username string) error {
	if username == "" {
		return errors.New("username is required")
	}

	if len(username) < 3 {
		return errors.New("username must be at least 3 characters")
	}

	if len(username) > 50 {
		return errors.New("username must not exceed 50 characters")
	}

	// Check for valid characters
	for _, char := range username {
		if !unicode.IsLetter(char) && !unicode.IsNumber(char) && char != '_' {
			return errors.New("username can only contain letters, numbers, and underscores")
		}
	}

	return nil
}

// ValidateRequired validates that a field is not empty
func ValidateRequired(field, value string) error {
	if strings.TrimSpace(value) == "" {
		return fmt.Errorf("%s is required", field)
	}
	return nil
}

// ValidateStringLength validates string length
func ValidateStringLength(field, value string, min, max int) error {
	length := len(value)
	if length < min {
		return fmt.Errorf("%s must be at least %d characters", field, min)
	}
	if max > 0 && length > max {
		return fmt.Errorf("%s must not exceed %d characters", field, max)
	}
	return nil
}

// ValidateContentType validates content type
func ValidateContentType(contentType string) error {
	validTypes := map[string]bool{
		"text":   true,
		"image":  true,
		"mixed":  true,
	}

	if !validTypes[contentType] {
		return fmt.Errorf("invalid content type: %s", contentType)
	}

	return nil
}

// ValidateStruct validates a struct's fields
func ValidateStruct(v interface{}) *ValidationErr {
	val := reflect.ValueOf(v)
	if val.Kind() == reflect.Ptr {
		val = val.Elem()
	}

	if val.Kind() != reflect.Struct {
		return nil
	}

	var validationErrors []ValidationErrDetail
	typ := val.Type()

	for i := 0; i < val.NumField(); i++ {
		field := typ.Field(i)
		fieldValue := val.Field(i)

		// Skip unexported fields
		if !field.IsExported() {
			continue
		}

		tag := field.Tag.Get("validate")
		if tag == "" {
			continue
		}

		// Parse validation tags
		tags := strings.Split(tag, ",")
		for _, t := range tags {
			t = strings.TrimSpace(t)
			if strings.HasPrefix(t, "required") {
				if isEmpty(fieldValue) {
					validationErrors = append(validationErrors, ValidationErrDetail{
						Field:   field.Name,
						Message: "is required",
					})
				}
			}
			if strings.HasPrefix(t, "min=") {
				minStr := strings.TrimPrefix(t, "min=")
				var min int
				fmt.Sscanf(minStr, "%d", &min)
				if fieldValue.Len() < min {
					validationErrors = append(validationErrors, ValidationErrDetail{
						Field:   field.Name,
						Message: fmt.Sprintf("must be at least %d characters", min),
					})
				}
			}
			if strings.HasPrefix(t, "max=") {
				maxStr := strings.TrimPrefix(t, "max=")
				var max int
				fmt.Sscanf(maxStr, "%d", &max)
				if fieldValue.Len() > max {
					validationErrors = append(validationErrors, ValidationErrDetail{
						Field:   field.Name,
						Message: fmt.Sprintf("must not exceed %d characters", max),
					})
				}
			}
		}
	}

	if len(validationErrors) > 0 {
		return &ValidationErr{Errors: validationErrors}
	}

	return nil
}

// isEmpty checks if a value is empty
func isEmpty(v reflect.Value) bool {
	switch v.Kind() {
	case reflect.String:
		return strings.TrimSpace(v.String()) == ""
	case reflect.Slice, reflect.Array, reflect.Map:
		return v.Len() == 0
	case reflect.Ptr, reflect.Interface:
		return v.IsNil()
	}
	return false
}
