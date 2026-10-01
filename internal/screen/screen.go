package screen

import (
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/BurntSushi/toml"
)

type Manifest struct {
	Name          string
	Mode          string
	Entry         string
	Setup         string
	SetupOptional bool `toml:"setup_optional"`
	Refresh       int
	Needs         []string
	Glyphs        string
	Reads         []string
	Contacts      []string
}

type Screen struct {
	Manifest
	Dir     string
	Private bool
}

func (s Screen) Interactive() bool { return s.Mode == "interactive" }

func (s Screen) Command() string { return filepath.Join(s.Dir, s.Entry) }

func LoadManifest(dir string) (Manifest, error) {
	var m Manifest
	b, err := os.ReadFile(filepath.Join(dir, "manifest.toml"))
	if err != nil {
		return m, err
	}
	if err := toml.Unmarshal(b, &m); err != nil {
		return m, fmt.Errorf("%s: %w", dir, err)
	}
	if m.Name == "" {
		m.Name = filepath.Base(dir)
	}
	switch m.Mode {
	case "interactive", "passive":
	default:
		m.Mode = "passive"
	}
	if m.Entry == "" {
		m.Entry = "screen.sh"
	}
	if m.Refresh <= 0 {
		m.Refresh = 60
	}
	return m, nil
}

func SyncEmbedded(embedded embed.FS, dest string) error {
	root := "screens"
	return fs.WalkDir(embedded, root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dest, rel)
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		data, err := embedded.ReadFile(path)
		if err != nil {
			return err
		}
		mode := os.FileMode(0o644)
		if strings.HasSuffix(rel, ".sh") || strings.HasSuffix(rel, ".py") {
			mode = 0o755
		}
		if err := os.WriteFile(target, data, mode); err != nil {
			return err
		}
		return nil
	})
}

func Discover(configDir, dataDir string, enabled []string) ([]Screen, error) {
	byName := map[string]Screen{}
	var order []string

	embeddedDir := filepath.Join(dataDir, "screens")
	dirs := []struct {
		base    string
		private bool
	}{
		{embeddedDir, false},
		{filepath.Join(configDir, "screens"), true},
	}
	for _, d := range dirs {
		entries, err := os.ReadDir(d.base)
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				continue
			}
			return nil, err
		}
		var names []string
		for _, e := range entries {
			if e.IsDir() {
				names = append(names, e.Name())
			}
		}
		sort.Strings(names)
		for _, n := range names {
			dir := filepath.Join(d.base, n)
			m, err := LoadManifest(dir)
			if err != nil {
				fmt.Fprintf(os.Stderr, "ambient: skipping screen %s: %v\n", dir, err)
				continue
			}
			if m.Name == "ticker" {
				continue
			}
			if _, seen := byName[m.Name]; !seen {
				order = append(order, m.Name)
			}
			byName[m.Name] = Screen{Manifest: m, Dir: dir, Private: d.private}
		}
	}

	var out []Screen
	if len(enabled) > 0 {
		for _, want := range enabled {
			s, ok := byName[want]
			if !ok {
				fmt.Fprintf(os.Stderr, "ambient: unknown screen %q (run `ambient screens` to list)\n", want)
				continue
			}
			out = append(out, s)
		}
		if len(out) == 0 {
			return nil, fmt.Errorf("no enabled screens found")
		}
		return out, nil
	}
	for _, n := range order {
		out = append(out, byName[n])
	}
	return out, nil
}

func (s Screen) JSON() string {
	b, _ := json.Marshal(struct {
		Name     string   `json:"name"`
		Mode     string   `json:"mode"`
		Private  bool     `json:"private"`
		Refresh  int      `json:"refresh"`
		Needs    []string `json:"needs,omitempty"`
		Reads    []string `json:"reads,omitempty"`
		Contacts []string `json:"contacts,omitempty"`
	}{s.Name, s.Mode, s.Private, s.Refresh, s.Needs, s.Reads, s.Contacts})
	return string(b)
}
