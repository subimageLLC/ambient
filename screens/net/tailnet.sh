#!/bin/sh
# tailnet peer dots — port of the kiosk's screen-tailnet, pane-size aware:
# peers capped to pane rows, lines truncated to width (never wrapped)
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
        printf '\033[2m  tailscale not installed — this pane recovers once it is\033[0m\n'
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

    tailscale status --json | jq -r '
        (.Self | "  \u001b[1;32m●\u001b[0m " + (.HostName // "?") + " \u001b[2m(this device)\u001b[0m"),
        ((.Peer // {}) | to_entries | map(.value) | sort_by(.Online | not) | .[] |
            "  " + (if .Online then "\u001b[1;32m●\u001b[0m" else "\u001b[90m○\u001b[0m" end)
            + " " + (.HostName // "?") + " \u001b[2m" + (.OS // "") + "\u001b[0m")' 2>/dev/null \
        | awk -v w="$W" -v max="$max" 'NR <= max { print substr($0, 1, w) "\033[K" }'

    hidden=$((total + 1 - max))
    [ "$hidden" -gt 0 ] && printf '\033[2m  +%d more peers\033[0m\n' "$hidden"
    printf '\033[K\033[J'
    sleep 15
done
