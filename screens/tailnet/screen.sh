#!/bin/sh
# tailnet — self + peer dots, online first. Pane-size aware: peers capped to
# pane rows, lines truncated to width (never wrapped). The kiosk runs this same
# script as the bottom pane of its private lan screen.
while :; do
    printf '\033[H'
    W=$(tput cols 2>/dev/null || echo 80)
    L=$(tput lines 2>/dev/null || echo 24)
    # full redraw when the pane resizes, else stale rows linger
    if [ "$W" != "${lastW:-0}" ] || [ "$L" != "${lastL:-0}" ]; then
        printf '\033[H\033[2J'
        lastW=$W; lastL=$L
    fi

    if ! command -v tailscale >/dev/null 2>&1; then
        printf '\033[2J'
        printf ' tailnet\n\n'
        printf '\033[2m  tailscale not installed — this screen recovers once it is\033[0m\n'
        sleep 30
        continue
    fi

    printf ' \033[1;36mtailnet\033[0m  %s\n\n' "$(date '+%H:%M')"

    total=$(tailscale status --json 2>/dev/null | jq -r '(.Peer // {}) | length' 2>/dev/null)
    total=${total:-0}
    # self + peers = total+1 lines available in the list; header eats 2
    list=$((L - 2))
    max=$((total + 1))
    [ "$list" -lt "$max" ] && max="$list"
    [ "$max" -lt 1 ] && max=1

    # data first, decoration second: jq emits status\x1fhost\x1fos records and
    # awk strips control chars from those untrusted fields BEFORE adding ANSI.
    # Stripping after the fact (the old net/tailnet.sh bug) eats the ESC bytes
    # and prints literal "[1;32m●[0m" garbage.
    tailscale status --json | jq -r '
        (.Self | "S\u001f" + (.HostName // "?")),
        ((.Peer // {}) | to_entries | map(.value) | sort_by(.Online | not) | .[] |
            (if .Online then "O" else "F" end) + "\u001f" + (.HostName // "?") + "\u001f" + (.OS // ""))' 2>/dev/null \
        | awk -F'\037' -v w="$W" -v max="$max" '
            { for (i = 1; i <= NF; i++) gsub(/[[:cntrl:]]/, "", $i) }
            {
                dot = "\033[1;32m●\033[0m"
                note = $3
                if ($1 == "F") dot = "\033[90m○\033[0m"
                if ($1 == "S") note = "(this device)"
                line = "  " dot " " $2 " \033[2m" note "\033[0m"
                if (++n <= max) print substr(line, 1, w) "\033[K"
            }'

    hidden=$((total + 1 - max))
    [ "$hidden" -gt 0 ] && printf '\033[2m  +%d more peers\033[0m\n' "$hidden"
    printf '\033[K\033[J'
    sleep 15
done
