package client

import (
	"sync"
	"sync/atomic"
	"testing"
)

func TestTeamMemberIDs_LoadsOncePerTeam(t *testing.T) {
	c := &Client{}
	var calls int32
	load := func() ([]string, error) {
		atomic.AddInt32(&calls, 1)
		return []string{"a", "b", "c"}, nil
	}

	for i := 0; i < 5; i++ {
		set, err := c.TeamMemberIDs("team1", load)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if _, ok := set["b"]; !ok {
			t.Fatalf("expected member b in roster, got %v", set)
		}
		if _, ok := set["zzz"]; ok {
			t.Fatalf("did not expect member zzz in roster")
		}
	}
	if got := atomic.LoadInt32(&calls); got != 1 {
		t.Fatalf("expected load to run once, ran %d times", got)
	}
}

func TestTeamMemberIDs_SeparateTeams(t *testing.T) {
	c := &Client{}
	var calls int32
	mk := func(id string) func() ([]string, error) {
		return func() ([]string, error) {
			atomic.AddInt32(&calls, 1)
			return []string{id}, nil
		}
	}
	if _, err := c.TeamMemberIDs("t1", mk("x")); err != nil {
		t.Fatal(err)
	}
	if _, err := c.TeamMemberIDs("t2", mk("y")); err != nil {
		t.Fatal(err)
	}
	if got := atomic.LoadInt32(&calls); got != 2 {
		t.Fatalf("expected one load per team (2), got %d", got)
	}
}

func TestTeamMemberIDs_Invalidate(t *testing.T) {
	c := &Client{}
	var calls int32
	load := func() ([]string, error) {
		atomic.AddInt32(&calls, 1)
		return []string{"a"}, nil
	}
	if _, err := c.TeamMemberIDs("t", load); err != nil {
		t.Fatal(err)
	}
	c.InvalidateTeamRoster("t")
	if _, err := c.TeamMemberIDs("t", load); err != nil {
		t.Fatal(err)
	}
	if got := atomic.LoadInt32(&calls); got != 2 {
		t.Fatalf("expected reload after invalidate (2), got %d", got)
	}
}

func TestTeamMemberIDs_ErrorNotCached(t *testing.T) {
	c := &Client{}
	var calls int32
	failOnce := func() ([]string, error) {
		if atomic.AddInt32(&calls, 1) == 1 {
			return nil, errTest
		}
		return []string{"a"}, nil
	}
	if _, err := c.TeamMemberIDs("t", failOnce); err == nil {
		t.Fatal("expected first call to error")
	}
	set, err := c.TeamMemberIDs("t", failOnce)
	if err != nil {
		t.Fatalf("expected second call to succeed, got %v", err)
	}
	if _, ok := set["a"]; !ok {
		t.Fatalf("expected member a after retry, got %v", set)
	}
	if got := atomic.LoadInt32(&calls); got != 2 {
		t.Fatalf("expected 2 loads (error not cached), got %d", got)
	}
}

func TestTeamMemberIDs_ConcurrentCoalesces(t *testing.T) {
	c := &Client{}
	var calls int32
	load := func() ([]string, error) {
		atomic.AddInt32(&calls, 1)
		return []string{"a", "b"}, nil
	}
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := c.TeamMemberIDs("team", load); err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		}()
	}
	wg.Wait()
	if got := atomic.LoadInt32(&calls); got != 1 {
		t.Fatalf("expected concurrent reads to coalesce to 1 load, got %d", got)
	}
}

type testErr struct{}

func (testErr) Error() string { return "boom" }

var errTest = testErr{}
