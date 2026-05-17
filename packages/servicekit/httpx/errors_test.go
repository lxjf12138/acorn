package httpx

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestHTTPStatusFromError(t *testing.T) {
	tests := []struct {
		code codes.Code
		want int
	}{
		{code: codes.InvalidArgument, want: http.StatusBadRequest},
		{code: codes.PermissionDenied, want: http.StatusForbidden},
		{code: codes.NotFound, want: http.StatusNotFound},
		{code: codes.AlreadyExists, want: http.StatusConflict},
		{code: codes.FailedPrecondition, want: http.StatusConflict},
		{code: codes.ResourceExhausted, want: http.StatusRequestEntityTooLarge},
		{code: codes.Unauthenticated, want: http.StatusUnauthorized},
		{code: codes.Unavailable, want: http.StatusServiceUnavailable},
	}
	for _, tt := range tests {
		if got := HTTPStatusFromError(status.Error(tt.code, "failed")); got != tt.want {
			t.Fatalf("HTTPStatusFromError(%s) = %d, want %d", tt.code, got, tt.want)
		}
	}
	if got := HTTPStatusFromError(errors.New("plain")); got != http.StatusInternalServerError {
		t.Fatalf("HTTPStatusFromError(plain) = %d, want %d", got, http.StatusInternalServerError)
	}
}

func TestErrorEncoder(t *testing.T) {
	recorder := httptest.NewRecorder()
	ErrorEncoder(recorder, httptest.NewRequest(http.MethodGet, "/", nil), status.Error(codes.ResourceExhausted, "too large"))

	if recorder.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusRequestEntityTooLarge)
	}
	var body ErrorResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal body: %v", err)
	}
	if body.Code != codes.ResourceExhausted.String() || body.Message != "too large" {
		t.Fatalf("unexpected body: %+v", body)
	}
}
