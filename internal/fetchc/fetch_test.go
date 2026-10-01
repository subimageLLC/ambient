package fetchc

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func setup(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	os.Setenv("AMBIENT_CACHE_DIR", dir)
	t.Cleanup(func() { os.Unsetenv("AMBIENT_CACHE_DIR") })
}

func TestFetchFreshThenCache(t *testing.T) {
	setup(t)
	hits := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		w.Write([]byte(`{"n":` + string(rune('0'+hits)) + `}`))
	}))
	defer srv.Close()

	res, err := Fetch(srv.URL, "t1", time.Minute, 5*time.Second, "test")
	if err != nil || res.Stale {
		t.Fatalf("first fetch: %v stale=%v", err, res.Stale)
	}
	res, err = Fetch(srv.URL, "t1", time.Minute, 5*time.Second, "test")
	if err != nil {
		t.Fatalf("cached fetch: %v", err)
	}
	if hits != 1 {
		t.Fatalf("expected 1 request, got %d", hits)
	}
	if string(res.Body) != `{"n":1}` {
		t.Fatalf("expected cached body, got %s", res.Body)
	}
}

func TestFetch429BackoffAndStale(t *testing.T) {
	setup(t)
	ok := true
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if ok {
			w.Write([]byte("good"))
			ok = false
			return
		}
		w.Header().Set("Retry-After", "120")
		w.WriteHeader(429)
	}))
	defer srv.Close()

	if _, err := Fetch(srv.URL, "t2", time.Millisecond, 5*time.Second, "test"); err != nil {
		t.Fatalf("prime: %v", err)
	}
	time.Sleep(5 * time.Millisecond)
	if _, err := Fetch(srv.URL, "t2", time.Millisecond, 5*time.Second, "test"); err == nil {
		t.Fatal("429 should error")
	}
	b, err := os.ReadFile(filepath.Join(dir(), "t2.state.json"))
	if err != nil {
		t.Fatalf("state file missing: %v", err)
	}
	if string(b) == "" {
		t.Fatal("empty state")
	}

	res, err := Fetch(srv.URL, "t2", time.Millisecond, 5*time.Second, "test")
	if err != nil {
		t.Fatalf("backoff window should serve stale without error: %v", err)
	}
	if res.Body == nil || string(res.Body) != "good" {
		t.Fatalf("stale body should be served, got %q err=%v", res.Body, err)
	}
	if !res.Stale {
		t.Fatal("stale flag not set")
	}
}

func TestFetchNothingCachedGivesNoBody(t *testing.T) {
	setup(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
	}))
	defer srv.Close()

	res, err := Fetch(srv.URL, "t3", time.Minute, 5*time.Second, "test")
	if err == nil {
		t.Fatal("500 should error")
	}
	if res.Body != nil {
		t.Fatal("no stale body expected")
	}
}
