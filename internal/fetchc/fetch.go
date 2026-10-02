package fetchc

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type state struct {
	RetryAt int64 `json:"retry_at"`
}

func dir() string {
	d := os.Getenv("AMBIENT_CACHE_DIR")
	if d == "" {
		home, _ := os.UserHomeDir()
		d = filepath.Join(home, ".cache", "ambient")
	}
	return filepath.Join(d, "fetch")
}

func cachePath(name string) string { return filepath.Join(dir(), name+".json") }
func statePath(name string) string { return filepath.Join(dir(), name+".state.json") }

type Result struct {
	Body  []byte
	Stale bool
}

func Fetch(url, name string, ttl, timeout time.Duration, ua string) (Result, error) {
	if strings.ContainsAny(name, "/\\") || strings.Contains(name, "..") {
		return Result{}, fmt.Errorf("invalid cache name %q", name)
	}
	os.MkdirAll(dir(), 0o755)
	if body, err := os.ReadFile(cachePath(name)); err == nil {
		if fi, err := os.Stat(cachePath(name)); err == nil && time.Since(fi.ModTime()) < ttl {
			return Result{Body: body}, nil
		}
	}
	var st state
	if b, err := os.ReadFile(statePath(name)); err == nil {
		json.Unmarshal(b, &st)
	}
	now := time.Now()
	if st.RetryAt > now.Unix() {
		if body, err := os.ReadFile(cachePath(name)); err == nil {
			return Result{Body: body, Stale: true}, nil
		}
		return Result{}, fmt.Errorf("backing off until %s", time.Unix(st.RetryAt, 0).Format("15:04:05"))
	}
	client := &http.Client{Timeout: timeout}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return Result{}, err
	}
	if ua == "" {
		ua = "ambient"
	}
	req.Header.Set("User-Agent", ua)
	resp, err := client.Do(req)
	if err != nil {
		fail(name, now, 10*time.Second)
		return Result{}, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 10<<20))
	if err != nil {
		fail(name, now, 10*time.Second)
		return Result{}, err
	}
	if resp.StatusCode == 429 {
		wait := 30 * time.Second
		if ra := resp.Header.Get("Retry-After"); ra != "" {
			if secs, err := strconv.Atoi(ra); err == nil && secs > 0 {
				wait = min(time.Duration(secs), time.Hour/time.Second) * time.Second
			}
		}
		wait = max(wait, 30*time.Second)
		fail(name, now, wait)
		return Result{}, fmt.Errorf("HTTP 429 — next try in %s", wait)
	}
	if resp.StatusCode >= 400 {
		fail(name, now, 10*time.Second)
		return Result{}, fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	os.WriteFile(cachePath(name), body, 0o644)
	os.Remove(statePath(name))
	return Result{Body: body}, nil
}

func fail(name string, now time.Time, wait time.Duration) {
	st := state{RetryAt: now.Add(wait).Unix()}
	b, _ := json.Marshal(st)
	os.WriteFile(statePath(name), b, 0o644)
}
