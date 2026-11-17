package client

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/hashicorp/go-retryablehttp"
)

// errOffOrigin marks a request refused because it left the API origin; retrying can't help.
var errOffOrigin = errors.New("sigma: off-origin request")

// maxRetryAfter is the longest Retry-After we'll wait out by retrying.
const maxRetryAfter = time.Minute

// retryAfter parses Retry-After (seconds or HTTP-date).
func retryAfter(resp *http.Response) (time.Duration, bool) {
	ra := resp.Header.Get("Retry-After")
	if ra == "" {
		return 0, false
	}
	if secs, err := strconv.Atoi(ra); err == nil {
		return time.Duration(secs) * time.Second, true
	}
	if t, err := http.ParseTime(ra); err == nil {
		return time.Until(t), true
	}
	return 0, false
}

// retryPolicy retries 429 and 5xx with exponential backoff.
// Honors the Retry-After header when present (seconds or HTTP-date).
func retryPolicy(ctx context.Context, resp *http.Response, err error) (bool, error) {
	if ctx.Err() != nil {
		return false, ctx.Err()
	}
	if errors.Is(err, errOffOrigin) {
		return false, err
	}
	// Network error: defer to default heuristic.
	if err != nil {
		return retryablehttp.DefaultRetryPolicy(ctx, resp, err)
	}
	if resp == nil {
		return false, nil
	}
	// A long Retry-After (e.g. 3600s) won't clear within our retry budget; fail fast.
	if resp.StatusCode == http.StatusTooManyRequests {
		if wait, ok := retryAfter(resp); ok && wait > maxRetryAfter {
			return false, nil
		}
	}
	switch resp.StatusCode {
	case http.StatusTooManyRequests,
		http.StatusBadGateway,
		http.StatusServiceUnavailable:
		return true, nil
	case http.StatusGatewayTimeout:
		// The request may have completed upstream; don't repeat a create.
		return !isPost(resp), nil
	case http.StatusInternalServerError:
		// 500s in Sigma are usually transient, but a POST may have been applied.
		return !isPost(resp), nil
	}
	return false, nil
}

func isPost(resp *http.Response) bool {
	return resp.Request != nil && resp.Request.Method == http.MethodPost
}

// retryBackoff respects Retry-After when the server sends it; otherwise jittered exponential.
func retryBackoff(min, max time.Duration, attempt int, resp *http.Response) time.Duration {
	if resp != nil {
		if ra := resp.Header.Get("Retry-After"); ra != "" {
			if secs, err := strconv.Atoi(ra); err == nil {
				wait := time.Duration(secs) * time.Second
				if wait > max {
					return max
				}
				if wait < min {
					return min
				}
				return wait
			}
			if t, err := http.ParseTime(ra); err == nil {
				wait := time.Until(t)
				if wait > max {
					return max
				}
				if wait < min {
					return min
				}
				return wait
			}
		}
	}
	return retryablehttp.DefaultBackoff(min, max, attempt, resp)
}

// applyRetryPolicy wires the provider's retry, backoff and error handling onto rc.
func applyRetryPolicy(rc *retryablehttp.Client) {
	rc.CheckRetry = retryPolicy
	rc.Backoff = retryBackoff
	rc.ErrorHandler = surfaceLastResponse
}

// surfaceLastResponse returns the final response once retries run out, so Do can
// parse Sigma's error body instead of reporting a bare "giving up after N attempts".
func surfaceLastResponse(resp *http.Response, err error, _ int) (*http.Response, error) {
	if resp != nil && err == nil {
		return resp, nil
	}
	if resp != nil {
		_ = resp.Body.Close()
	}
	return nil, err
}
