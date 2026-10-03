package session

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/creack/pty"

	"ambient/internal/cfg"
	"ambient/internal/screen"
	"ambient/internal/status"
)

// Name is the tmux session ambient builds. AMBIENT_SESSION overrides it —
// used for side-by-side trials and tests so a live session is never touched.
func Name() string {
	if n := os.Getenv("AMBIENT_SESSION"); n != "" {
		return n
	}
	return "ambient"
}

type Options struct {
	Screens []screen.Screen
	Config  cfg.Config
	Exe     string
}

func flagPath() string { return cfg.CacheDir() + "/ambient-pause" }

func Paused() bool {
	_, err := os.Stat(flagPath())
	return err == nil
}

func TogglePause() (bool, error) {
	if Paused() {
		if err := os.Remove(flagPath()); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return true, err
		}
		return false, nil
	}
	if err := os.MkdirAll(cfg.CacheDir(), 0o755); err != nil {
		return false, err
	}
	f, err := os.OpenFile(flagPath(), os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return false, err
	}
	f.Close()
	return true, nil
}

func termBroken() bool {
	t := os.Getenv("TERM")
	return t == "" || t == "dumb"
}

func fixEnv(env []string) []string {
	if !termBroken() {
		return env
	}
	out := make([]string, 0, len(env)+1)
	for _, e := range env {
		if !strings.HasPrefix(e, "TERM=") {
			out = append(out, e)
		}
	}
	return append(out, "TERM=xterm-256color")
}

func run(args []string, withPty bool) error {
	cmd := exec.Command("tmux", args...)
	cmd.Env = fixEnv(os.Environ())
	if withPty {
		pt, err := pty.Start(cmd)
		if err != nil {
			return err
		}
		go io.Copy(io.Discard, pt)
		err = cmd.Wait()
		pt.Close()
		return err
	}
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	return cmd.Run()
}

func out(args []string) (string, error) {
	cmd := exec.Command("tmux", args...)
	b, err := cmd.Output()
	return string(b), err
}

func q(s string) string { return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'" }

func (o Options) envPrefix() string {
	e := []string{
		"AMBIENT_CONFIG_DIR=" + q(cfg.Dir()),
		"AMBIENT_DATA_DIR=" + q(cfg.DataDir()),
		"AMBIENT_CACHE_DIR=" + q(cfg.CacheDir()),
		"AMBIENT_GLYPHS=" + q(o.Config.Glyphs),
		"AMBIENT_VERSION=" + q(o.version()),
		"AMBIENT_EXE=" + q(o.Exe),
	}
	return strings.Join(e, " ")
}

func (o Options) version() string {
	if v := os.Getenv("AMBIENT_VERSION"); v != "" {
		return v
	}
	return "dev"
}

func (o Options) screenCommand(s screen.Screen) string {
	env := o.envPrefix()
	if s.Interactive() {
		return env + " " + q(s.Command())
	}
	return env + " " + q(o.Exe) + " run " + q(s.Dir)
}

// paneCommand builds the command for an extra pane. Panes always own their
// loop — they run raw, never under `ambient run`.
func (o Options) paneCommand(s screen.Screen, p screen.Pane) string {
	return o.envPrefix() + " " + q(filepath.Join(s.Dir, p.Entry))
}

func (o Options) openWindow(name, cmd string, first bool) error {
	args := []string{"new-window", "-d", "-t", Name(), "-n", name, cmd}
	if first {
		args = []string{"new-session", "-d", "-s", Name(), "-n", name, cmd}
	}
	pty := first // fresh server may need a real terminal
	if err := run(args, pty); err != nil {
		return fmt.Errorf("tmux %s %s: %w", args[0], name, err)
	}
	return nil
}

func (o Options) openPanes(s screen.Screen) error {
	for _, p := range s.Panes {
		args := []string{
			"split-window", "-d",
			"-t", Name() + ":" + s.Name,
			"-" + p.Split,
			"-p", strconv.Itoa(p.Percent),
			o.paneCommand(s, p),
		}
		if err := run(args, false); err != nil {
			return fmt.Errorf("tmux split-window %s/%s: %w", s.Name, p.Entry, err)
		}
	}
	return nil
}

func Has() bool {
	_, err := out([]string{"has-session", "-t", Name()})
	return err == nil
}

func Kill() error {
	return run([]string{"kill-session", "-t", Name()}, false)
}

func Build(o Options) error {
	if len(o.Screens) == 0 {
		return fmt.Errorf("no screens to build")
	}
	first := o.Screens[0]
	if err := o.openWindow(first.Name, o.screenCommand(first), true); err != nil {
		return err
	}
	if err := o.openPanes(first); err != nil {
		return err
	}
	for _, s := range o.Screens[1:] {
		if err := o.openWindow(s.Name, o.screenCommand(s), false); err != nil {
			return err
		}
		if err := o.openPanes(s); err != nil {
			return err
		}
	}
	styling := [][]string{
		{"set-option", "-t", Name(), "mouse", "on"},
		{"set-option", "-t", Name(), "status", "on"},
		{"set-option", "-t", Name(), "status-interval", "5"},
		{"set-option", "-t", Name(), "status-position", "bottom"},
		{"set-option", "-t", Name(), "status-left-length", "40"},
		{"set-option", "-t", Name(), "status-right-length", "120"},
		{"set-option", "-t", Name(), "status-style", "bg=#002b36,fg=#93a1a1"},
		{"set-option", "-t", Name(), "status-left", "#[fg=#002b36,bg=#2aa198,bold] ambient #[default]"},
		{"set-option", "-t", Name(), "status-right", "#[fg=#93a1a1,bold]? keys #[default]#[fg=#586e75]· #[default]#[fg=#93a1a1,bold]* setup #[default]#[fg=#586e75]· %H:%M %d-%b "},
		{"set-option", "-t", Name(), "-a", "update-environment", " COLORFGBG"},
		{"set-option", "-t", Name(), "detach-on-destroy", "off"},
	}
	for _, args := range styling {
		if err := run(args, false); err != nil {
			return err
		}
	}
	if err := o.bindKeys(); err != nil {
		return err
	}
	// chips exist immediately after a build, not just after the first tick —
	// the kiosk's original builder lost them on every rebuild
	if err := RenderStatus(); err != nil {
		return err
	}
	if list, err := out([]string{"list-windows", "-t", Name(), "-F", "#{window_index}"}); err == nil {
		for _, idx := range strings.Fields(list) {
			t := Name() + ":" + idx
			run([]string{"set-window-option", "-t", t, "remain-on-exit", "on"}, false)
			run([]string{"set-window-option", "-t", t, "window-size", "latest"}, false)
			run([]string{"set-window-option", "-t", t, "window-status-style", "fg=#586e75"}, false)
			run([]string{"set-window-option", "-t", t, "window-status-current-style", "fg=#002b36,bg=#2aa198,bold"}, false)
		}
	}
	return nil
}

// bindKeys wires ambient's keys without touching what other sessions see.
// In tmux, key tables are server-global and the key-table option selects a
// table consulted for BARE keypresses, so a custom table would hijack plain
// letters from every pane app. Instead the default root/prefix tables stay
// in place and only the keys ambient changes are bound, gated per-session:
// inside ambient they run our command, elsewhere they fall back to the
// stock binding. n/C-n/d/0-9 need no binds — the stock bindings already do
// what ambient wants.
func (o Options) bindKeys() error {
	gate := "#{==:#{session_name}," + Name() + "}"
	pauseCmd := "run-shell " + q(q(o.Exe)+" pause")
	helpCmd := "display-popup -w 64 -h 20 -E " + q(q(o.Exe)+" help --esc")
	setupCmd := "display-popup -w 80 -h 16 -E AMBIENT_POPUP=1 " + q(q(o.Exe)+" setup-here")
	binds := [][]string{
		{"bind-key", "-T", "prefix", "N", "if-shell", "-F", gate, "previous-window", ""},
		{"bind-key", "-T", "prefix", "C-N", "if-shell", "-F", gate, "previous-window", ""},
		{"bind-key", "-T", "prefix", "p", "if-shell", "-F", gate, pauseCmd, "previous-window"},
		{"bind-key", "-T", "prefix", "?", "if-shell", "-F", gate, helpCmd, "list-keys -N"},
		{"bind-key", "-T", "prefix", "*", "if-shell", "-F", gate, setupCmd, ""},
		{"bind-key", "-n", "MouseDown1StatusRight", "if-shell", "-F", gate, helpCmd, ""},
		{"bind-key", "-n", "MouseDown3StatusRight", "if-shell", "-F", gate, setupCmd, ""},
	}
	for _, b := range binds {
		if err := run(b, false); err != nil {
			return err
		}
	}
	return nil
}

// RenderStatus applies the status-bar chips (state files under
// ~/.cache/ambient/status/) to this session's status-right.
func RenderStatus() error {
	rendered := status.Render(status.Read(), time.Now())
	return run([]string{"set-option", "-t", Name(), "status-right", rendered}, false)
}

func Attach() error {
	if !Has() {
		return fmt.Errorf("no ambient session — run `ambient start`")
	}
	cmd := exec.Command("tmux", "attach", "-t", Name())
	cmd.Env = fixEnv(os.Environ())
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	return cmd.Run()
}

func Rotate(force bool) error {
	if !Has() {
		return fmt.Errorf("no ambient session")
	}
	if !force && Paused() {
		return nil
	}
	return run([]string{"next-window", "-t", Name()}, false)
}

func SpawnTicker(exe string, secs int) error {
	pidFile := cfg.CacheDir() + "/ticker.pid"
	if b, err := os.ReadFile(pidFile); err == nil {
		if pid, err := strconv.Atoi(strings.TrimSpace(string(b))); err == nil && tickerRunning(pid) {
			return nil
		}
	}
	os.MkdirAll(cfg.CacheDir(), 0o755)
	logF, err := os.OpenFile(cfg.CacheDir()+"/ticker.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	cmd := exec.Command(exe, "ticker", strconv.Itoa(secs))
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	cmd.Stdout, cmd.Stderr = logF, logF
	if err := cmd.Start(); err != nil {
		logF.Close()
		return err
	}
	os.WriteFile(pidFile, []byte(strconv.Itoa(cmd.Process.Pid)), 0o644)
	logF.Close()
	return nil
}

func tickerRunning(pid int) bool {
	p, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	if err := p.Signal(syscall.Signal(0)); err != nil {
		if !errors.Is(err, syscall.EPERM) {
			return false
		}
	}
	b, err := exec.Command("ps", "-p", strconv.Itoa(pid), "-o", "command=").Output()
	if err != nil {
		return true
	}
	return strings.Contains(string(b), "ticker")
}
