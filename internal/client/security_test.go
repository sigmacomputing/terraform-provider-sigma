package client

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"golang.org/x/oauth2"
)

func TestNew_RequiresHTTPS(t *testing.T) {
	for base, ok := range map[string]bool{
		"https://api.sigmacomputing.com": true,
		"http://localhost:8080":          true,
		"http://127.0.0.1:9000":          true,
		"http://api.sigmacomputing.com":  false,
		"api.sigmacomputing.com":         false,
	} {
		_, err := New(context.Background(), Config{BaseURL: base, ClientID: "id", ClientSecret: "secret"})
		if (err == nil) != ok {
			t.Errorf("base %q: err=%v, want ok=%v", base, err, ok)
		}
	}
}

// TestAuthTransport_NoTokenOffHost checks a redirect to another host never
// receives the bearer token.
func TestAuthTransport_NoTokenOffHost(t *testing.T) {
	var leaked bool
	other := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		leaked = r.Header.Get("Authorization") != ""
	}))
	defer other.Close()
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, other.URL+"/steal", http.StatusFound)
	}))
	defer api.Close()

	c, err := New(context.Background(), Config{BaseURL: api.URL, ClientID: "id", ClientSecret: "secret"})
	if err != nil {
		t.Fatal(err)
	}
	c.http.RetryMax = 0
	c.http.HTTPClient.Transport.(*authTransport).src = oauth2.StaticTokenSource(&oauth2.Token{AccessToken: "tok"})

	err = c.Do(context.Background(), "GET", "/v2/whoami", nil, nil)
	if err == nil || !strings.Contains(err.Error(), "redirect") {
		t.Errorf("err = %v, want refused redirect", err)
	}
	if leaked {
		t.Error("token sent to redirect target")
	}
}

// TestTokenFetch_NoSecretOffHost checks a 307 on the token endpoint doesn't
// re-post the client secret to another host.
func TestTokenFetch_NoSecretOffHost(t *testing.T) {
	var leaked bool
	other := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		leaked = true
	}))
	defer other.Close()
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, other.URL+"/steal", http.StatusTemporaryRedirect)
	}))
	defer api.Close()

	c, err := New(context.Background(), Config{BaseURL: api.URL, ClientID: "id", ClientSecret: "secret"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := c.tokSrc.Token(); err == nil {
		t.Error("token fetch succeeded, want refused redirect")
	}
	if leaked {
		t.Error("token request sent to redirect target")
	}
}

// TestAuthTransport_NoDowngrade checks a same-host https->http redirect is
// refused without retrying.
func TestAuthTransport_NoDowngrade(t *testing.T) {
	var hits int
	api := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		http.Redirect(w, r, "http://"+r.Host+"/plain", http.StatusFound)
	}))
	defer api.Close()

	c, err := New(context.Background(), Config{BaseURL: api.URL, ClientID: "id", ClientSecret: "secret"})
	if err != nil {
		t.Fatal(err)
	}
	at := c.http.HTTPClient.Transport.(*authTransport)
	at.src = oauth2.StaticTokenSource(&oauth2.Token{AccessToken: "tok"})
	at.base = api.Client().Transport

	err = c.Do(context.Background(), "GET", "/v2/whoami", nil, nil)
	if err == nil || !strings.Contains(err.Error(), "refusing redirect") {
		t.Errorf("err = %v, want refused redirect", err)
	}
	if hits != 1 {
		t.Errorf("hits = %d, want 1 (no retries)", hits)
	}
}
