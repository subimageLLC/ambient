# TODO

## packaging (next milestone: v0.2.0)
## done (2026-10-29, v0.2.0)
- [x] goreleaser release pipeline (darwin+linux, amd64+arm64, checksums)
- [x] homebrew tap: subimageLLC/homebrew-tap (`brew tap subimageLLC/tap && brew install ambient`)
- [x] tag v0.2.0, first release with binaries
- [x] repo public
- [ ] goreleaser nfpm: `.deb` with `Depends: tmux, toilet, jq, gping` and `.rpm` with `Requires` — so a dpkg/dnf install pulls screen deps
- [ ] homebrew tap (goreleaser generates the formula; brew pulls toilet/jq/gping/btop automatically)
- [ ] AUR package
- [ ] hosted apt repo (GitHub Pages feed) if there's demand — enables literal `apt install ambient`
- [ ] tag v0.2.0, cut the first release with binaries

## screens
- [ ] sys theme detection: KDE (kreadconfig kdeglobals ColorScheme) and hyprland/sway equivalents; OSC 11 terminal-background query for terminal-theme ≠ OS-theme cases (unreliable through tmux)
- [x] coins: axis labels fit the pi's 88x25 pane — verified live on the kiosk 2026-10-02
- [ ] coins: migrate to `ambient fetch` / `ambient chart` subcommands — one implementation of backoff + braille rendering instead of the vendored copy (deferred deliberately: too risky to rework the most-polished screen unattended)

## done (2026-10-02, the kiosk cutover)
- [x] pane splits per screen (`[[panes]]` in manifest) — the kiosk's layouts port
- [x] status chips (`ambient chip`/`status`, ticker render, stale dim, dead drop)
- [x] the kiosk parity screens: docker (read-only), events (replaced logs), sec, net tailnet pane
- [x] the kiosk cutover: Go binary at /usr/local/bin/ambient, private screens (lan, sec+unifi) in ~<service-user>/.config/ambient/screens/, shim with old builder kept at ~/bin/ambient.bash.bak, runbook on the kiosk
- [x] clock → now rename (now.conf with clock.conf fallback)
- [x] doctor: distro detection (suse + red hat families, correct apk/zypper verbs), journal-readability probe, non-zero exit when enabled screens' needs are missing
- [x] CI: `make check` = vet + gofmt + shellcheck + script syntax + go tests + headless smoke against the real screens, on ubuntu and macOS
