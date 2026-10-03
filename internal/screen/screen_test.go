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

func TestDiscoverDefaultOrderLeadsWithNow(t *testing.T) {
	root := t.TempDir()
	cfgDir := filepath.Join(root, "config")
	dataDir := filepath.Join(root, "data")
	for _, n := range []string{"coins", "now", "sys", "zz-private"} {
		dir := filepath.Join(dataDir, "screens", n)
		if n == "zz-private" {
			dir = filepath.Join(cfgDir, "screens", n)
		}
		os.MkdirAll(dir, 0o755)
		os.WriteFile(filepath.Join(dir, "manifest.toml"), []byte("name = \""+n+"\"\n"), 0o644)
	}
	screens, err := Discover(cfgDir, dataDir, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(screens) != 4 {
		t.Fatalf("expected 4 screens, got %d", len(screens))
	}
	if screens[0].Name != "now" {
		t.Fatalf("blank-slate config should open on now, got %s", screens[0].Name)
	}
	if screens[3].Name != "zz-private" {
		t.Fatalf("unknown-to-DefaultOrder screens append at the end, got %s", screens[3].Name)
	}
}
