package client

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestParseError_StandardEnvelope(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"message":"member not found","code":"RESOURCE_NOT_FOUND","requestId":"req-abc-123"}`))
	}))
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/v2/members/missing")
	if err != nil {
		t.Fatal(err)
	}
	got := parseError(resp)

	apiErr := &APIError{}
	if !errors.As(got, &apiErr) {
		t.Fatalf("expected *APIError, got %T", got)
	}
	if apiErr.Status != http.StatusNotFound {
		t.Errorf("Status = %d, want 404", apiErr.Status)
	}
	if apiErr.Code != "RESOURCE_NOT_FOUND" {
		t.Errorf("Code = %q, want RESOURCE_NOT_FOUND", apiErr.Code)
	}
	if apiErr.RequestID != "req-abc-123" {
		t.Errorf("RequestID = %q, want req-abc-123", apiErr.RequestID)
	}
	// IsNotFound and IsConflict helpers.
	if !IsNotFound(apiErr) {
		t.Error("IsNotFound should return true on 404")
	}
	if IsConflict(apiErr) {
		t.Error("IsConflict should return false on 404")
	}
	// Idiomatic errors.Is form must work too.
	if !errors.Is(apiErr, ErrNotFound) {
		t.Error("errors.Is(err, ErrNotFound) should be true on 404")
	}
	if errors.Is(apiErr, ErrConflict) {
		t.Error("errors.Is(err, ErrConflict) should be false on 404")
	}
	// Error string includes requestId.
	if !strings.Contains(apiErr.Error(), "req-abc-123") {
		t.Errorf("Error() = %q, must include requestId", apiErr.Error())
	}
}

func TestParseError_EmptyBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	resp, err := http.Get(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	got := parseError(resp)
	apiErr := &APIError{}
	if !errors.As(got, &apiErr) {
		t.Fatalf("expected *APIError, got %T", got)
	}
	// With no JSON body, falls back to http.StatusText.
	if apiErr.Message != "Service Unavailable" {
		t.Errorf("Message = %q, want fallback to status text", apiErr.Message)
	}
}

func TestIsNotFound_NilAndOtherErrors(t *testing.T) {
	if IsNotFound(nil) {
		t.Error("IsNotFound(nil) should be false")
	}
	if IsNotFound(errors.New("some random error")) {
		t.Error("IsNotFound on non-APIError should be false")
	}
}
