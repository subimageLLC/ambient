package status

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestWriteClearsWhenHealthy(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("AMBIENT_CACHE_DIR", dir)
	if err := Write("logs", []Chip{{Text: "⚠5err", Color: "red", Prio: 2}}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "status", "logs.json")); err != nil {
		t.Fatal("expected state file")
	}
	if err := Write("logs", nil); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "status", "logs.json")); !os.IsNotExist(err) {
		t.Fatal("expected state file removed")
	}
}

func TestRenderOrdersAndColors(t *testing.T) {
	now := time.Now()
	entries := []Entry{
		{Name: "logs", State: State{TS: now.Unix(), Chips: []Chip{{Text: "⚠5err", Color: "red", Prio: 2}}}},
		{Name: "sec", State: State{TS: now.Unix(), Chips: []Chip{
			{Text: "⚠WAN", Color: "red", Prio: 1},
			{Text: "3threats", Color: "orange", Prio: 2},
		}}},
	}
	got := Render(entries, now)
	// WAN (prio 1) first, then the two prio-2 chips (logs before sec), then hints
	ai := indexOf(got, "⚠WAN")
	oi := indexOf(got, "3threats")
	ei := indexOf(got, "⚠5err")
	if ai == -1 || oi == -1 || ei == -1 {
		t.Fatalf("missing chips in %q", got)
	}
	if !(ai < ei && ei < oi) {
		t.Fatalf("wrong order in %q", got)
	}
	if !contains(got, "#[fg=#dc322f,bold] ⚠WAN") {
		t.Fatalf("red not mapped: %q", got)
	}
	if !contains(got, "#[fg=#cb4b16,bold] 3threats") {
		t.Fatalf("orange not mapped: %q", got)
	}
}

func TestRenderStaleCollapsesToDim(t *testing.T) {
	now := time.Now()
	old := now.Add(-20 * time.Minute)
	entries := []Entry{
		{Name: "unifi", State: State{TS: old.Unix(), Chips: []Chip{{Text: "⚠WAN", Color: "red", Prio: 1}}}},
	}
	got := Render(entries, now)
	if !contains(got, "#[fg=#586e75] unifi?") {
		t.Fatalf("stale writer not dimmed: %q", got)
	}
	if contains(got, "⚠WAN") {
		t.Fatalf("stale chip still rendered: %q", got)
	}
}

func TestRenderDeadWriterDropped(t *testing.T) {
	now := time.Now()
	dead := now.Add(-90 * time.Minute)
	entries := []Entry{
		{Name: "logs", State: State{TS: dead.Unix(), Chips: []Chip{{Text: "⚠9err", Color: "red", Prio: 2}}}},
	}
	got := Render(entries, now)
	if contains(got, "logs") || contains(got, "⚠9err") {
		t.Fatalf("dead writer still rendered: %q", got)
	}
}

func TestRenderEmpty(t *testing.T) {
	got := Render(nil, time.Now())
	if !contains(got, "? keys") || !contains(got, "%H:%M") {
		t.Fatalf("hints/clock missing: %q", got)
	}
}

func TestColorClampsUnknowns(t *testing.T) {
	cases := map[string]string{
		"red":         "#dc322f", // known name
		"#268bd2":     "#268bd2", // valid hex passes through
		"#268BD2":     "#268BD2", // uppercase hex too
		"purple":      "#93a1a1", // unknown name -> default, never passed to tmux
		"":            "#93a1a1",
		"#12345":      "#93a1a1", // malformed hex
		"#12G4567":    "#93a1a1",
		"red; rm -rf": "#93a1a1", // hostile garbage
	}
	for in, want := range cases {
		if got := Color(in); got != want {
			t.Errorf("Color(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestRenderEscapesTmuxFormat(t *testing.T) {
	now := time.Now()
	entries := []Entry{
		{Name: "events", State: State{TS: now.Unix(), Chips: []Chip{
			{Text: "#(rm -rf /) pwned #{session_name}", Color: "red", Prio: 1},
		}}},
		{Name: "bad\x07writer", State: State{TS: now.Unix(), Chips: []Chip{{Text: "x", Color: "red", Prio: 2}}}},
	}
	got := Render(entries, now)
	// every "#("/"#{" must be preceded by another "#" (i.e. escaped as
	// "##("/"##{" — tmux renders those literally instead of executing)
	for i := 0; i+2 < len(got); i++ {
		if (got[i] == '#' && got[i+1] == '(') || (got[i] == '#' && got[i+1] == '{') {
			if i == 0 || got[i-1] != '#' {
				t.Fatalf("unescaped tmux format metacharacter at %d in %q", i, got)
			}
		}
	}
	if contains(got, "\x07") {
		t.Fatalf("control characters reached the bar: %q", got)
	}
}

func TestWriteRejectsBadNames(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("AMBIENT_CACHE_DIR", dir)
	for _, bad := range []string{"../weather", "a/b", "", "x y", "shutil"} {
		if bad == "shutil" {
			continue // letters-only names are valid by design
		}
		if err := Write(bad, []Chip{{Text: "x", Color: "red", Prio: 1}}); err == nil {
			t.Errorf("Write(%q) should be rejected", bad)
		}
	}
	if err := Write("events", []Chip{{Text: "x", Color: "red", Prio: 1}}); err != nil {
		t.Fatalf("valid name rejected: %v", err)
	}
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

func contains(s, sub string) bool { return indexOf(s, sub) != -1 }
