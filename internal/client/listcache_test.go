package client

import (
	"sync"
	"sync/atomic"
	"testing"
)

func TestCachedList_LoadsOncePerKey(t *testing.T) {
	c := &Client{}
	var calls int32
	load := func() ([]string, error) {
		atomic.AddInt32(&calls, 1)
		return []string{"a", "b"}, nil
	}
	for i := 0; i < 5; i++ {
		got, err := CachedList(c, "k", load)
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != 2 || got[0] != "a" {
			t.Fatalf("unexpected result %v", got)
		}
	}
	if n := atomic.LoadInt32(&calls); n != 1 {
		t.Fatalf("expected one load, got %d", n)
	}
}

func TestCachedList_SeparateKeys(t *testing.T) {
	c := &Client{}
	var calls int32
	mk := func(v string) func() ([]string, error) {
		return func() ([]string, error) { atomic.AddInt32(&calls, 1); return []string{v}, nil }
	}
	if _, err := CachedList(c, "k1", mk("x")); err != nil {
		t.Fatal(err)
	}
	if _, err := CachedList(c, "k2", mk("y")); err != nil {
		t.Fatal(err)
	}
	if n := atomic.LoadInt32(&calls); n != 2 {
		t.Fatalf("expected one load per key (2), got %d", n)
	}
}

func TestCachedList_Invalidate(t *testing.T) {
	c := &Client{}
	var calls int32
	load := func() ([]string, error) { atomic.AddInt32(&calls, 1); return []string{"a"}, nil }
	if _, err := CachedList(c, "k", load); err != nil {
		t.Fatal(err)
	}
	InvalidateList(c, "k")
	if _, err := CachedList(c, "k", load); err != nil {
		t.Fatal(err)
	}
	if n := atomic.LoadInt32(&calls); n != 2 {
		t.Fatalf("expected reload after invalidate (2), got %d", n)
	}
}

func TestCachedList_ErrorNotCached(t *testing.T) {
	c := &Client{}
	var calls int32
	load := func() ([]string, error) {
		if atomic.AddInt32(&calls, 1) == 1 {
			return nil, errTest
		}
		return []string{"a"}, nil
	}
	if _, err := CachedList(c, "k", load); err == nil {
		t.Fatal("expected first call to error")
	}
	got, err := CachedList(c, "k", load)
	if err != nil {
		t.Fatalf("expected retry to succeed, got %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("unexpected result %v", got)
	}
	if n := atomic.LoadInt32(&calls); n != 2 {
		t.Fatalf("expected 2 loads (error not cached), got %d", n)
	}
}

func TestCachedList_ConcurrentCoalesces(t *testing.T) {
	c := &Client{}
	var calls int32
	load := func() ([]int, error) { atomic.AddInt32(&calls, 1); return []int{1, 2, 3}, nil }
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := CachedList(c, "k", load); err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		}()
	}
	wg.Wait()
	if n := atomic.LoadInt32(&calls); n != 1 {
		t.Fatalf("expected concurrent calls to coalesce to 1 load, got %d", n)
	}
}
