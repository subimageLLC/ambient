package run

import (
	"fmt"
	"math"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"ambient/internal/screen"
)

func Screen(dir string) error {
	m, err := screen.LoadManifest(dir)
	if err != nil {
		return err
	}
	entry := filepath.Join(dir, m.Entry)
	refresh := time.Duration(m.Refresh) * time.Second
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	fails := 0
	for {
		fmt.Print("\033[2J\033[H")
		cmd := exec.Command(entry)
		cmd.Env = os.Environ()
		cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
		err := cmd.Run()
		wait := refresh
		if err != nil {
			fails++
			back := refresh * time.Duration(math.Pow(2, float64(min(fails, 6))))
			if back > 10*time.Minute {
				back = 10 * time.Minute
			}
			wait = back
			fmt.Fprintf(os.Stdout, "\n\n  screen error (%v) — retry in %s\n", err, back)
		} else {
			fails = 0
		}
		select {
		case <-sigs:
			return nil
		case <-time.After(wait):
		}
	}
}

func Ticker(secs int, rotate func() error, alive func() bool) error {
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	for {
		wait := time.Duration(secs) * time.Second
		select {
		case <-sigs:
			return nil
		case <-time.After(wait):
		}
		if !alive() {
			return nil
		}
		if err := rotate(); err != nil {
			fmt.Fprintln(os.Stderr, "rotate:", err)
		}
	}
}
