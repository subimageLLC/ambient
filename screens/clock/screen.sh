#!/bin/sh
set -eu
CONF=${AMBIENT_CONFIG_DIR:-$HOME/.config/ambient}/screens/clock.conf
BOLD=$(printf '\033[1m')
DIM=$(printf '\033[90m')
WHITE=$(printf '\033[97m')
RESET=$(printf '\033[0m')

AMB=${AMBIENT_EXE:-ambient}
LOC=""
LABEL=""
FMT=${AMBIENT_CLOCK_FORMAT:-24h}

CONFDIR=$(dirname "$CONF")
if [ -f "$CONF" ]; then
    while IFS='=' read -r k v; do
        case "$k" in
            loc) LOC=$v ;;
            label) LABEL=$v ;;
            format) FMT=$v ;;
        esac
    done <"$CONF"
fi

if [ "$FMT" = "12h" ]; then
    HH=$(date +%I:%M)
    AM=$(date +"%p")
else
    HH=$(date +%H:%M)
    AM=""
fi

printf '\n\n'
date +"%a %b %d %Y" | sed "s/^/${DIM}        ${RESET}/"
printf '\n'
printf '%s   %s\n' "${WHITE}${BOLD}${HH}${RESET}" "${DIM}${AM}${RESET}"
printf '%s\n' "${DIM}        $(date %Z 2>/dev/null || true)${RESET}"

if [ -n "$LOC" ]; then
    URL="https://wttr.in/$LOC?format=j1"
    if BODY=$("$AMB" fetch --ttl 1800 --name weather "$URL" 2>/dev/null); then
        printf '\n'
        printf '%s\n' "$BODY" | python3 "$(dirname "$0")/weather.py"
        if [ -n "$LABEL" ]; then
            printf '%s\n' "${DIM}        (${LABEL})${RESET}"
        fi
    else
        printf '\n%s\n' "${DIM}  weather unreachable${RESET}"
    fi
else
    printf '\n%s\n' "${DIM}  weather not configured — run: ambient setup clock${RESET}"
fi
