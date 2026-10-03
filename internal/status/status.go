// Package status implements the status-bar chip convention: screens (or any
// private script) drop JSON state files under ~/.cache/ambient/status/, and
// the ticker renders them into tmux's status-right. Silence means healthy —
// writers clear their file when there is nothing to say.
package status

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"ambient/internal/cfg"
)

const StaleAfter = 15 * time.Minute

// DeadAfter is when a silent writer stops rendering at all: 15 minutes of
// silence earns a dim "name?", an hour earns nothing. Dead screens must not
// haunt the bar forever.
const DeadAfter = time.Hour

type Chip struct {
	Text  string `json:"text"`
	Color string `json:"color"`
	Prio  int    `json:"prio"`
}

type State struct {
	TS    int64  `json:"ts"`
	Chips []Chip `json:"chips"`
}

type Entry struct {
	Name  string
	State State
}

var colors = map[string]string{
	"red":     "#dc322f",
	"orange":  "#cb4b16",
	"yellow":  "#b58900",
	"green":   "#859900",
	"cyan":    "#2aa198",
	"blue":    "#268bd2",
	"magenta": "#d33682",
	"violet":  "#6c71c4",
	"dim":     "#586e75",
	"default": "#93a1a1",
}

// Color resolves a chip color to a solarized hex. Known names map; valid
// #rrggbb passes through; anything else falls back to default so a typo'd
// chip can never inject an invalid tmux style and wedge the whole bar.
func Color(name string) string {
	if c, ok := colors[name]; ok {
		return c
	}
	if isHex(name) {
		return name
	}
	return colors["default"]
}

func isHex(s string) bool {
	if len(s) != 7 || s[0] != '#' {
		return false
	}
	for _, r := range s[1:] {
		switch {
		case r >= '0' && r <= '9', r >= 'a' && r <= 'f', r >= 'A' && r <= 'F':
		default:
			return false
		}
	}
	return true
}

func Dir() string { return filepath.Join(cfg.CacheDir(), "status") }

// Write replaces the state for one writer. An empty chip set removes the
// file — healthy writers go silent, not stale.
func Write(name string, chips []Chip) error {
	dir := Dir()
	if len(chips) == 0 {
		err := os.Remove(filepath.Join(dir, name+".json"))
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		return err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	st := State{TS: time.Now().Unix(), Chips: chips}
	b, err := json.Marshal(st)
	if err != nil {
		return err
	}
	tmp := filepath.Join(dir, name+".json.tmp")
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, filepath.Join(dir, name+".json"))
}

// Read loads every writer's state, skipping unreadable or malformed files.
func Read() []Entry {
	files, err := os.ReadDir(Dir())
	if err != nil {
		return nil
	}
	var out []Entry
	for _, f := range files {
		name := f.Name()
		if !strings.HasSuffix(name, ".json") {
			continue
		}
		b, err := os.ReadFile(filepath.Join(Dir(), name))
		if err != nil {
			continue
		}
		var st State
		if err := json.Unmarshal(b, &st); err != nil {
			continue
		}
		out = append(out, Entry{Name: strings.TrimSuffix(name, ".json"), State: st})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// Render turns entries into a tmux status-right string: chips sorted by
// priority (lower first, then writer name), stale writers collapse to a dim
// "name?" chip, and the clock trails. Mirrors the solarized kiosk bar.
func Render(entries []Entry, now time.Time) string {
	type flat struct {
		writer string
		chip   Chip
		stale  bool
	}
	var all []flat
	for _, e := range entries {
		age := now.Sub(time.Unix(e.State.TS, 0))
		if age > DeadAfter {
			continue
		}
		if age > StaleAfter {
			all = append(all, flat{writer: e.Name, stale: true})
			continue
		}
		for _, c := range e.State.Chips {
			all = append(all, flat{writer: e.Name, chip: c})
		}
	}
	sort.SliceStable(all, func(i, j int) bool {
		if all[i].stale != all[j].stale {
			return all[j].stale
		}
		if !all[i].stale && all[i].chip.Prio != all[j].chip.Prio {
			return all[i].chip.Prio < all[j].chip.Prio
		}
		return all[i].writer < all[j].writer
	})
	var b strings.Builder
	for _, f := range all {
		if f.stale {
			fmt.Fprintf(&b, "#[fg=%s] %s?#[default]", colors["dim"], f.writer)
			continue
		}
		fmt.Fprintf(&b, "#[fg=%s,bold] %s#[default]", Color(f.chip.Color), f.chip.Text)
	}
	b.WriteString(" #[fg=#93a1a1,bold]? keys #[default]#[fg=#586e75]· #[default]#[fg=#93a1a1,bold]* setup #[default]#[fg=#586e75]· %H:%M %d-%b ")
	return b.String()
}
