package client

import "sync"

// TeamMemberIDs returns the set of member IDs belonging to a team, listing the
// team's roster at most once per team for the lifetime of this *Client (one
// provider run). load is invoked only on a cache miss and should perform the
// paginated GET /v2/teams/{teamId}/members list, returning the member IDs.
//
// This is the fix for the O(N²) blow-up in sigma_team_membership: there is no
// single-membership read endpoint, so each membership's Read must consult the
// full roster. Without memoization, N membership resources each list the whole
// roster (N pages) — N² calls per team. With it, the roster is listed once and
// every membership Read for that team reuses the result.
//
// Concurrent first-reads for the same team are coalesced via a per-team lock so
// Terraform's parallel resource walk triggers a single list, not one per
// goroutine. Errors are never cached.
func (c *Client) TeamMemberIDs(teamID string, load func() ([]string, error)) (map[string]struct{}, error) {
	if set, ok := c.cachedRoster(teamID); ok {
		return set, nil
	}

	// Serialize cache-fill per team so only the first arrival lists; the rest
	// block here and then hit the cache on the double-check below.
	l := c.teamLock(teamID)
	l.Lock()
	defer l.Unlock()

	if set, ok := c.cachedRoster(teamID); ok {
		return set, nil
	}

	ids, err := load()
	if err != nil {
		return nil, err
	}
	set := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		set[id] = struct{}{}
	}

	c.rosterMu.Lock()
	if c.rosterCache == nil {
		c.rosterCache = make(map[string]map[string]struct{})
	}
	c.rosterCache[teamID] = set
	c.rosterMu.Unlock()
	return set, nil
}

// InvalidateTeamRoster drops a team's cached roster so the next TeamMemberIDs
// call re-lists. Called after a membership add/remove mutates the roster.
func (c *Client) InvalidateTeamRoster(teamID string) {
	c.rosterMu.Lock()
	delete(c.rosterCache, teamID)
	c.rosterMu.Unlock()
}

func (c *Client) cachedRoster(teamID string) (map[string]struct{}, bool) {
	c.rosterMu.Lock()
	defer c.rosterMu.Unlock()
	set, ok := c.rosterCache[teamID]
	return set, ok
}

func (c *Client) teamLock(teamID string) *sync.Mutex {
	c.rosterMu.Lock()
	defer c.rosterMu.Unlock()
	if c.rosterLocks == nil {
		c.rosterLocks = make(map[string]*sync.Mutex)
	}
	l, ok := c.rosterLocks[teamID]
	if !ok {
		l = &sync.Mutex{}
		c.rosterLocks[teamID] = l
	}
	return l
}
