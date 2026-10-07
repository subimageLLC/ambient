package main

import (
	"bufio"
	"embed"
	"encoding/json"
	"flag"
	"fmt"
	"math"
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
	"ambient/internal/status"
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
		now, err := session.TogglePause()
		if err != nil {
			die(err)
		}
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
		// a zero/negative interval would hot-loop Rotate; the config
		// validator enforces >=5, the raw CLI path must too
		if secs < 5 {
			secs = 5
		}
		tick := func() error {
			err := session.Rotate(false)
			if rerr := session.RenderStatus(); rerr != nil {
				fmt.Fprintln(os.Stderr, "status:", rerr)
			}
			return err
		}
		if err := run.Ticker(secs, tick, session.Has); err != nil {
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
		c, err := cfg.Load()
		if err != nil {
			die(fmt.Errorf("config: %w", err))
		}
		screens := loadScreens(c)
		// non-zero when enabled screens have unsatisfied needs (CI-friendly)
		os.Exit(doctor.Run(c, screens))
	case "chip":
		cmdChip(os.Args[2:])
	case "status":
		if err := session.RenderStatus(); err != nil {
			die(err)
		}
		fmt.Println(status.Render(status.Read(), time.Now()))
	case "screens":
		c, err := cfg.Load()
		if err != nil {
			die(fmt.Errorf("config: %w", err))
		}
		for _, s := range loadScreens(c) {
			setup := ""
			if s.Setup != "" {
				setup = "  (ambient setup " + s.Name + ")"
			}
			fmt.Printf("%-12s %-12s refresh=%ds%s\n", s.Name, s.Mode, s.Refresh, setup)
		}
	case "help":
		cmdHelp(len(os.Args) > 2 && os.Args[2] == "--esc")
	case "setup":
		cmdSetup(os.Args[2:])
	case "setup-here":
		cmdSetup([]string{currentWindow()})
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
  ambient chip <n> clear|<c> <p> <t>  set status-bar chips for a screen
  ambient status                      render status-bar chips now
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

func currentWindow() string {
	cmd := exec.Command("tmux", "display-message", "-p", "#W")
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func cmdHelp(waitEsc bool) {
	screens := loadScreens(cfg.Default())
	name := currentWindow()
	fmt.Println()
	fmt.Printf("  ambient keys — %s\n", func() string {
		if name == "" {
			return "all screens"
		}
		return name
	}())
	if name != "" {
		for _, s := range screens {
			if s.Name == name {
				fmt.Printf("  ── %s ─────────────────────\n", s.Name)
				for _, k := range s.Keys {
					fmt.Printf("    %s\n", k)
				}
				break
			}
		}
	} else {
		for _, s := range screens {
			if len(s.Keys) == 0 {
				continue
			}
			fmt.Printf("  ── %s ─────────────────────\n", s.Name)
			for _, k := range s.Keys {
				fmt.Printf("    %s\n", k)
			}
		}
	}
	fmt.Println("  ── global ───────────────────")
	fmt.Println("    C-b n / N    next / previous screen")
	fmt.Println("    C-b p        pause / resume rotation")
	fmt.Println("    C-b ?        this help")
	fmt.Println("    C-b *        settings for this screen")
	fmt.Println("    C-b d        detach")
	fmt.Println("    click ? keys — this help · right-click — settings")
	fmt.Println("    click a window name — switch screen")
	if waitEsc && stdinIsTty() {
		fmt.Println()
		fmt.Println("  esc to close")
		waitEscape()
	}
	fmt.Println()
}

func stdinIsTty() bool {
	fi, err := os.Stdin.Stat()
	return err == nil && fi.Mode()&os.ModeCharDevice != 0
}

func waitEscape() {
	save := exec.Command("stty", "-g")
	save.Stdin = os.Stdin
	saved, _ := save.Output()
	raw := exec.Command("stty", "raw", "-echo")
	raw.Stdin = os.Stdin
	raw.Run()
	defer func() {
		restore := exec.Command("stty", strings.TrimSpace(string(saved)))
		restore.Stdin = os.Stdin
		restore.Run()
	}()
	buf := make([]byte, 256)
	for {
		n, err := os.Stdin.Read(buf)
		if err != nil || n == 0 {
			return
		}
		if buf[0] == 3 || (buf[0] == 0x1b && n == 1) {
			return
		}
	}
}

func cmdSetup(args []string) {
	c, err := cfg.Load()
	if err != nil {
		die(fmt.Errorf("config: %w", err))
	}
	screens := loadScreens(c)
	var targets []screen.Screen
	if len(args) > 0 {
		for _, s := range screens {
			if s.Name == args[0] {
				targets = append(targets, s)
				break
			}
		}
		if len(targets) == 0 {
			die(fmt.Errorf("unknown screen %q", args[0]))
		}
	} else {
		for _, s := range screens {
			if s.Setup != "" {
				targets = append(targets, s)
			}
		}
		if len(targets) == 0 {
			fmt.Fprintln(os.Stderr, "no screens have a setup command")
			os.Exit(2)
		}
	}
	for _, target := range targets {
		if target.Setup == "" {
			fmt.Printf("\n  %s — settings live on-screen:\n", target.Name)
			if len(target.Keys) == 0 {
				fmt.Println("    no settings")
			}
			for _, k := range target.Keys {
				fmt.Printf("    %s\n", k)
			}
			if os.Getenv("AMBIENT_POPUP") == "1" {
				fmt.Println()
				fmt.Println("  esc to close")
				waitEscape()
			}
			continue
		}
		if err := runSetup(target); err != nil {
			die(err)
		}
	}
}

// cmdChip is the writer side of the status-bar chip convention, built for
// screen scripts:
//
//	ambient chip <name> clear                          — healthy, go silent
//	ambient chip <name> <color> <prio> <text> [...]    — triples
func cmdChip(args []string) {
	if len(args) < 2 {
		die(fmt.Errorf("usage: ambient chip <name> clear | <color> <prio> <text> [...]"))
	}
	name, rest := args[0], args[1:]
	if len(rest) == 1 && rest[0] == "clear" {
		if err := status.Write(name, nil); err != nil {
			die(err)
		}
		return
	}
	if len(rest) < 3 || len(rest)%3 != 0 {
		die(fmt.Errorf("chips come as <color> <prio> <text> triples"))
	}
	var chips []status.Chip
	for i := 0; i < len(rest); i += 3 {
		prio, err := strconv.Atoi(rest[i+1])
		if err != nil {
			die(fmt.Errorf("prio must be a number: %q", rest[i+1]))
		}
		chips = append(chips, status.Chip{Color: rest[i], Prio: prio, Text: rest[i+2]})
	}
	if err := status.Write(name, chips); err != nil {
		die(err)
	}
}

func unconfigured(screens []screen.Screen) []string {
	var out []string
	for _, s := range screens {
		if s.Setup == "" || s.SetupOptional || len(s.Reads) == 0 {
			continue
		}
		p := filepath.Join(os.Getenv("HOME"), strings.TrimPrefix(s.Reads[0], "~"))
		if strings.Contains(p, "*") {
			continue
		}
		if _, err := os.Stat(p); err != nil {
			out = append(out, s.Name)
		}
	}
	return out
}

func runSetup(target screen.Screen) error {
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
	return cmd.Run()
}

func cmdStart(rebuild bool) {
	fs := flag.NewFlagSet("start", flag.ExitOnError)
	detach := fs.Bool("d", false, "build without attaching")
	noRotate := fs.Bool("no-rotate", false, "do not spawn the rotation ticker")
	fs.Parse(os.Args[2:])

	c, err := cfg.Load()
	if err != nil {
		die(fmt.Errorf("config: %w", err))
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
		if len(c.Screens) == 0 {
			fmt.Println("ambient: no screens list set — rotating through all of them;")
			fmt.Println("          pick your rotation in ~/.config/ambient/ambient.toml (screens = [...])")
		}
		if un := unconfigured(screens); len(un) > 0 {
			fmt.Printf("ambient: not configured yet: %s — run `ambient setup %s` (or press s on the screen)\n",
				strings.Join(un, ", "), un[0])
		}
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
	fs.Parse(args)
	// non-positive dimensions would panic in make; clamp instead
	if *height < 1 {
		*height = 1
	}
	if *width < 1 {
		*width = 1
	}

	var vals []float64
	sc := bufio.NewScanner(os.Stdin)
	sc.Buffer(make([]byte, 1024*1024), 1024*1024)
	for sc.Scan() {
		f := strings.Fields(sc.Text())
		if len(f) == 0 {
			continue
		}
		if v, err := strconv.ParseFloat(f[len(f)-1], 64); err == nil {
			if math.IsNaN(v) || math.IsInf(v, 0) {
				continue // a NaN token parses fine and renders garbage braille
			}
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
