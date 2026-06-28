package errors

import (
	"net/http"
	"testing"
)

func TestExternalServiceErrorHandlesNilCause(t *testing.T) {
	err := ExternalServiceError(nil, "no response from provider")
	if err == nil {
		t.Fatal("ExternalServiceError() = nil, want AppError")
	}
	if err.Code != ErrCodeServiceUnavailable {
		t.Fatalf("Code = %q, want %q", err.Code, ErrCodeServiceUnavailable)
	}
	if err.HTTPStatus != http.StatusBadGateway {
		t.Fatalf("HTTPStatus = %d, want %d", err.HTTPStatus, http.StatusBadGateway)
	}
	if err.Cause != nil {
		t.Fatalf("Cause = %v, want nil", err.Cause)
	}
}
