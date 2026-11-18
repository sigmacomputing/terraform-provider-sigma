package client

import (
	"time"

	"github.com/hashicorp/go-retryablehttp"
)

// newPlainRetryableClient returns a retryablehttp client without any custom
// transport — used by unit tests that don't go through OAuth.
func newPlainRetryableClient() *retryablehttp.Client {
	rc := retryablehttp.NewClient()
	rc.RetryMax = 0
	rc.RetryWaitMin = 1 * time.Millisecond
	rc.RetryWaitMax = 1 * time.Millisecond
	rc.Logger = nil
	return rc
}
