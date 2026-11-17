package client

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
)

// Sentinel errors for use with errors.Is. Resource Read implementations should
// prefer `errors.Is(err, client.ErrNotFound)` over the IsNotFound helper for
// idiomatic Go.
var (
	ErrNotFound = errors.New("sigma: not found")
	ErrConflict = errors.New("sigma: conflict")
)

// APIError is Sigma's standard error envelope: {message, code, requestId}.
// Returned by Do() for any non-2xx response. Surface RequestID in user-facing
// errors so support escalations can be traced. Implements errors.Is so that
// `errors.Is(err, ErrNotFound)` matches a 404 and `errors.Is(err, ErrConflict)`
// matches a 409.
type APIError struct {
	Status     int    `json:"-"`
	Code       string `json:"code"`
	Message    string `json:"message"`
	RequestID  string `json:"requestId"`
	RawBody    string `json:"-"`
	HTTPMethod string `json:"-"`
	URL        string `json:"-"`
}

// Is enables errors.Is(err, client.ErrNotFound|ErrConflict).
func (e *APIError) Is(target error) bool {
	switch target {
	case ErrNotFound:
		return e.Status == http.StatusNotFound
	case ErrConflict:
		return e.Status == http.StatusConflict
	}
	return false
}

func (e *APIError) Error() string {
	switch {
	case e.Code != "" && e.RequestID != "":
		return fmt.Sprintf("sigma: %s %s -> %d %s (code=%s, requestId=%s)", e.HTTPMethod, e.URL, e.Status, e.Message, e.Code, e.RequestID)
	case e.RequestID != "":
		return fmt.Sprintf("sigma: %s %s -> %d %s (requestId=%s)", e.HTTPMethod, e.URL, e.Status, e.Message, e.RequestID)
	case e.Message != "":
		return fmt.Sprintf("sigma: %s %s -> %d %s", e.HTTPMethod, e.URL, e.Status, e.Message)
	default:
		return fmt.Sprintf("sigma: %s %s -> %d (no body)", e.HTTPMethod, e.URL, e.Status)
	}
}

// IsNotFound is a convenience wrapper around errors.Is(err, ErrNotFound).
// Equivalent to checking for HTTP 404 from the Sigma API.
func IsNotFound(err error) bool { return errors.Is(err, ErrNotFound) }

// IsConflict is a convenience wrapper around errors.Is(err, ErrConflict).
// Equivalent to checking for HTTP 409.
func IsConflict(err error) bool { return errors.Is(err, ErrConflict) }

// IsArchived reports whether err is Sigma's "inode_archived" error, returned
// when reading or mutating a workspace/folder/data model that has been archived
// (Sigma's soft-delete is archival, not hard deletion). For drift purposes,
// treat an archived inode like ErrNotFound: the resource is effectively gone and
// should be removed from state so Terraform proposes recreation.
//
// Sigma is inconsistent about how it signals archival: GET /v2/workspaces/{id}
// returns 409 with code "inode_archived", whereas GET /v2/files/{id} returns 200
// with "isArchived": true in the body. This helper covers the error case; callers
// reading /v2/files must additionally inspect the IsArchived response field.
func IsArchived(err error) bool {
	var apiErr *APIError
	if errors.As(err, &apiErr) {
		return apiErr.Code == "inode_archived"
	}
	return false
}

func parseError(resp *http.Response) error {
	body, _ := io.ReadAll(resp.Body)
	apiErr := &APIError{
		Status:     resp.StatusCode,
		HTTPMethod: resp.Request.Method,
		URL:        resp.Request.URL.String(),
		RawBody:    string(body),
	}
	if len(body) > 0 {
		_ = json.Unmarshal(body, apiErr)
	}
	if apiErr.Message == "" {
		apiErr.Message = http.StatusText(resp.StatusCode)
	}
	if resp.StatusCode == http.StatusTooManyRequests {
		if ra := resp.Header.Get("Retry-After"); ra != "" {
			apiErr.Message += " (Sigma asks to retry after " + ra + "s)"
		}
	}
	return apiErr
}
