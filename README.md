# ambient

An ambient display for any unix: a rotating dashboard of terminal screens —
time & weather, network, crypto, system, docker, events, security — that runs
over ssh, in a corner terminal, or fullscreen on a kiosk.

- **No daemon** — a tmux session you can attach to from anywhere
- **No browser, no cloud, no telemetry**
- **Every screen is a plain script** you can read and edit
- **PII-free core** — nothing phones home with identifying data; add your own
  private screens in `~/.config/ambient/screens/` and they never leave your machine


## Screens

Pane captures from live sessions, real data, one monospace grid — dark and
light. Light mode is the same ANSI bytes through a Solarized Light palette
(what a light terminal profile does); `sys` follows the desktop appearance
via btop's own theme. The status bar (badge, window tabs, chips) is
reconstructed from the session's format strings.

All eight screens in one pass — slideshow at ~1.4s each (the real rotation
is 60s by default):

![ambient's screens rotating](assets/demo.gif)

Dark:

<p>
  <img src="assets/screens/now.png" alt="now — time and weather" width="49.5%">
  <img src="assets/screens/coins.png" alt="coins — crypto charts" width="49.5%">
  <img src="assets/screens/sys.png" alt="sys — system stats" width="49.5%">
  <img src="assets/screens/net.png" alt="net — latency graph" width="49.5%">
  <img src="assets/screens/docker.png" alt="docker — containers" width="49.5%">
  <img src="assets/screens/sec.png" alt="sec — security posture" width="49.5%">
  <img src="assets/screens/events.png" alt="events — journal as events" width="49.5%">
  <img src="assets/screens/tailnet.png" alt="tailnet — tailnet peers" width="49.5%">
</p>

Light:

<p>
  <img src="assets/screens/light/now.png" alt="now — light" width="49.5%">
  <img src="assets/screens/light/coins.png" alt="coins — light" width="49.5%">
  <img src="assets/screens/light/sys.png" alt="sys — light" width="49.5%">
  <img src="assets/screens/light/net.png" alt="net — light" width="49.5%">
  <img src="assets/screens/light/docker.png" alt="docker — light" width="49.5%">
  <img src="assets/screens/light/sec.png" alt="sec — light" width="49.5%">
  <img src="assets/screens/light/events.png" alt="events — light" width="49.5%">
  <img src="assets/screens/light/tailnet.png" alt="tailnet — light" width="49.5%">
</p>

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
    screens = ["now", "net", "coins", "sys"]

**Screens configure themselves — no config file editing required:**

Blank-slate friendly: with no config at all, ambient rotates through every
screen in a curated order (glance screen first) and tells you how to opt
into the rest; every wait shows its state (fetching, rate-limited with a
retry countdown, stale) — never a blank or frozen frame.

- `coins` — on first run, a picker opens right on the screen: browse popular
  coins grouped by network (majors, ethereum, base, solana, hyperliquid,
  privacy), type to filter, space to pick. Typing two or more characters also
  searches all of coingecko, so any listed coin can be added. `a` reopens it
  anytime, `x` removes the selected coin.
- `now` — the glance screen: big block-digit clock, date, and weather. Press
  `s` right on the screen to set a zipcode, city, or lat,lon (that query goes
  to wttr.in; nothing else about you does). `ambient setup now` works from
  the terminal too.
- `net` — live gping-style latency charts for your gateway and the internet.
  Press `s` to change the WAN target. Works out of the box.
- `ambient setup` with no arguments runs every screen's setup wizard.

Run `ambient screens` to see which screens have a setup command.

Keys and clicks — the same on every screen, shown in the status bar:

- `Ctrl-b ?` — key help for the current screen (per-screen keys come from its manifest)
- `Ctrl-b *` — settings for the current screen (runs its setup in a popup; screens without one list their on-screen keys instead)
- `Ctrl-b n` / `N` — next / previous screen
- `Ctrl-b p` — pause/unpause rotation (a yellow `⏸ paused` chip shows in the status bar while paused)
- `Ctrl-b d` — detach
- **Mouse**: click `⌨ ?` or `* setup` in the status bar for the same popups; click the `ambient` badge to pause/unpause (works on a keyboardless kiosk); click a window name to switch screens

## Requirements

- **Core**: tmux 3.2+ — that's it. The binary is static and dependency-free
- **Screens declare their own deps** in their manifest (`needs = ["toilet", "jq", ...]`).
  `ambient doctor` checks every one and prints the install command for your
  package manager (brew / apt / dnf / zypper / pacman / apk). A screen whose
  deps are missing shows an in-pane install hint and recovers automatically
  once you install them — no session rebuild needed
- python3 + a nerd font for the coins screen (`ambient doctor` prints a glyph
  test card; set `glyphs = "ascii"` to degrade gracefully)

### Platform support

Same code everywhere; screens degrade per-host rather than failing:

| tier | platforms | story |
|---|---|---|
| tested | macOS, Debian / Ubuntu / Raspberry Pi OS | the kiosk is a Pi; day-to-day dev on macOS |
| same family | Fedora, RHEL / CentOS / Rocky / Alma, Arch, openSUSE | systemd + GNU userland — everything should just work. On Arch (and stock RHEL), a default user cannot read the *system* journal; `ambient doctor` checks and tells you to add yourself to `systemd-journal`/`adm`, or the events screen shows nothing |
| partial | non-systemd linux (Alpine, Void, Artix, Devuan, Gentoo/openrc) | core + now/net/sys/coins/docker/sec work; events needs a systemd journal and shows n/a — but it degrades in place |
| untested | FreeBSD / OpenBSD / NetBSD | the Go binary and tmux exist everywhere and the scripts carry BSD fallbacks (`stat -f`, `route get`, `lsof`), but there are no release binaries yet — build from source and expect rough edges |

### Why toilet?

The now screen's big digits come from `toilet -f mono12` — the same renderer the
original kiosk uses, so the look is identical everywhere. toilet, jq, gping,
and btop are packaged in Debian, Ubuntu, Fedora, Arch, openSUSE, the BSDs
(ports), and Homebrew.

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

    [[panes]]                    # optional extra panes; the kiosk's layout ported
    entry   = "tailnet.sh"       # each pane owns its own loop
    split   = "v"                # "v" stacked below, "h" side by side
    percent = 40                 # this pane's share, 5-90

## Status chips

Anything can raise an alert chip on the status bar — visible on every screen;
silence means healthy:

    ambient chip events red 2 "⚠ssh 203.0.113.x"  # color prio text (repeat triples for more)
    ambient chip events clear                   # healthy again — file removed

Chips live in `~/.cache/ambient/status/<writer>.json`; the rotation ticker
renders them into the bar (sorted by prio, stale writers collapse to a dim
`name?`). `ambient status` renders once for testing.

## Screen selection

Core screens: `now` `net` `coins` `sys` `docker` `events` `sec`. The first
four work out of the box; `events` needs a systemd journal host (macOS
degrades to error-hum only), `sec` degrades to "not configured" without a
private threat feed, `docker` needs docker CLI access (read-only by default —
scoped sudoers or the docker group; running lazydocker with stop/restart
powers is an opt-in you wire up yourself).

`events` is what the old `logs` screen became: a raw log stream on an
ambient display was noise theater, so the screen mines the journal with
small local rules (no agent, no cloud) into plain-language events — *what
happened, when, and whether it is still broken*:

    EVENTS   1 open · 1 fixed · 24h window
     16:04  ⚠ live   203.0.113.45 — ssh: 87 failed logins (1h)
     15:37  ✓ fix    backup-timer — failed (exec), recovered after 37m
     hum    kernel:vc4 ×6.4k · muted sources stay counted

Everything else collapses into the one-line hum footer, grouped by source.
Acknowledged noise goes quiet via `~/.config/ambient/screens/events-mute`
(one source per line): muted sources stay counted in the hum but stop
paging the status bar — and the chip names the worst *unresolved* event
(`⚠relay-1 down`), not a volume (`⚠7183err`).

Every screen is pane-size aware: narrow panes drop columns, short panes
drop low-priority sections, and lines are cut to width rather than wrapped
— the same screen works fullscreen on a kiosk and in a corner split.

Drop one in `~/.config/ambient/screens/<name>/` to add or override — private
screens are never part of the published repo.

## Status

Pre-alpha. Dogfooding on a live kiosk. See DESIGN.md.
