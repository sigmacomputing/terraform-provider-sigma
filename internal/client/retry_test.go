package client

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestRetryPolicy_Retries429And5xx(t *testing.T) {
	cases := []struct {
		status int
		want   bool
	}{
		{http.StatusTooManyRequests, true},
		{http.StatusBadGateway, true},
		{http.StatusServiceUnavailable, true},
		{http.StatusGatewayTimeout, true},
		{http.StatusInternalServerError, true},
		{http.StatusOK, false},
		{http.StatusBadRequest, false},
		{http.StatusNotFound, false},
	}
	for _, tc := range []struct {
		status int
		want   bool
	}{
		{http.StatusTooManyRequests, true},
		{http.StatusServiceUnavailable, true},
		{http.StatusInternalServerError, false},
		{http.StatusGatewayTimeout, false},
	} {
		resp := &http.Response{StatusCode: tc.status, Request: &http.Request{Method: http.MethodPost}}
		if got, _ := retryPolicy(context.Background(), resp, nil); got != tc.want {
			t.Errorf("POST status=%d: retry=%v, want %v", tc.status, got, tc.want)
		}
	}
	for _, tc := range cases {
		resp := &http.Response{StatusCode: tc.status}
		got, _ := retryPolicy(context.Background(), resp, nil)
		if got != tc.want {
			t.Errorf("status=%d: retry=%v, want %v", tc.status, got, tc.want)
		}
	}
}

func TestRetryBackoff_HonorsRetryAfterSeconds(t *testing.T) {
	resp := &http.Response{
		StatusCode: http.StatusTooManyRequests,
		Header:     http.Header{"Retry-After": []string{"7"}},
	}
	got := retryBackoff(1*time.Second, 30*time.Second, 1, resp)
	want := 7 * time.Second
	if got != want {
		t.Errorf("Retry-After=7s: got %v, want %v", got, want)
	}
}

func TestRetryBackoff_ClampsRetryAfterToMax(t *testing.T) {
	resp := &http.Response{
		StatusCode: http.StatusTooManyRequests,
		Header:     http.Header{"Retry-After": []string{"3600"}},
	}
	got := retryBackoff(1*time.Second, 30*time.Second, 1, resp)
	if got != 30*time.Second {
		t.Errorf("Retry-After=1h with max=30s: got %v, want 30s (clamped)", got)
	}
}

// TestDo_SurfacesErrorBodyAfterRetries: a persistent 503 must come back as the
// API's error message, not "giving up after N attempts".
func TestDo_SurfacesErrorBodyAfterRetries(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Retry-After", "0")
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte(`{"requestId":"r1","message":"SMTP Error: Invalid host or port","code":"service_error"}`))
	}))
	defer srv.Close()

	c := newTestClientNoAuth(srv.URL)
	c.http.RetryMax = 1
	applyRetryPolicy(c.http)
	err := c.Do(context.Background(), "PATCH", "/x", map[string]string{"a": "b"}, nil)
	var apiErr *APIError
	if !errors.As(err, &apiErr) || apiErr.Status != http.StatusServiceUnavailable || !strings.Contains(apiErr.Message, "Invalid host") {
		t.Fatalf("want 503 APIError with message, got %v", err)
	}
}

// TestDo_FailsFastOnLongRetryAfter: a 429 asking for a long wait is not retried
// and the wait is shown to the user.
func TestDo_FailsFastOnLongRetryAfter(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls++
		w.Header().Set("Retry-After", "3600")
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer srv.Close()

	c := newTestClientNoAuth(srv.URL)
	c.http.RetryMax = 4
	applyRetryPolicy(c.http)
	err := c.Do(context.Background(), "GET", "/x", nil, nil)
	var apiErr *APIError
	if !errors.As(err, &apiErr) || apiErr.Status != http.StatusTooManyRequests || !strings.Contains(apiErr.Message, "3600") {
		t.Fatalf("want 429 APIError mentioning 3600, got %v", err)
	}
	if calls != 1 {
		t.Errorf("expected 1 call (no retry), got %d", calls)
	}
}
