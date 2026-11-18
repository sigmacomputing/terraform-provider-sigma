package client

import (
	"context"
	"fmt"
	"net/http"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/clientcredentials"
)

// Sigma's /v2/auth/token expects credentials in the JSON body, not in the
// standard OAuth2 form-encoded body. We construct a minimal token source that
// posts {"grant_type":"client_credentials","client_id":..,"client_secret":..}
// and parses Sigma's response shape.
//
// We use clientcredentials.Config wrapped in oauth2.ReuseTokenSource so the
// token is shared across all parallel resources for the lifetime of the
// provider. The standard library handles refresh-on-expiry automatically.

// newTokenSource returns an oauth2.TokenSource that fetches client-credentials
// tokens from Sigma and caches them via ReuseTokenSource.
func newTokenSource(ctx context.Context, baseURL, clientID, clientSecret string) oauth2.TokenSource {
	cfg := &clientcredentials.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		TokenURL:     baseURL + "/v2/auth/token",
		AuthStyle:    oauth2.AuthStyleInParams,
	}
	// ReuseTokenSource caches the token until shortly before expiry, then refreshes.
	return oauth2.ReuseTokenSource(nil, cfg.TokenSource(ctx))
}

// authTransport injects the Authorization header on every request, fetching a
// token from the wrapped TokenSource if needed. It is the *innermost*
// transport — wrapped by the retryablehttp transport so that retries reuse the
// same token.
type authTransport struct {
	src          oauth2.TokenSource
	base         http.RoundTripper
	scheme, host string // API origin; the token is only sent there
}

func (t *authTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if req.URL.Scheme != t.scheme || req.URL.Host != t.host {
		return nil, fmt.Errorf("%w: refusing to send credentials to %s://%s", errOffOrigin, req.URL.Scheme, req.URL.Host)
	}
	tok, err := t.src.Token()
	if err != nil {
		return nil, fmt.Errorf("sigma: fetching access token: %w", err)
	}
	// Clone the request so we don't mutate the caller's headers.
	req2 := req.Clone(req.Context())
	req2.Header.Set("Authorization", "Bearer "+tok.AccessToken)
	return t.base.RoundTrip(req2)
}
