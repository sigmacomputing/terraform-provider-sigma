package client

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

type fakeEntry struct {
	ID string `json:"id"`
}

// TestListAllPaged_DrainsCursor verifies the helper pages through cursor-based
// responses until nextPage is empty.
func TestListAllPaged_DrainsCursor(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		page := r.URL.Query().Get("page")
		if got := r.URL.Query().Get("limit"); got != "1000" {
			t.Errorf("limit = %q, want 1000", got)
		}
		w.Header().Set("Content-Type", "application/json")
		switch page {
		case "":
			_ = json.NewEncoder(w).Encode(Page[fakeEntry]{
				Entries:  []fakeEntry{{ID: "a"}, {ID: "b"}},
				NextPage: "cursor1",
			})
		case "cursor1":
			_ = json.NewEncoder(w).Encode(Page[fakeEntry]{
				Entries:  []fakeEntry{{ID: "c"}},
				NextPage: "",
			})
		default:
			t.Fatalf("unexpected cursor %q", page)
		}
	}))
	defer srv.Close()

	c := newTestClientNoAuth(srv.URL)
	got, err := ListAllPaged[fakeEntry](context.Background(), c, "/items", url.Values{})
	if err != nil {
		t.Fatal(err)
	}
	if calls != 2 {
		t.Errorf("expected 2 calls (page 1 + page 2), got %d", calls)
	}
	want := []string{"a", "b", "c"}
	if len(got) != len(want) {
		t.Fatalf("got %d entries, want %d", len(got), len(want))
	}
	for i, e := range got {
		if e.ID != want[i] {
			t.Errorf("entries[%d].ID = %q, want %q", i, e.ID, want[i])
		}
	}
}

// newTestClientNoAuth builds a minimal *Client that talks to a test server
// without OAuth (tests don't exercise the auth layer).
func newTestClientNoAuth(baseURL string) *Client {
	rc := &Client{
		base: baseURL,
	}
	// Build a retryablehttp client with a default transport; we bypass authTransport.
	rc.http = newPlainRetryableClient()
	return rc
}

// TestListAllPageToken_DrainsTokens verifies pageToken/nextPageToken paging.
func TestListAllPageToken_DrainsTokens(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.Header().Set("Content-Type", "application/json")
		if got := r.URL.Query().Get("pageSize"); got != "500" {
			t.Errorf("pageSize = %q, want 500", got)
		}
		switch tok := r.URL.Query().Get("pageToken"); tok {
		case "":
			_ = json.NewEncoder(w).Encode(PageToken[fakeEntry]{Entries: []fakeEntry{{ID: "a"}}, NextPageToken: "t 1"})
		case "t 1":
			_ = json.NewEncoder(w).Encode(PageToken[fakeEntry]{Entries: []fakeEntry{{ID: "b"}}})
		default:
			t.Fatalf("unexpected token %q", tok)
		}
	}))
	defer srv.Close()

	got, err := ListAllPageToken[fakeEntry](context.Background(), newTestClientNoAuth(srv.URL), "/items", PageSize(500))
	if err != nil {
		t.Fatal(err)
	}
	if calls != 2 || len(got) != 2 || got[0].ID != "a" || got[1].ID != "b" {
		t.Errorf("calls=%d got=%+v", calls, got)
	}
}

// TestForEachPaged_KeepsCallerLimitAndStopsEarly checks an explicit limit wins
// and stop=true skips the remaining pages.
func TestForEachPaged_KeepsCallerLimitAndStopsEarly(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if got := r.URL.Query().Get("limit"); got != "5" {
			t.Errorf("limit = %q, want 5", got)
		}
		_ = json.NewEncoder(w).Encode(Page[fakeEntry]{Entries: []fakeEntry{{ID: "a"}, {ID: "b"}}, NextPage: "more"})
	}))
	defer srv.Close()

	var seen []string
	err := ForEachPaged(context.Background(), newTestClientNoAuth(srv.URL), "/items", url.Values{"limit": {"5"}}, func(e fakeEntry) (bool, error) {
		seen = append(seen, e.ID)
		return e.ID == "a", nil
	})
	if err != nil || calls != 1 || len(seen) != 1 {
		t.Errorf("err=%v calls=%d seen=%v", err, calls, seen)
	}
}

// TestListAllPageToken_NilExtraSendsNoParams covers endpoints that reject
// unknown query params (jwkSets keys).
func TestListAllPageToken_NilExtraSendsNoParams(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.RawQuery != "" {
			t.Errorf("query = %q, want none", r.URL.RawQuery)
		}
		_ = json.NewEncoder(w).Encode(PageToken[fakeEntry]{Entries: []fakeEntry{{ID: "a"}}})
	}))
	defer srv.Close()

	got, err := ListAllPageToken[fakeEntry](context.Background(), newTestClientNoAuth(srv.URL), "/items", nil)
	if err != nil || len(got) != 1 {
		t.Errorf("err=%v got=%v", err, got)
	}
}
