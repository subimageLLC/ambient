package doctor

import (
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"time"

	"ambient/internal/cfg"
	"ambient/internal/screen"
)

func check(exe string) string {
	if _, err := exec.LookPath(exe); err == nil {
		return "ok"
	}
	return "missing"
}

func probe(url string) string {
	client := &http.Client{Timeout: 5 * time.Second}
	req, _ := http.NewRequest("HEAD", url, nil)
	req.Header.Set("User-Agent", "ambient")
	resp, err := client.Do(req)
	if err != nil {
		return "unreachable"
	}
	resp.Body.Close()
	return fmt.Sprintf("HTTP %d", resp.StatusCode)
}

func Run(c cfg.Config, screens []screen.Screen) {
	fmt.Println("ambient doctor")
	fmt.Println()
	fmt.Printf("tmux        %s\n", check("tmux"))
	for _, s := range screens {
		for _, need := range s.Needs {
			if need == "network" {
				continue
			}
			fmt.Printf("%-11s %s  (for %s)\n", need, check(need), s.Name)
		}
	}
	t := os.Getenv("TERM")
	note := ""
	if t == "" || t == "dumb" {
		note = "  (no usable terminal type; ambient will substitute xterm-256color)"
	}
	fmt.Printf("TERM        %s%s\n", t, note)
	fmt.Println()
	fmt.Println("network (endpoints contacted by enabled screens):")
	seen := map[string]bool{}
	for _, s := range screens {
		for _, host := range s.Contacts {
			if seen[host] {
				continue
			}
			seen[host] = true
			fmt.Printf("  %-28s %s\n", host, probe("https://"+host))
		}
	}
	fmt.Println()
	fmt.Println("privacy — what each enabled screen reads and contacts:")
	for _, s := range screens {
		fmt.Printf("  %s (%s, %s)\n", s.Name, s.Mode, pri(s.Private))
		for _, r := range s.Reads {
			fmt.Printf("    reads    %s\n", r)
		}
		for _, h := range s.Contacts {
			fmt.Printf("    contacts %s\n", h)
		}
	}
	fmt.Println()
	fmt.Println("glyph test card — you should see blocks and dots, not boxes/question marks:")
	fmt.Println("  braille  ⣿ ⠿ ⠶ ⠦ ⠄ ⠂ ⠁")
	fmt.Println("  eighths  █ ▇ ▆ ▅ ▄ ▃ ▂ ▁")
	fmt.Println("  marks    ▲ ▼ · ╌ ─ ▒")
	fmt.Println()
	if c.Glyphs == "ascii" {
		fmt.Println("glyphs = \"ascii\" is set — charts will render as # marks.")
	} else {
		fmt.Println("if any row shows tofu, install a nerd font or set glyphs = \"ascii\".")
	}
}

func pri(p bool) string {
	if p {
		return "private"
	}
	return "builtin"
}
