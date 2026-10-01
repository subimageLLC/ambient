package main

import (
	"bufio"
	"embed"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"ambient/internal/cfg"
	"ambient/internal/chartc"
	"ambient/internal/doctor"
	"ambient/internal/fetchc"
	"ambient/internal/run"
	"ambient/internal/screen"
	"ambient/internal/session"
)

//go:embed screens
var screensFS embed.FS

var version = "0.1.0"

func main() {
	if len(os.Args) < 2 {
		usage()
	}
	switch os.Args[1] {
	case "start":
		cmdStart(false)
	case "rebuild":
		cmdStart(true)
	case "attach":
		if err := session.Attach(); err != nil {
			die(err)
		}
	case "rotate":
		fs := flag.NewFlagSet("rotate", flag.ExitOnError)
		force := fs.Bool("force", false, "rotate even while paused")
		fs.Parse(os.Args[2:])
		if err := session.Rotate(*force); err != nil {
			die(err)
		}
	case "pause":
		now := session.TogglePause()
		state := "resumed"
		if now {
			state = "paused"
		}
		fmt.Println("rotation:", state)
	case "ticker":
		secs := 60
		if len(os.Args) > 2 {
			if n, err := strconv.Atoi(os.Args[2]); err == nil {
				secs = n
			}
		}
		if err := run.Ticker(secs, func() error { return session.Rotate(false) }, session.Has); err != nil {
			die(err)
		}
	case "run":
		if len(os.Args) < 3 {
			die(fmt.Errorf("usage: ambient run <screen-dir>"))
		}
		if err := run.Screen(os.Args[2]); err != nil {
			die(err)
		}
	case "fetch":
		cmdFetch(os.Args[2:])
	case "chart":
		cmdChart(os.Args[2:])
	case "doctor":
		c, _ := cfg.Load()
		screens := loadScreens(c)
		doctor.Run(c, screens)
	case "screens":
		c, _ := cfg.Load()
		for _, s := range loadScreens(c) {
			setup := ""
			if s.Setup != "" {
				setup = "  (ambient setup " + s.Name + ")"
			}
			fmt.Printf("%-12s %-12s refresh=%ds%s\n", s.Name, s.Mode, s.Refresh, setup)
		}
	case "setup":
		cmdSetup(os.Args[2:])
	case "version":
		fmt.Println("ambient", version)
	default:
		usage()
	}
}

func usage() {
	fmt.Fprintf(os.Stderr, `ambient %s — an ambient display for any unix

usage:
  ambient start [-d] [--no-rotate]    build the session (and attach)
  ambient rebuild [-d] [--no-rotate]  kill and rebuild the session
  ambient attach                      attach to the running session
  ambient rotate [--force]            advance to the next screen
  ambient pause                       toggle rotation pause
  ambient screens                     list available screens
  ambient setup <screen>              configure a screen (weather, net, ...)
  ambient doctor                      check environment, glyphs, privacy
  ambient fetch --ttl N --name N URL  cached fetch with rate-limit backoff
  ambient chart line|volume           render values (stdin) as braille/blocks
  ambient version
`, version)
	os.Exit(2)
}

func die(err error) {
	fmt.Fprintln(os.Stderr, "ambient:", err)
	os.Exit(1)
}

func loadScreens(c cfg.Config) []screen.Screen {
	if err := screen.SyncEmbedded(screensFS, filepath.Join(cfg.DataDir(), "screens")); err != nil {
		fmt.Fprintln(os.Stderr, "ambient: sync screens:", err)
	}
	screens, err := screen.Discover(cfg.Dir(), cfg.DataDir(), c.Screens)
	if err != nil {
		die(err)
	}
	return screens
}

func cmdSetup(args []string) {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "usage: ambient setup <screen>")
		fmt.Fprintln(os.Stderr, "configurable screens:")
		c, _ := cfg.Load()
		for _, s := range loadScreens(c) {
			if s.Setup != "" {
				fmt.Fprintf(os.Stderr, "  %s\n", s.Name)
			}
		}
		os.Exit(2)
	}
	c, _ := cfg.Load()
	screens := loadScreens(c)
	var target *screen.Screen
	for i := range screens {
		if screens[i].Name == args[0] {
			target = &screens[i]
			break
		}
	}
	if target == nil {
		die(fmt.Errorf("unknown screen %q", args[0]))
	}
	if target.Setup == "" {
		die(fmt.Errorf("screen %q has no setup", args[0]))
	}
	exe, _ := os.Executable()
	cmd := exec.Command(filepath.Join(target.Dir, target.Setup))
	cmd.Env = append(os.Environ(),
		"AMBIENT_CONFIG_DIR="+cfg.Dir(),
		"AMBIENT_DATA_DIR="+cfg.DataDir(),
		"AMBIENT_CACHE_DIR="+cfg.CacheDir(),
		"AMBIENT_EXE="+exe,
		"AMBIENT_VERSION="+version,
	)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	if err := cmd.Run(); err != nil {
		die(err)
	}
}

func cmdStart(rebuild bool) {
	fs := flag.NewFlagSet("start", flag.ExitOnError)
	detach := fs.Bool("d", false, "build without attaching")
	noRotate := fs.Bool("no-rotate", false, "do not spawn the rotation ticker")
	fs.Parse(os.Args[2:])

	c, err := cfg.Load()
	if err != nil {
		fmt.Fprintln(os.Stderr, "ambient: config:", err)
	}
	exe, err := os.Executable()
	if err != nil {
		die(err)
	}
	screens := loadScreens(c)

	if rebuild || !session.Has() {
		if session.Has() {
			session.Kill()
		}
		opts := session.Options{Screens: screens, Config: c, Exe: exe}
		os.Setenv("AMBIENT_VERSION", version)
		if err := session.Build(opts); err != nil {
			die(err)
		}
		fmt.Printf("ambient: session built with %d screens\n", len(screens))
	}
	if !*noRotate {
		if err := session.SpawnTicker(exe, c.RotationSecs); err != nil {
			fmt.Fprintln(os.Stderr, "ambient: ticker:", err)
		}
	}
	if !*detach {
		if err := session.Attach(); err != nil {
			die(err)
		}
	}
}

func cmdFetch(args []string) {
	fs := flag.NewFlagSet("fetch", flag.ExitOnError)
	ttl := fs.Int("ttl", 300, "cache lifetime in seconds")
	name := fs.String("name", "", "cache name (required)")
	timeout := fs.Int("timeout", 10, "request timeout in seconds")
	fs.Parse(args)
	if fs.NArg() < 1 || *name == "" {
		die(fmt.Errorf("usage: ambient fetch --name NAME [--ttl 300] URL"))
	}
	res, err := fetchc.Fetch(fs.Arg(0), *name, time.Duration(*ttl)*time.Second, time.Duration(*timeout)*time.Second, "ambient/"+version)
	if err != nil && res.Body == nil {
		die(err)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "ambient: serving stale cache:", err)
	}
	os.Stdout.Write(res.Body)
	if res.Stale {
		os.Exit(3)
	}
}

func cmdChart(args []string) {
	if len(args) < 1 || (args[0] != "line" && args[0] != "volume") {
		die(fmt.Errorf("usage: ambient chart line|volume [--height N] [--width N] [--ascii] [--json] < values"))
	}
	fs := flag.NewFlagSet("chart", flag.ExitOnError)
	height := fs.Int("height", 11, "rows")
	width := fs.Int("width", 80, "columns")
	ascii := fs.Bool("ascii", false, "plain # rendering")
	js := fs.Bool("json", false, "emit metadata as json (line only)")
	fs.Parse(args[1:])

	var vals []float64
	sc := bufio.NewScanner(os.Stdin)
	sc.Buffer(make([]byte, 1024*1024), 1024*1024)
	for sc.Scan() {
		f := strings.Fields(sc.Text())
		if len(f) == 0 {
			continue
		}
		if v, err := strconv.ParseFloat(f[len(f)-1], 64); err == nil {
			vals = append(vals, v)
		}
	}
	if os.Getenv("AMBIENT_GLYPHS") == "ascii" {
		*ascii = true
	}
	if args[0] == "volume" {
		rows := chartc.VolGrid(vals, *height, *width)
		if *ascii {
			rows = chartc.Ascii(rows)
		}
		for _, r := range rows {
			fmt.Println(r)
		}
		return
	}
	line := chartc.LineGrid(vals, *height, *width)
	rows := line.Rows
	if *ascii {
		rows = chartc.Ascii(rows)
	}
	if *js {
		lo, hi := 0.0, 0.0
		if line.Lo != nil {
			lo, hi = *line.Lo, *line.Hi
		}
		b, _ := json.Marshal(struct {
			Rows    []string `json:"rows"`
			Lo      float64  `json:"lo"`
			Hi      float64  `json:"hi"`
			LastRow int      `json:"last_row"`
			Empty   bool     `json:"empty"`
		}{rows, lo, hi, line.LastRow, line.Empty})
		fmt.Println(string(b))
		return
	}
	for _, r := range rows {
		fmt.Println(r)
	}
}
