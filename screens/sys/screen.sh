#!/bin/sh
# shellcheck disable=SC2093  # exec is the point: this screen becomes the monitor
# btop with a theme that follows the desktop appearance.
# btop must stay foreground (it refuses background), so a watcher
# respawns this window when the appearance flips.
CACHE=${AMBIENT_CACHE_DIR:-$HOME/.cache/ambient}

is_light() {
    if command -v defaults >/dev/null 2>&1; then
        defaults read -g AppleInterfaceStyle >/dev/null 2>&1 || return 0
        return 1
    fi
    if command -v gsettings >/dev/null 2>&1; then
        gsettings get org.gnome.desktop.interface color-scheme 2>/dev/null | grep -q light && return 0
        return 1
    fi
    case "${COLORFGBG##*;}" in 7|15) return 0 ;; esac
    return 1
}

theme_now() {
    if is_light; then echo solarized_light; else echo solarized_dark; fi
}

if ! command -v btop >/dev/null 2>&1; then
    if command -v htop >/dev/null 2>&1; then
        exec htop
    fi
    exec top
fi

mkdir -p "$CACHE"
T=$(theme_now)
# flip only the theme — btop writes box/layout tweaks back into this conf,
# and a blind rewrite would reset them on every appearance change. POSIX
# only: no sed -i (BSD sed wants a backup suffix); a missing theme line is
# appended; every other line survives verbatim.
[ -f "$CACHE/btop.conf" ] || : >"$CACHE/btop.conf"
awk -v t="$T" '
    /^color_theme[[:space:]]*=/ { printf "color_theme = \"%s\"\n", t; found = 1; next }
    { print }
    END { if (!found) printf "color_theme = \"%s\"\n", t }
' "$CACHE/btop.conf" >"$CACHE/btop.conf.tmp" && mv "$CACHE/btop.conf.tmp" "$CACHE/btop.conf"

sq() { printf "'%s'" "$(printf '%s' "$1" | sed "s/'/'\\\\''/g")"; }
RESTART="AMBIENT_CONFIG_DIR=$(sq "${AMBIENT_CONFIG_DIR:-}")"
RESTART="$RESTART AMBIENT_DATA_DIR=$(sq "${AMBIENT_DATA_DIR:-}")"
RESTART="$RESTART AMBIENT_CACHE_DIR=$(sq "${AMBIENT_CACHE_DIR:-}")"
RESTART="$RESTART $(sq "$0")"

WATCHER=""
if [ -n "${TMUX_PANE:-}" ] && { command -v defaults >/dev/null 2>&1 || command -v gsettings >/dev/null 2>&1; }; then
    (
        while :; do
            sleep 10
            [ "$(theme_now)" != "$T" ] || continue
            exec tmux respawn-window -k -t "$TMUX_PANE" "$RESTART"
        done
    ) &
    WATCHER=$!
fi

btop -c "$CACHE/btop.conf"
RC=$?
[ -n "$WATCHER" ] && kill "$WATCHER" 2>/dev/null
exit $RC
