# ambient — design

One-liner: a rotating terminal dashboard that runs anywhere — ssh session,
corner terminal, or dedicated kiosk — where every screen is a plain script you
can read and edit.

Principles: no daemon, no browser, no cloud, no telemetry. PII-free core.
Screens are first-class plugins.

## Why

Incumbents (MagicMirror², Homepage, Glance, e-ink projects) are all
browser/graphical. Terminal-native is the open niche: tmux + TUI screens means
the same code runs over ssh, on a Pi kiosk, or in a corner terminal.

~80% of this design is proven in production on a hand-built kiosk ("the kiosk"):
the tmux session builder, rotation timer, pause flag, per-screen caching with
rate-limit backoff, and ANSI theme passthrough all run there daily.

## Language decision

Go core (single static binary, trivial cross-compilation including arm64 Pi,
`go install` for free) + screens as plain bash/python scripts. The core owns
the flaky parts once and properly: pty handling, rotation scheduling, config,
cached fetching, terminal/glyph detection. Screens keep the edit-live
property: drop a file in the screens dir, it applies on next rebuild.

The binary also exposes the hard parts to script screens as subcommands:

    ambient fetch --ttl 300 --name foo "https://..."    # cached, 429 backoff
    ambient chart line --height 11 --width 80 < values  # braille rows

## Architecture

    ambient (Go binary)           repo screens/ (embedded)     user space
    ├── start/attach/rebuild      screens/<name>/              ~/.config/ambient/
    ├── rotate/pause (ticker)       manifest.toml               ├── ambient.toml
    ├── fetch (cache+backoff)       entrypoint script           ├── screens/   (private)
    ├── chart (braille/blocks)    targets/kitty/ (kiosk docs)   └── themes/
    └── doctor (env/glyph/privacy)

- Embedded screens are extracted to `~/.local/share/ambient/screens/` at
  start (embedded FS is the source of truth; extraction always syncs).
  Private screens in `~/.config/ambient/screens/<name>/` override embedded
  screens by name. That is how personal-infrastructure screens stay
  unpublished: they live only on the owner's machine.
- Rotation: `ambient ticker` — a child process of `ambient start` that sleeps
  `rotation_secs`, skips while the pause flag is set, exits when the session
  dies. Wire `ambient rotate` to systemd/cron/launchd instead if preferred.
- The tmux layer is a faithful port of the kiosk's builder: one window per
  screen, remain-on-exit on, `detach-on-destroy off`, n/N/p binds,
  solarized status bar. Builds run under a real pty (no `script` hack, no
  TERM=dumb fragility).
- Passive screens are wrapped by `ambient run <dir>`: clear, exec entrypoint,
  sleep refresh; repeated fast failures back off instead of hammering.
- Interactive screens own their loop (curses etc.).

## Per-screen configuration (screens own their settings)

Users never edit config files by hand:

- **coins** — first run opens an in-TUI portfolio picker: popular coins grouped
  by network (majors, ethereum, base, solana, hyperliquid, privacy), type to
  filter, space to pick; typing 2+ characters also searches coingecko's full
  catalog (cached), so any listed coin can be added. `a` reopens the picker
  anytime, `x` removes the selected coin. Saves to
  `~/.config/ambient/screens/coins.conf`.
- **now (time+weather)** — `ambient setup now` asks for a zipcode, city, or
  lat,lon, validates it against wttr.in, and saves it. The query string is the
  only thing about the user that leaves the machine.
- **net** — works with defaults; `ambient setup net` changes the WAN target.

Screens read their own `<name>.conf` (KEY=VALUE lines) under
`~/.config/ambient/screens/` — never the global ambient.toml, which stays
core-only (rotation, screen list, glyph mode). A manifest may declare a
`setup` entrypoint, which `ambient setup <name>` runs interactively.

## Fetch subcommand semantics

- Cache: `~/.cache/ambient/fetch/<name>.json` + `<name>.state.json`
  (`retry_at` timestamp — backoff state must survive across invocations
  because screens call fetch as a subprocess).
- Fresh within TTL → stdout, exit 0. Backing off → stale body to stdout if
  present, exit 3; nothing cached → exit 1. HTTP 429 → retry_at = now +
  max(30s, min(1h, Retry-After)). Other errors → now + 10s.

## Chart subcommand

Faithful port of the kiosk's proven renderers: braille line grid
(2x4 dots/cell, gridline-friendly) and eighths-block volume bars.
`--json` emits rows + lo/hi/last_row for labeling. `glyphs = "ascii"` in
config degrades braille/blocks to `#`.

## Screen contract

A screen is a directory with a `manifest.toml` and an entrypoint. Screens may
declare extra panes (`[[panes]]` with `entry`, `split` v/h, `percent`); each
pane runs a script that owns its own loop. Passive screens are wrapped by
`ambient run` (clear, exec, sleep refresh, backoff on fast failures);
interactive screens and all panes run raw.

## Events, not logs (the events screen)

A raw log stream on an ambient display is noise theater — unreadable from
across the room, unactionable up close. The events screen turns the journal
into a few plain-language statements — what happened, when, and whether it
is still broken (✓ fixed / × down / ⚠ live). Small local pattern rules do
the work; no agent, no cloud:

- systemd unit failures and crash loops (restart counters), with live state
  from `systemctl is-active` deciding ✓/×
- OOM kills, disk I/O errors (per device), ssh brute-force (top attacker)
- everything else collapses into a one-line "hum" footer, grouped by
  source, muteable (acknowledged noise stays visible but stops paging)

The status chip names the worst *unresolved* event (`⚠relay-1 down`), so
the bar says what to do, not how loud the machine is. macOS has no systemd
or kernel journal — the screen degrades honestly to grouped hum.

## Status chips

The status bar is the ambient health surface, visible on every screen.
Writers (screen scripts, private integrations — anything) call:

    ambient chip <writer> <color> <prio> <text> [...]   # or: clear

which stores `~/.cache/ambient/status/<writer>.json`. The ticker renders all
writers into `status-right` on every rotation, and the builder renders once
at build time — chips never vanish on a rebuild. Ordering: prio ascending,
then writer name; a writer older than 15 minutes collapses to a dim `name?`
chip so a dead writer is visible, not silent. Silence means healthy: writers
clear their file when there is nothing to report.

## TUI conventions

Screens are plain scripts, so the shared look lives in convention rather
than a framework. Anything user-facing follows these rules:

- **Screens adapt to their pane.** The same screen runs fullscreen on an
  800x480 kiosk, in a corner terminal, and inside a split. Each screen reads
  its own geometry (`tput cols`/`lines`) and reflows: narrow panes drop
  columns rather than truncating rows, short panes drop low-priority
  sections, and lines are cut to width — never wrapped (a wrapped line
  scrolls the whole pane and crawls). clock centers by size, coins grows a
  market-cap column past 64 cols; every screen follows the same rule.

- **Spinners lead their text.** `▃ searching coingecko…`, never
  `loading chart… ▇`. One spinner glyph set everywhere (the bar wave
  `▁▃▅▇█▇▅▃`, `SPINNER` in coins.py); frames advance on the input tick,
  which drops to ~140ms while work is in flight.
- **Busy work never freezes the UI.** Network calls run in a background
  thread and the loop keeps reading keys; the spinner is the honest
  state, not a decoration.
- **Color semantics:** cyan = selection/brand, green/red = up/down,
  dim gray (244) = meta, hints, and empty states. Notes state facts
  (`no coingecko matches for "xyz"`) without blinking or drawing
  attention.
- **Ambiguous results identify themselves.** Search rows show the coin's
  chain and a shortened contract (`eth  0x7d9f…b1c2`), from a
  daily-cached platform index (`/coins/list?include_platform=true`,
  compacted to id → {chain: address} on disk). Until the index lands —
  or for native coins with no contract — the market-cap rank (`#373`)
  stands in. Chain priority: base, eth, arb, op, polygon, bnb, avax,
  sol.
- **Density scales with width.** The coins ticker grows a market-cap
  column past 64 columns. Narrow panes drop columns rather than
  truncating rows.
- **Addresses are pasted, not typed** — filters preserve case so
  base58 and checksummed hex survive; contract lookups probe chains in
  a fixed order (base, ethereum, then the other majors) and stop at the
  first hit.

## Privacy charter

- Zero telemetry, zero accounts. All caches under `~/.cache/ambient/` with a
  documented inventory of what each screen stores.
- Weather: user-provided coordinates only, never IP-geolocated. Coins: the
  user's own coin list. No defaults that phone home with identifying data.
- Manifests declare `reads` and `contacts`; `ambient doctor --privacy` prints
  exactly what each enabled screen reads and where it sends.
- Example configs use fake data.

## Run modes

1. `ambient attach` — mirror into any terminal
2. `ambient start` in a local terminal — instant demo
3. Kiosk target (planned, not yet shipped): `targets/` will carry recipes
   (Linux: getty autologin → xinit → kitty fullscreen; macOS: login item +
   kitty fullscreen) with the solarized kitty.conf. Until then, the kiosk's
   AMBIENT-README.md documents a working boot flow to copy from.

## Packaging & CI

- goreleaser: linux/amd64+arm64, macOS universal; Homebrew tap; nix/AUR as
  PRs arrive. MIT.
- CI: build + test + vet on ubuntu/macOS, smoke test that starts the session
  headless in tmux and captures panes; golden tests for chart/fetch.

## Roadmap

1. Core start/attach/rotate/pause/rebuild + ticker
2. fetch + chart subcommands
3. Screen loader (embedded + private merge)
4. Core screens: coins, now (time+weather), net, sys
5. doctor
6. Kiosk recipes, goreleaser, tap
7. Dogfood on the kiosk as its replacement

Out of scope for now: community contrib repo, web rendering, Windows.
