# TODO

## packaging
- [ ] goreleaser nfpm: `.deb` with `Depends: tmux, toilet, jq, gping` and `.rpm` with `Requires` — so a dpkg/dnf install pulls screen deps
- [ ] homebrew tap (goreleaser generates the formula; brew pulls toilet/jq/gping/btop automatically)
- [ ] AUR package
- [ ] hosted apt repo (GitHub Pages feed) if there's demand — enables literal `apt install ambient`
- [ ] tag v0.2.0, cut the first release with binaries

## screens
- [x] core: pane splits per screen (`[[panes]]` in manifest) — done, ported the kiosk's layouts
- [x] core: status chips (ambient chip/status + ticker render) — done
- [x] the kiosk migration screens: docker (read-only), logs (logsum+lnav), sec (generic core + optional unifi), net tailnet pane — done
- [x] the kiosk cutover — done 2026-10-02: Go binary at /usr/local/bin/ambient, private screens (lan, sec+unifi) in ~<service-user>/.config/ambient/screens/, launcher shim with old builder kept at ~/bin/ambient.bash.bak, runbook at the kiosk:the cutover runbook
- [ ] sys theme detection: KDE (kreadconfig kdeglobals ColorScheme) and hyprland/sway equivalents; OSC 11 terminal-background query for terminal-theme ≠ OS-theme cases (unreliable through tmux)
- [ ] coins: check axis labels fit the pi's 88x25 pane (may need fewer ticks at small widths)
- [ ] coins: migrate to `ambient fetch` / `ambient chart` subcommands — one implementation of backoff + braille rendering instead of the vendored copy

## core
- [ ] doctor: non-zero exit when deps are missing (CI-friendly)
- [ ] test attach from two terminals simultaneously (mirror mode)
- [ ] systemd user timer alternative to the internal ticker, documented
