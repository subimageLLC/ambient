package session

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"

	"github.com/creack/pty"

	"ambient/internal/cfg"
	"ambient/internal/screen"
)

const Name = "ambient"
const keyTable = "ambient"

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

func Has() bool {
	_, err := out([]string{"has-session", "-t", Name})
	return err == nil
}

func Kill() error {
	return run([]string{"kill-session", "-t", Name}, false)
}

func Build(o Options) error {
	if len(o.Screens) == 0 {
		return fmt.Errorf("no screens to build")
	}
	first := o.Screens[0]
	if err := run([]string{"new-session", "-d", "-s", Name, "-n", first.Name, o.screenCommand(first)}, true); err != nil {
		return fmt.Errorf("tmux new-session: %w", err)
	}
	for _, s := range o.Screens[1:] {
		if err := run([]string{"new-window", "-t", Name, "-n", s.Name, o.screenCommand(s)}, false); err != nil {
			return fmt.Errorf("tmux new-window %s: %w", s.Name, err)
		}
	}
	styling := [][]string{
		{"set-option", "-t", Name, "mouse", "on"},
		{"set-option", "-t", Name, "status", "on"},
		{"set-option", "-t", Name, "status-interval", "5"},
		{"set-option", "-t", Name, "status-position", "bottom"},
		{"set-option", "-t", Name, "status-left-length", "40"},
		{"set-option", "-t", Name, "status-right-length", "40"},
		{"set-option", "-t", Name, "status-style", "bg=#002b36,fg=#93a1a1"},
		{"set-option", "-t", Name, "status-left", "#[fg=#002b36,bg=#2aa198,bold] ambient #[default]"},
		{"set-option", "-t", Name, "status-right", "#[fg=#93a1a1,bold]? keys #[default]#[fg=#586e75]· #[default]#[fg=#93a1a1,bold]* setup #[default]#[fg=#586e75]· %H:%M %d-%b "},
		{"set-option", "-t", Name, "-a", "update-environment", " COLORFGBG"},
		{"set-option", "-t", Name, "detach-on-destroy", "off"},
		{"set-option", "-t", Name, "key-table", keyTable},
	}
	for _, args := range styling {
		if err := run(args, false); err != nil {
			return err
		}
	}
	if err := o.bindKeys(); err != nil {
		return err
	}
	if list, err := out([]string{"list-windows", "-t", Name, "-F", "#{window_index}"}); err == nil {
		for _, idx := range strings.Fields(list) {
			t := Name + ":" + idx
			run([]string{"set-window-option", "-t", t, "remain-on-exit", "on"}, false)
			run([]string{"set-window-option", "-t", t, "window-size", "latest"}, false)
			run([]string{"set-window-option", "-t", t, "window-status-style", "fg=#586e75"}, false)
			run([]string{"set-window-option", "-t", t, "window-status-current-style", "fg=#002b36,bg=#2aa198,bold"}, false)
		}
	}
	return nil
}

func (o Options) bindKeys() error {
	toggle := q(o.Exe) + " pause"
	helpCmd := q(o.Exe) + " help --esc"
	setupCmd := "AMBIENT_POPUP=1 " + q(o.Exe) + " setup-here"
	binds := [][]string{
		{"bind-key", "-T", keyTable, "d", "detach-client"},
		{"bind-key", "-T", keyTable, "n", "next-window"},
		{"bind-key", "-T", keyTable, "C-n", "next-window"},
		{"bind-key", "-T", keyTable, "N", "previous-window"},
		{"bind-key", "-T", keyTable, "C-N", "previous-window"},
		{"bind-key", "-T", keyTable, "p", "run-shell", toggle, "\\;", "display-message", "rotation: toggled"},
		{"bind-key", "-T", keyTable, "?", "display-popup", "-w", "64", "-h", "20", "-E", helpCmd},
		{"bind-key", "-T", keyTable, "*", "display-popup", "-w", "80", "-h", "16", "-E", setupCmd},
	}
	for i := 0; i <= 9; i++ {
		binds = append(binds, []string{"bind-key", "-T", keyTable, strconv.Itoa(i), "select-window", "-t", ":" + strconv.Itoa(i)})
	}
	gate := "#{==:#{session_name}," + Name + "}"
	binds = append(binds,
		[]string{"bind-key", "-n", "MouseDown1StatusRight", "if-shell", "-F", gate,
			"display-popup -w 64 -h 20 -E " + q(helpCmd), ""},
		[]string{"bind-key", "-n", "MouseDown3StatusRight", "if-shell", "-F", gate,
			"display-popup -w 80 -h 16 -E " + q(setupCmd), ""},
	)
	for _, b := range binds {
		if err := run(b, false); err != nil {
			return err
		}
	}
	return nil
}

func Attach() error {
	if !Has() {
		return fmt.Errorf("no ambient session — run `ambient start`")
	}
	cmd := exec.Command("tmux", "attach", "-t", Name)
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
	return run([]string{"next-window", "-t", Name}, false)
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
