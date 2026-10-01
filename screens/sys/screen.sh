#!/bin/sh
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
printf 'color_theme = "%s"\n' "$T" >"$CACHE/btop.conf"

if [ -n "${TMUX_PANE:-}" ] && { command -v defaults >/dev/null 2>&1 || command -v gsettings >/dev/null 2>&1; }; then
    (
        while :; do
            sleep 10
            [ "$(theme_now)" != "$T" ] || continue
            exec tmux respawn-window -k -t "$TMUX_PANE" "$0"
        done
    ) &
fi

exec btop -c "$CACHE/btop.conf"
