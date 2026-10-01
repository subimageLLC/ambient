# TODO

## packaging
- [ ] goreleaser nfpm: `.deb` with `Depends: tmux, toilet, jq, gping` and `.rpm` with `Requires` — so a dpkg/dnf install pulls screen deps
- [ ] homebrew tap (goreleaser generates the formula; brew pulls toilet/jq/gping/btop automatically)
- [ ] AUR package
- [ ] hosted apt repo (GitHub Pages feed) if there's demand — enables literal `apt install ambient`
- [ ] tag v0.2.0, cut the first release with binaries

## screens
- [ ] coins: check axis labels fit the pi's 88x25 pane (may need fewer ticks at small widths)
- [ ] coins: migrate to `ambient fetch` / `ambient chart` subcommands — one implementation of backoff + braille rendering instead of the vendored copy
- [ ] the kiosk migration: private screens (sec rail w/ unifi, tailnet, lan census, docker, logs) into the kiosk's `~/.config/ambient/screens/`
- [ ] core: pane splits per screen (the kiosk's clock is 75% clock + 25% sec rail) — needed before the kiosk swap

## core
- [ ] doctor: non-zero exit when deps are missing (CI-friendly)
- [ ] test attach from two terminals simultaneously (mirror mode)
- [ ] systemd user timer alternative to the internal ticker, documented
