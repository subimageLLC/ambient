#!/bin/sh
# pick btop theme to match the desktop/terminal appearance
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

THEME=solarized_dark
is_light && THEME=solarized_light

if command -v btop >/dev/null 2>&1; then
    mkdir -p "$CACHE"
    printf 'color_theme = "%s"\n' "$THEME" >"$CACHE/btop.conf"
    exec btop -c "$CACHE/btop.conf"
elif command -v htop >/dev/null 2>&1; then
    exec htop
else
    exec top
fi
