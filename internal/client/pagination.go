package client

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
)

// Page is the v2.1 cursor-paginated response envelope.
//
//	{"entries":[...], "nextPage":"opaque-or-null", "total":N}
type Page[T any] struct {
	Entries  []T    `json:"entries"`
	NextPage string `json:"nextPage,omitempty"`
	Total    int    `json:"total,omitempty"`
}

// MaxPageSize is the largest page Sigma's list endpoints accept (some cap at 500).
const MaxPageSize = 1000

// ListAllPaged drains a v2.1 cursor-paginated endpoint into a single slice.
// `path` is the API path (e.g. "/v2.1/members"); query params other than
// `page` may be passed via `extra`. Pages are MaxPageSize unless `extra` sets
// `limit`.
//
// Sigma returns `nextPage` as an opaque, ALREADY URL-ENCODED token. We must
// not re-encode it (doing so produces `%257B...` instead of `%7B...` and the
// server rejects the request). We append it verbatim while still encoding the
// caller-supplied `extra` params normally.
func ListAllPaged[T any](ctx context.Context, c *Client, path string, extra url.Values) ([]T, error) {
	var out []T
	err := ForEachPaged(ctx, c, path, extra, func(e T) (bool, error) {
		out = append(out, e)
		return false, nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// ListAllArray drains a legacy v2 endpoint that returns a bare JSON array.
func ListAllArray[T any](ctx context.Context, c *Client, path string) ([]T, error) {
	var out []T
	if err := c.Do(ctx, "GET", path, nil, &out); err != nil {
		return nil, fmt.Errorf("sigma: listing %s: %w", path, err)
	}
	return out, nil
}

// ForEachPaged iterates a v2.1 cursor-paginated endpoint, calling `fn` on each
// entry. Return `(stop=true, nil)` from `fn` to short-circuit (e.g. lookup-by-name
// data sources). Any error from `fn` is propagated. Use this instead of
// ListAllPaged when the caller can stop early — Sigma's `/v2/connections/paths`
// endpoint ignores the `connectionId` filter, so a full drain is hundreds of
// requests; early termination is critical for tolerable plan times.
func ForEachPaged[T any](ctx context.Context, c *Client, path string, extra url.Values, fn func(T) (bool, error)) error {
	q := url.Values{}
	for k, v := range extra {
		q[k] = v
	}
	if q.Get("limit") == "" {
		q.Set("limit", strconv.Itoa(MaxPageSize))
	}
	base := path + "?" + q.Encode()
	cursor := ""
	for {
		full := base
		if cursor != "" {
			full += "&page=" + cursor
		}
		var page Page[T]
		if err := c.Do(ctx, "GET", full, nil, &page); err != nil {
			return fmt.Errorf("sigma: paging %s: %w", path, err)
		}
		for _, e := range page.Entries {
			stop, err := fn(e)
			if err != nil {
				return err
			}
			if stop {
				return nil
			}
		}
		if page.NextPage == "" {
			return nil
		}
		cursor = page.NextPage
	}
}

// PageToken is the pageToken/nextPageToken envelope used by a few v2 endpoints.
type PageToken[T any] struct {
	Entries       []T    `json:"entries"`
	NextPageToken string `json:"nextPageToken,omitempty"`
}

// PageSize returns query params requesting n entries per page, for the
// pageToken helpers. Not every pageToken endpoint accepts pageSize (jwkSets
// keys reject unknown params), so callers opt in.
func PageSize(n int) url.Values {
	return url.Values{"pageSize": {strconv.Itoa(n)}}
}

// ListAllPageToken drains an endpoint paged with pageToken/nextPageToken.
// `extra` (may be nil) is sent on every page, e.g. PageSize(MaxPageSize).
func ListAllPageToken[T any](ctx context.Context, c *Client, path string, extra url.Values) ([]T, error) {
	var out []T
	err := ForEachPageToken(ctx, c, path, extra, func(e T) (bool, error) {
		out = append(out, e)
		return false, nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// ForEachPageToken is ForEachPaged for pageToken/nextPageToken endpoints.
func ForEachPageToken[T any](ctx context.Context, c *Client, path string, extra url.Values, fn func(T) (bool, error)) error {
	token := ""
	for {
		q := url.Values{}
		for k, v := range extra {
			q[k] = v
		}
		if token != "" {
			q.Set("pageToken", token)
		}
		full := path
		if len(q) > 0 {
			full += "?" + q.Encode()
		}
		var page PageToken[T]
		if err := c.Do(ctx, "GET", full, nil, &page); err != nil {
			return fmt.Errorf("sigma: paging %s: %w", path, err)
		}
		for _, e := range page.Entries {
			stop, err := fn(e)
			if err != nil {
				return err
			}
			if stop {
				return nil
			}
		}
		if page.NextPageToken == "" {
			return nil
		}
		token = page.NextPageToken
	}
}
