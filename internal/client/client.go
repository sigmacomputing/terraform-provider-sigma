// Package client is the HTTP foundation for the Sigma Terraform provider:
// OAuth2 client-credentials auth, retry-aware transport, error parsing, and
// pagination helpers. Every resource talks to Sigma through this package; if
// you find yourself reaching for net/http directly, stop and add the helper here.
package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/hashicorp/go-retryablehttp"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"golang.org/x/oauth2"
)

// EscapeID URL-path-escapes a state-derived identifier before concatenating it
// into a request path. Sigma's IDs are alphanumeric-with-hyphens in practice
// so this is usually a no-op, but every call site uses it as defense-in-depth
// against any future API that surfaces IDs containing path-special characters.
func EscapeID(s string) string { return url.PathEscape(s) }

// SubsystemHTTP is the tflog subsystem name for wire-level HTTP logs.
const SubsystemHTTP = "sigma_http"

// DefaultBaseURL is the global Sigma API endpoint. Region-specific deploys
// override via the provider's `base_url` attribute or SIGMA_BASE_URL env var.
const DefaultBaseURL = "https://api.sigmacomputing.com"

type Config struct {
	BaseURL      string
	ClientID     string
	ClientSecret string
	UserAgent    string
}

// Client is the shared HTTP client for all Sigma API calls. Safe for concurrent
// use; one *Client per provider instance, shared by every resource.
type Client struct {
	base   string
	http   *retryablehttp.Client
	ua     string
	tokSrc oauth2.TokenSource

	// roster memoizes team member-ID sets for the lifetime of one provider run
	// so sigma_team_membership reads cost one roster list per team rather than
	// one per membership (see roster.go). Guarded by rosterMu.
	rosterMu    sync.Mutex
	rosterCache map[string]map[string]struct{}
	rosterLocks map[string]*sync.Mutex

	// listCache memoizes whole-collection list results per run so that
	// per-item resource Reads (grants, user-attribute assignments, etc.) cost
	// one parent-collection list per parent rather than one per item — the
	// generalization of the roster cache (see listcache.go). Guarded by listMu.
	listMu    sync.Mutex
	listCache map[string]any
	listLocks map[string]*sync.Mutex
}

func New(ctx context.Context, cfg Config) (*Client, error) {
	if cfg.ClientID == "" || cfg.ClientSecret == "" {
		return nil, fmt.Errorf("sigma: client_id and client_secret are required")
	}
	base := strings.TrimRight(cfg.BaseURL, "/")
	if base == "" {
		base = DefaultBaseURL
	}
	baseURL, err := url.Parse(base)
	if err != nil || baseURL.Host == "" {
		return nil, fmt.Errorf("sigma: invalid base_url %q", base)
	}
	// The client secret is posted to base_url, so require TLS outside local testing.
	if baseURL.Scheme != "https" && !(baseURL.Scheme == "http" && isLoopback(baseURL.Hostname())) {
		return nil, fmt.Errorf("sigma: base_url must use https, got %q", base)
	}

	// The token source must outlive Configure()'s short-lived ctx — otherwise
	// every token refresh on a later resource op fails with "context canceled".
	// Per-request ctx still cancels in-flight HTTP work via retryablehttp.
	_ = ctx
	rc := retryablehttp.NewClient()
	// Keep enough idle connections for Terraform's default parallelism (10);
	// http.DefaultTransport keeps 2 per host and re-handshakes the rest.
	pooled := rc.HTTPClient.Transport.(*http.Transport)
	pooled.MaxIdleConnsPerHost = 16
	// Requests carry the bearer token or client secret, so never follow a
	// redirect off the API origin (including an https->http downgrade).
	sameHost := func(req *http.Request, via []*http.Request) error {
		if req.URL.Scheme != baseURL.Scheme || req.URL.Host != baseURL.Host {
			return fmt.Errorf("%w: refusing redirect to %s://%s", errOffOrigin, req.URL.Scheme, req.URL.Host)
		}
		if len(via) >= 10 {
			return fmt.Errorf("sigma: stopped after 10 redirects")
		}
		return nil
	}
	tokCtx := context.WithValue(context.Background(), oauth2.HTTPClient, &http.Client{Transport: pooled, Timeout: 30 * time.Second, CheckRedirect: sameHost})
	tokSrc := newTokenSource(tokCtx, base, cfg.ClientID, cfg.ClientSecret)

	rc.RetryMax = 4
	rc.RetryWaitMin = 1 * time.Second
	rc.RetryWaitMax = maxRetryAfter
	applyRetryPolicy(rc)
	rc.Logger = nil // wire logging goes through tflog from Do()
	rc.HTTPClient.Transport = &authTransport{src: tokSrc, base: pooled, scheme: baseURL.Scheme, host: baseURL.Host}
	rc.HTTPClient.Timeout = 5 * time.Minute
	rc.HTTPClient.CheckRedirect = sameHost

	return &Client{
		base:   base,
		http:   rc,
		ua:     cfg.UserAgent,
		tokSrc: tokSrc,
	}, nil
}

func isLoopback(host string) bool {
	return host == "localhost" || host == "127.0.0.1" || host == "::1"
}

// secretFieldKeys are masked in any structured log field passed to tflog.
// Covers OAuth (Authorization, *_token, client_secret) and warehouse credential
// shapes Sigma accepts inside connection.details (password / privateKey /
// privateKeyPassphrase / serviceAccount).
var secretFieldKeys = []string{
	"Authorization", "client_secret", "clientSecret",
	"access_token", "accessToken",
	"refresh_token", "refreshToken",
	"password", "secret", "api_key", "apiKey",
	"privateKey", "private_key",
	"privateKeyPassphrase", "private_key_passphrase",
	"serviceAccount", "service_account",
}

// secretBodyRegex masks JSON values for sensitive keys in raw body strings.
// It is the fallback for bodies that aren't valid JSON and the pattern given to
// tflog's message masking; maskBody redacts parsed JSON by key instead.
// Also includes embedUrl (deprecated `secure` embeds carry signed tokens in the URL;
// `public` embed URLs are shareable but still not appropriate for trace logs).
// The value pattern skips escaped quotes so `\"` inside a value doesn't end it early.
var secretBodyRegex = regexp.MustCompile(`"(client_secret|clientSecret|access_token|accessToken|refresh_token|refreshToken|password|secret|api_key|apiKey|privateKey|private_key|privateKeyPassphrase|private_key_passphrase|serviceAccount|service_account|embedUrl|embed_url|token|bearerToken|secretAccessKey|sessionToken)"\s*:\s*"(?:[^"\\]|\\.)*"`)

// uaValueBodyRegex masks Sigma user-attribute values, which arrive on the wire as
// typed envelopes `{"type":"<t>","val":"<v>"}`. The key is `value` on assignment
// endpoints and `defaultValue` (camelCase) / `default_value` on the UA itself.
// The schema marks UA values Sensitive (so they're redacted in plan/state), but
// the matching HTTP body logging needs its own pattern since `Sensitive` doesn't
// extend to log lines.
var uaValueBodyRegex = regexp.MustCompile(`"(value|defaultValue|default_value)"\s*:\s*\{"type":"[^"]+","val":"[^"]*"\}`)

// secretKeys are JSON keys whose whole value (string, object or array) is
// redacted, compared lowercased with underscores removed.
var secretKeys = map[string]bool{
	"authorization": true, "clientsecret": true, "accesstoken": true, "refreshtoken": true,
	"password": true, "secret": true, "apikey": true, "privatekey": true,
	"privatekeypassphrase": true, "serviceaccount": true, "embedurl": true,
	"token": true, "bearertoken": true, "secretaccesskey": true, "sessiontoken": true,
	"usercredential": true,
}

// uaValueKeys hold user-attribute values when they're a {"type","val"} envelope.
var uaValueKeys = map[string]bool{"value": true, "defaultvalue": true}

const redacted = "<<REDACTED>>"

// maskBody redacts secrets in a request/response body before it's logged.
// Used at the log boundary because tflog's masking helpers operate on log
// message text or structured field keys — they do NOT mask substrings inside a
// field VALUE.
func maskBody(b []byte) string {
	if len(b) == 0 {
		return ""
	}
	var v any
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.UseNumber()
	if err := dec.Decode(&v); err != nil {
		return maskBodyRegex(string(b))
	}
	var out bytes.Buffer
	enc := json.NewEncoder(&out)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(maskValue(v)); err != nil {
		return maskBodyRegex(string(b))
	}
	return strings.TrimSuffix(out.String(), "\n")
}

func maskValue(v any) any {
	switch t := v.(type) {
	case map[string]any:
		for k, val := range t {
			nk := strings.ToLower(strings.ReplaceAll(k, "_", ""))
			switch {
			case secretKeys[nk]:
				t[k] = redacted
			case uaValueKeys[nk] && isUAEnvelope(val):
				t[k] = redacted
			case nk == "value" && isConnectorParam(t):
				t[k] = redacted
			default:
				t[k] = maskValue(val)
			}
		}
		return t
	case []any:
		for i := range t {
			t[i] = maskValue(t[i])
		}
		return t
	case string:
		// JSON nested in a string (e.g. connection details_json) is masked too.
		if s := strings.TrimSpace(t); strings.HasPrefix(s, "{") || strings.HasPrefix(s, "[") {
			if masked := maskBody([]byte(s)); masked != s {
				return masked
			}
		}
		return t
	}
	return v
}

// isConnectorParam matches an api-connector header/query param ({key, mode, value}).
func isConnectorParam(m map[string]any) bool {
	_, hasKey := m["key"]
	_, hasMode := m["mode"]
	return hasKey && hasMode
}

func isUAEnvelope(v any) bool {
	m, ok := v.(map[string]any)
	if !ok {
		return false
	}
	_, hasType := m["type"]
	_, hasVal := m["val"]
	return hasType && hasVal
}

func maskBodyRegex(s string) string {
	redact := func(m string) string {
		idx := strings.Index(m, ":")
		if idx < 0 {
			return m
		}
		return m[:idx+1] + `"` + redacted + `"`
	}
	s = secretBodyRegex.ReplaceAllStringFunc(s, redact)
	return uaValueBodyRegex.ReplaceAllStringFunc(s, redact)
}

// Do executes an API call. `body`, if non-nil, is JSON-encoded into the request
// body. `out`, if non-nil, is JSON-decoded from the response body. Returns
// *APIError on any non-2xx response.
//
// Path may be either a full URL or a path starting with "/" (relative to BaseURL).
func (c *Client) Do(ctx context.Context, method, path string, body, out any) error {
	// Mask common secret fields and body patterns for the duration of this call.
	ctx = tflog.MaskFieldValuesWithFieldKeys(ctx, secretFieldKeys...)
	ctx = tflog.MaskMessageRegexes(ctx, secretBodyRegex, uaValueBodyRegex)
	ctx = tflog.SubsystemMaskFieldValuesWithFieldKeys(ctx, SubsystemHTTP, secretFieldKeys...)
	ctx = tflog.SubsystemMaskMessageRegexes(ctx, SubsystemHTTP, secretBodyRegex, uaValueBodyRegex)

	url := path
	if strings.HasPrefix(path, "/") {
		url = c.base + path
	}

	var bodyReader io.Reader
	var bodyBytes []byte
	if body != nil {
		var err error
		bodyBytes, err = json.Marshal(body)
		if err != nil {
			return fmt.Errorf("sigma: marshalling %s body: %w", method, err)
		}
		bodyReader = bytes.NewReader(bodyBytes)
	}

	req, err := retryablehttp.NewRequestWithContext(ctx, method, url, bodyReader)
	if err != nil {
		return fmt.Errorf("sigma: building %s %s: %w", method, url, err)
	}
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.ua != "" {
		req.Header.Set("User-Agent", c.ua)
	}

	tflog.Debug(ctx, "sigma api request", map[string]any{
		"method": method,
		"url":    url,
	})
	// tflog field-value masking and message-regex masking do NOT inspect inside
	// a string field's content. Pre-mask the body bytes before passing them in,
	// otherwise JSON like {"privateKey":"...PEM..."} leaks at TRACE.
	if traceEnabled() {
		tflog.SubsystemTrace(ctx, SubsystemHTTP, "request body", map[string]any{
			"body": maskBody(bodyBytes),
		})
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("sigma: %s %s: %w", method, url, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode >= 400 {
		return parseError(resp)
	}

	tflog.Debug(ctx, "sigma api response", map[string]any{
		"method": method,
		"url":    url,
		"status": resp.StatusCode,
	})

	if out == nil || resp.StatusCode == http.StatusNoContent {
		_, _ = io.Copy(io.Discard, resp.Body)
		return nil
	}
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("sigma: reading %s %s response: %w", method, url, err)
	}
	if traceEnabled() {
		tflog.SubsystemTrace(ctx, SubsystemHTTP, "response body", map[string]any{
			"body": maskBody(respBody),
		})
	}
	if len(respBody) == 0 {
		return nil
	}
	if err := json.Unmarshal(respBody, out); err != nil {
		return fmt.Errorf("sigma: decoding %s %s response: %w", method, url, err)
	}
	return nil
}

// BaseURL returns the configured base URL.
func (c *Client) BaseURL() string { return c.base }

// traceEnabled reports whether provider logs are at TRACE, so the body masking
// (two regexes over every body) only runs when the bodies are logged.
var traceEnabled = sync.OnceValue(func() bool {
	for _, env := range []string{"TF_LOG_PROVIDER_SIGMA", "TF_LOG_PROVIDER", "TF_LOG"} {
		if lvl := strings.ToUpper(os.Getenv(env)); lvl != "" {
			return lvl == "TRACE" || lvl == "JSON"
		}
	}
	return false
})
