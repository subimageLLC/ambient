#!/bin/sh
# smoke — headless session build against the real screens.
#
# Catches the bug classes unit tests can't see: dash-vs-bash breakage,
# empty/dead panes, wrapped-line crawls, missing panes, chips that never
# render. Assertions are tolerant of missing optional deps (CI runners
# have no toilet/gping/tailscale/docker) — screens must degrade to a
# readable in-pane message, never to silence or a shell error.
#
# Runs entirely in a scratch env with AMBIENT_SESSION so a live session
# is never touched.
set -u

FAIL=0
note() { printf '  %s\n' "$1"; }
ok()   { printf '  ok    %s\n' "$1"; }
bad()  { printf '  FAIL  %s\n' "$1"; FAIL=$((FAIL + 1)); }

ROOT=$(cd "$(dirname "$0")/.." && pwd)
BIN=$ROOT/bin/ambient
export AMBIENT_SESSION=ambient-smoke
export AMBIENT_CONFIG_DIR AMBIENT_CACHE_DIR AMBIENT_DATA_DIR
AMBIENT_CONFIG_DIR=$(mktemp -d)/c
AMBIENT_CACHE_DIR=$(mktemp -d)/k
AMBIENT_DATA_DIR=$(mktemp -d)/d
mkdir -p "$AMBIENT_CONFIG_DIR"

# runners lack most screen deps; the screens must still render their
# graceful-availability states. coins included on purpose: a python
# startup crash (e.g. a NameError from a "cleanup") kills its window
# before remain-on-exit lands — that class belongs in CI, not on a kiosk
printf 'screens = ["now", "net", "coins", "docker", "events", "sec"]\nrotation_secs = 5\n' \
    > "$AMBIENT_CONFIG_DIR/ambient.toml"

note "env: $AMBIENT_CONFIG_DIR"

# ---- build & sanity ----------------------------------------------------
"$BIN" rebuild -d || { bad "rebuild failed"; exit 1; }
sleep 8   # let every screen run at least one frame

windows=$(tmux list-windows -t "$AMBIENT_SESSION" -F '#W' 2>/dev/null | tr '\n' ' ')
[ "$windows" = "now net coins docker events sec " ] && ok "windows: $windows" || bad "windows: got [$windows]"

# ---- per-pane assertions ------------------------------------------------
capture() { tmux capture-pane -p -t "$AMBIENT_SESSION:$1" 2>/dev/null; }

pane_ok() { # name label
    out=$(capture "$1")
    [ -n "$(printf '%s' "$out" | tr -d '[:space:]')" ] || { bad "$2: pane is empty"; return 1; }
    printf '%s' "$out" | grep -qE 'Bad substitution|syntax error|No such file or directory' \
        && { bad "$2: pane shows a shell error"; return 1; }
    ok "$2 rendered"
}

want() { # name pattern label  (pattern is ERE)
    capture "$1" | grep -qE "$2" && ok "$3" || bad "$3: pattern not found: $2"
}

pane_ok now    "now"
# toilet present locally -> weekday; absent (CI) -> install hint
capture now | grep -qE "$(date +%A)|now: missing|location" && ok "now content" || bad "now: no date/hint"
pane_ok docker "docker"
want   docker "docker" "docker header"
pane_ok coins "coins"
capture coins | grep -qE "BTC|\[D\]\[W\]|terminal too small|loading" && ok "coins content" || bad "coins: no render"
pane_ok events "events"
want   events "EVENTS" "events header"
pane_ok sec    "sec"
want   sec "SEC"     "sec header"
want   sec "exposed" "sec ports section"

# ---- panes feature: net is split (gping + tailnet) ----------------------
n=$(tmux list-panes -t "$AMBIENT_SESSION:net" -F x 2>/dev/null | wc -l | tr -d ' ')
[ "$n" = "2" ] && ok "net has 2 panes" || bad "net pane count: got $n, want 2"
want net.1 "tailscale not installed|tailnet" "tailnet pane (or its missing-binary state)"

# ---- chips end-to-end: write, render, land in the bar -------------------
"$BIN" chip smoke red 1 "SMOKE-CHIP"
out=$("$BIN" status)
printf '%s' "$out" | grep -q "SMOKE-CHIP" && ok "chip rendered by ambient status" || bad "chip missing from ambient status"
# the ticker (rotation_secs=5) should land it in the real bar too
sleep 7
tmux show-option -t "$AMBIENT_SESSION" status-right 2>/dev/null | grep -q "SMOKE-CHIP" \
    && ok "chip landed in status-right via ticker" || bad "ticker did not render chip into the bar"
"$BIN" chip smoke clear
out=$("$BIN" status)
printf '%s' "$out" | grep -q "SMOKE-CHIP" && bad "cleared chip still rendered" || ok "chip clears"

# ---- doctor exit semantics ----------------------------------------------
DOCT=$(mktemp -d)
mkdir -p "$DOCT/screens/good" "$DOCT/screens/starved"
printf 'name = "good"\nmode = "interactive"\nentry = "screen.sh"\nneeds = ["sh"]\n' > "$DOCT/screens/good/manifest.toml"
printf '#!/bin/sh\nwhile :; do sleep 30; done\n' > "$DOCT/screens/good/screen.sh"; chmod +x "$DOCT/screens/good/screen.sh"
printf 'name = "starved"\nmode = "interactive"\nentry = "screen.sh"\nneeds = ["definitely-not-a-real-command"]\n' > "$DOCT/screens/starved/manifest.toml"
printf '#!/bin/sh\nwhile :; do sleep 30; done\n' > "$DOCT/screens/starved/screen.sh"; chmod +x "$DOCT/screens/starved/screen.sh"

printf 'screens = ["good"]\n' > "$DOCT/ambient.toml"
AMBIENT_CONFIG_DIR="$DOCT" "$BIN" doctor >/dev/null 2>&1 && ok "doctor exits 0 when needs are met" || bad "doctor should exit 0 with needs met"
printf 'screens = ["starved"]\n' > "$DOCT/ambient.toml"
AMBIENT_CONFIG_DIR="$DOCT" "$BIN" doctor >/dev/null 2>&1 && bad "doctor should exit non-zero when needs are missing" || ok "doctor exits 1 when needs are missing"
rm -rf "$DOCT"

# ---- cleanup -------------------------------------------------------------
tmux kill-session -t "$AMBIENT_SESSION" 2>/dev/null
rm -rf "$AMBIENT_CONFIG_DIR" "$AMBIENT_CACHE_DIR" "$AMBIENT_DATA_DIR"

printf '\n'
[ "$FAIL" = 0 ] && { note "smoke: all green"; exit 0; }
note "smoke: $FAIL failure(s)"; exit 1
