package screen

import (
	"os"
	"path/filepath"
	"testing"
)

func writeManifest(t *testing.T, body string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "manifest.toml"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestLoadManifestDefaults(t *testing.T) {
	dir := writeManifest(t, `name = "net"`+"\n"+`mode = "interactive"`+"\n")
	m, err := LoadManifest(dir)
	if err != nil {
		t.Fatal(err)
	}
	if m.Mode != "interactive" || m.Entry != "screen.sh" || m.Refresh != 60 {
		t.Fatalf("unexpected defaults: %+v", m)
	}
	if len(m.Panes) != 0 {
		t.Fatalf("expected no panes, got %d", len(m.Panes))
	}
}

func TestLoadManifestPanes(t *testing.T) {
	dir := writeManifest(t, `
name     = "logs"
mode     = "interactive"
entry    = "logsum.sh"

[[panes]]
entry   = "lnav.sh"
split   = "v"
percent = 85
`)
	m, err := LoadManifest(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(m.Panes) != 1 {
		t.Fatalf("expected 1 pane, got %d", len(m.Panes))
	}
	p := m.Panes[0]
	if p.Entry != "lnav.sh" || p.Split != "v" || p.Percent != 85 {
		t.Fatalf("unexpected pane: %+v", p)
	}
}

func TestLoadManifestPaneDefaults(t *testing.T) {
	dir := writeManifest(t, `
name = "net"
[[panes]]
entry = "tailnet.sh"
`)
	m, err := LoadManifest(dir)
	if err != nil {
		t.Fatal(err)
	}
	p := m.Panes[0]
	if p.Split != "v" || p.Percent != 50 {
		t.Fatalf("expected defaults v/50, got %s/%d", p.Split, p.Percent)
	}
}

func TestLoadManifestPaneErrors(t *testing.T) {
	cases := []string{
		"[[panes]]\nsplit = \"v\"\n",                // no entry
		"[[panes]]\nentry = \"x\"\nsplit = \"z\"\n", // bad split
		"[[panes]]\nentry = \"x\"\npercent = 95\n",  // out of range
	}
	for i, body := range cases {
		dir := writeManifest(t, "name = \"t\"\n"+body)
		if _, err := LoadManifest(dir); err == nil {
			t.Fatalf("case %d: expected error", i)
		}
	}
}
