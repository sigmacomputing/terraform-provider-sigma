package client

import (
	"context"
	"fmt"
	"time"
)

// Polling policy for Eventually. Sigma's list endpoints are eventually
// consistent, so a row written by a POST may not appear in the immediately
// following GET. These bounds give the read-back a few seconds to converge.
const (
	eventualMaxAttempts = 6
	eventualBackoffMin  = 200 * time.Millisecond
	eventualBackoffMax  = 2 * time.Second
)

// Eventually polls fn until it reports done=true, returns a non-nil error, or
// the attempt budget is exhausted, sleeping with capped exponential backoff
// between tries (honoring ctx cancellation).
//
// Resources that confirm a create by re-listing — e.g. the grant resources,
// whose POST returns no id, so the new grantId must be discovered from the
// grant list — use this to tolerate eventual-consistency lag instead of
// erroring on the first miss.
func Eventually(ctx context.Context, fn func() (done bool, err error)) error {
	return eventually(ctx, eventualMaxAttempts, eventualBackoffMin, eventualBackoffMax, fn)
}

// eventually is the parameterized core, kept unexported so tests can drive it
// with tiny backoffs.
func eventually(ctx context.Context, maxAttempts int, backoffMin, backoffMax time.Duration, fn func() (bool, error)) error {
	backoff := backoffMin
	for attempt := 1; ; attempt++ {
		done, err := fn()
		if err != nil {
			return err
		}
		if done {
			return nil
		}
		if attempt >= maxAttempts {
			return fmt.Errorf("sigma: condition not satisfied after %d attempts (eventual-consistency timeout)", maxAttempts)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(backoff):
		}
		if backoff *= 2; backoff > backoffMax {
			backoff = backoffMax
		}
	}
}
