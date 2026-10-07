package doctor

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"
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

// pkgManagerFor maps /etc/os-release content to a package manager. The
// specific IDs matter: RHEL/CentOS/Rocky/Alma carry ID_LIKE=fedora today,
// but matching the IDs keeps working if that ever changes; suse is checked
// before the fedora family to avoid any ID_LIKE ambiguity.
func pkgManagerFor(osRelease string) string {
	id := strings.ToLower(osRelease)
	switch {
	case strings.Contains(id, "debian") || strings.Contains(id, "ubuntu") || strings.Contains(id, "raspbian"):
		return "apt"
	case strings.Contains(id, "suse") || strings.Contains(id, "sles"):
		return "zypper"
	case strings.Contains(id, "fedora") || strings.Contains(id, "rhel") ||
		strings.Contains(id, "centos") || strings.Contains(id, "rocky") ||
		strings.Contains(id, "almalinux") || strings.Contains(id, "oracle"):
		return "dnf"
	case strings.Contains(id, "arch"): // arch, manjaro, endeavouros (ID_LIKE=arch)
		return "pacman"
	case strings.Contains(id, "alpine"):
		return "apk"
	}
	return ""
}

func pkgManager() string {
	if _, err := exec.LookPath("brew"); err == nil {
		return "brew"
	}
	b, err := os.ReadFile("/etc/os-release")
	if err != nil {
		return ""
	}
	return pkgManagerFor(string(b))
}

// install verbs per package manager — not all use "install"
var installVerbs = map[string]string{
	"pacman": "-S",
	"apk":    "add",
}

func installHint(dep string) string {
	pm := pkgManager()
	if pm == "" {
		return ""
	}
	verb := installVerbs[pm]
	if verb == "" {
		verb = "install"
	}
	return fmt.Sprintf("→ %s %s %s", pm, verb, dep)
}

// journalReadable reports whether this user can read the SYSTEM journal —
// on arch (and stock RHEL) a default user cannot, which silently starves
// the events screen: journalctl exists but returns nothing.
func journalReadable() bool {
	out, err := exec.Command("journalctl", "--system", "-n", "1", "--no-pager", "-o", "cat").Output()
	return err == nil && len(strings.TrimSpace(string(out))) > 0
}

func probe(url string) string {
	client := &http.Client{Timeout: 5 * time.Second}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "unreachable" // malformed URL in a manifest — never panic doctor
	}
	req.Header.Set("User-Agent", "ambient")
	resp, err := client.Do(req)
	if err != nil {
		return "unreachable"
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, io.LimitReader(resp.Body, 4<<10))
	if resp.StatusCode >= 400 {
		return fmt.Sprintf("reachable (HTTP %d)", resp.StatusCode)
	}
	return "reachable"
}

// Run prints the environment report and returns the number of unsatisfied
// needs across enabled screens — non-zero is CI-meaningful (missing deps),
// while advisory notes (journal readability) never count.
func Run(c cfg.Config, screens []screen.Screen) int {
	missing := 0
	fmt.Println("ambient doctor")
	fmt.Println()
	fmt.Printf("tmux        %s\n", check("tmux"))
	if check("tmux") == "missing" {
		missing++
	}
	for _, s := range screens {
		for _, need := range s.Needs {
			if need == "network" {
				continue
			}
			// "a|b" — any one of the alternatives satisfies the need
			// (e.g. "journalctl|log": linux journal vs macOS unified log)
			alts := strings.Split(need, "|")
			status := "missing"
			for _, a := range alts {
				if check(a) == "ok" {
					status = "ok"
					break
				}
			}
			if status == "missing" {
				missing++
			}
			line := fmt.Sprintf("%-11s %-7s (for %s)", need, status, s.Name)
			if status == "missing" {
				var hints []string
				for _, a := range alts {
					if h := installHint(a); h != "" {
						hints = append(hints, h)
					}
				}
				if len(hints) > 0 {
					line += "  " + strings.Join(hints, " / ")
				}
			}
			fmt.Println(line)
		}
	}
	// the events screen's data source — existence is not readability
	if check("journalctl") == "ok" {
		if journalReadable() {
			fmt.Printf("%-11s %s\n", "journal", "readable")
		} else {
			fmt.Printf("%-11s %s\n", "journal", "empty for this user — add yourself to the systemd-journal (or adm) group, or events shows nothing")
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
	return missing
}

func pri(p bool) string {
	if p {
		return "private"
	}
	return "builtin"
}
