package client

import "sync"

// CachedList returns the result of load(), computing it at most once per key for
// the lifetime of this *Client (one provider run). It is the generalization of
// the team-roster cache (see roster.go): many per-item resource Reads share one
// parent (or global) collection — e.g. every grant on a connection, every
// user-attribute assignment on a UA. Without memoization, N such resources each
// list the whole collection on refresh, which is O(N²) API calls and rate-limits
// out (the sigma_team_membership / issue #24 failure mode). With it, the
// collection is listed once per key and reused.
//
// Concurrent first-calls for the same key are coalesced via a per-key lock so
// Terraform's parallel resource walk triggers a single load, not one per
// goroutine. Errors are never cached.
//
// Only read paths should call CachedList. Mutating ops (Create/Update/Delete)
// must use uncached lists and call InvalidateList afterward, so a write is never
// served a stale collection.
func CachedList[T any](c *Client, key string, load func() ([]T, error)) ([]T, error) {
	if v, ok := c.cachedList(key); ok {
		return v.([]T), nil
	}

	l := c.listLock(key)
	l.Lock()
	defer l.Unlock()

	if v, ok := c.cachedList(key); ok {
		return v.([]T), nil
	}

	out, err := load()
	if err != nil {
		return nil, err
	}

	c.listMu.Lock()
	if c.listCache == nil {
		c.listCache = make(map[string]any)
	}
	c.listCache[key] = out
	c.listMu.Unlock()
	return out, nil
}

// InvalidateList drops a cached collection so the next CachedList re-loads.
// Call it after a write that changes the collection (add/remove an item).
func InvalidateList(c *Client, key string) {
	c.listMu.Lock()
	delete(c.listCache, key)
	c.listMu.Unlock()
}

func (c *Client) cachedList(key string) (any, bool) {
	c.listMu.Lock()
	defer c.listMu.Unlock()
	v, ok := c.listCache[key]
	return v, ok
}

func (c *Client) listLock(key string) *sync.Mutex {
	c.listMu.Lock()
	defer c.listMu.Unlock()
	if c.listLocks == nil {
		c.listLocks = make(map[string]*sync.Mutex)
	}
	l, ok := c.listLocks[key]
	if !ok {
		l = &sync.Mutex{}
		c.listLocks[key] = l
	}
	return l
}
