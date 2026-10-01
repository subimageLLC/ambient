# ambient

An ambient display for any unix: a rotating dashboard of terminal screens —
clock, weather, network, crypto, system — that runs over ssh, in a corner
terminal, or fullscreen on a kiosk.

- **No daemon** — a tmux session you can attach to from anywhere
- **No browser, no cloud, no telemetry**
- **Every screen is a plain script** you can read and edit
- **PII-free core** — nothing phones home with identifying data; add your own
  private screens in `~/.config/ambient/screens/` and they never leave your machine

## Install

    go install ./...          # from a checkout
    # or: brew install ambient (tap coming)

## Quick start

    ambient start             # build the session and attach (Ctrl-b d to detach)
    ambient attach            # reattach from any terminal, any machine
    ambient pause             # toggle rotation pause (also Ctrl-b p)
    ambient doctor            # check terminal, fonts, deps, and privacy

Screens rotate every 60s by default. Core options live in `~/.config/ambient/ambient.toml`:

    rotation_secs = 90
    screens = ["clock", "net", "coins", "sys"]

**Screens configure themselves — no config file editing required:**

- `coins` — on first run, a picker opens right on the screen: browse popular
  coins grouped by network (majors, ethereum, base, solana, hyperliquid,
  privacy), type to filter, space to pick. `a` reopens it anytime, `x` removes
  the selected coin.
- `clock` — big block-digit clock and weather. Press `s` right on the screen
  to set a zipcode, city, or lat,lon (that query goes to wttr.in; nothing
  else about you does). `ambient setup clock` works from the terminal too.
- `net` — live gping-style latency charts for your gateway and the internet.
  Press `s` to change the WAN target. Works out of the box.
- `ambient setup` with no arguments runs every screen's setup wizard.

Run `ambient screens` to see which screens have a setup command.

Keys (inside the session): `Ctrl-b n/N` next/previous screen, `Ctrl-b p`
pause/unpause rotation, `Ctrl-b d` detach.

## Requirements

- **Core**: tmux 3.2+ — that's it. The binary is static and dependency-free
- **Screens declare their own deps** in their manifest (`needs = ["toilet", "jq", ...]`).
  `ambient doctor` checks every one and prints the install command for your
  package manager (brew / apt / dnf / pacman / apk). A screen whose deps are
  missing shows an in-pane install hint and recovers automatically once you
  install them — no session rebuild needed
- python3 + a nerd font for the coins screen (`ambient doctor` prints a glyph
  test card; set `glyphs = "ascii"` to degrade gracefully)

### Why toilet?

The clock's big digits come from `toilet -f mono12` — the same renderer the
original kiosk uses, so the look is identical everywhere. toilet, jq, gping,
and btop are packaged in Debian, Ubuntu, Fedora, Arch, and Homebrew.

## Screen contract

A screen is a directory with a `manifest.toml` and an entrypoint:

    name     = "coins"
    mode     = "interactive"     # owns its loop (TUI), or "passive" — core refreshes it
    refresh  = 60                # passive refresh interval, seconds
    entry    = "coins.py"
    setup    = "setup.sh"        # optional: makes `ambient setup <name>` work
    needs    = ["python3", "network"]
    glyphs   = "braille"
    reads    = ["~/.config/ambient/screens/coins.conf"]
    contacts = ["api.coingecko.com"]

Drop one in `~/.config/ambient/screens/<name>/` to add or override — private
screens are never part of the published repo.

## Status

Pre-alpha. Dogfooding on a live kiosk. See DESIGN.md.
