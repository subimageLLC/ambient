#!/bin/sh
# read-only docker health — port of the kiosk's screen-docker (the pre-lazydocker
# version). Interactive control (lazydocker) is an opt-in setup, not the default.
# Pane-size aware: table rows capped to the pane, stats block dropped when
# short, lines truncated to width (never wrapped).
EXE="${AMBIENT_EXE:-ambient}"
C="\033[1;36m"; D="\033[2m"; G="\033[1;32m"; Y="\033[1;33m"; R="\033[1;31m"; OFF="\033[0m"

# RUNP: never let a wedged docker call freeze the pane — wrap in timeout
# where the platform has one (linux yes; macOS usually not)
RUNP() {
    if command -v timeout >/dev/null 2>&1; then
        timeout 15 "$@"
    else
        "$@"
    fi
}

while :; do
    printf '\033[H'
    W=$(tput cols 2>/dev/null || echo 80)
    L=$(tput lines 2>/dev/null || echo 24)
    # full redraw when the pane resizes, else stale rows linger
    if [ "$W" != "${lastW:-0}" ] || [ "$L" != "${lastL:-0}" ]; then
        printf '\033[H\033[2J'
        lastW=$W; lastL=$L
    fi

    if ! command -v docker >/dev/null 2>&1; then
        HINT='install docker with your package manager'
        command -v brew    >/dev/null 2>&1 && HINT='brew install --cask docker (or colima + docker-cli)'
        command -v apt-get >/dev/null 2>&1 && HINT='sudo apt install docker.io'
        command -v zypper  >/dev/null 2>&1 && HINT='sudo zypper install docker'
        printf '\033[2J'
        printf ' docker: missing docker\n\n'
        printf '  %s\n\n' "$HINT"
        printf '\033[2m this pane recovers automatically once installed\033[0m\n'
        "$EXE" chip docker clear 2>/dev/null
        sleep 30
        continue
    fi

    # plain access first; fall back to a scoped sudoers rule (the kiosk's model)
    DC=""
    if docker ps >/dev/null 2>&1; then
        DC="docker"
    elif sudo -n docker ps >/dev/null 2>&1; then
        DC="sudo -n docker"
    fi
    if [ -z "$DC" ]; then
        printf '\033[2J'
        printf ' docker\n\n'
        printf '\033[2m  no access — daemon down, or this user lacks docker rights\033[0m\n'
        printf '\033[2m  read-only: sudoers entries for docker ps/stats/logs (see README)\033[0m\n'
        "$EXE" chip docker clear 2>/dev/null
        sleep 30
        continue
    fi

    printf '\033[K\033[J'
    printf ' %bdocker%b  %s\n\n' "$C" "$OFF" "$(date '+%H:%M')"

    down=0
    total=$($DC ps -aq 2>/dev/null | wc -l | tr -d ' ')
    # chip: running containers in bad states — "Up ... (unhealthy)" still
    # starts with Up, so match the bad words instead of inverting good ones
    down=$($DC ps --format '{{.Status}}' 2>/dev/null | grep -Eci 'unhealthy|Restarting|Dead' || true)
    [ "${down:-0}" -gt 0 ] \
        && "$EXE" chip docker red 2 "⚠${down}cont" 2>/dev/null \
        || "$EXE" chip docker clear 2>/dev/null

    # rows budget: header(2) + blank + summary(1) at minimum; stats block
    # only when there is room after the table
    table=$((L - 4))
    stats_rows=$((L - 4 - table))
    if [ "$L" -ge 14 ]; then
        table=$((L - 10))       # reserve: blank + stats(5) + blank + summary
        stats_rows=5
    fi
    [ "$table" -lt 1 ] && table=1

    shown=0
    $DC ps -a --format '{{.Names}}\t{{.Status}}\t{{.Image}}' 2>/dev/null \
        | while IFS="	" read -r name status image; do
            shown=$((shown + 1))
            [ "$shown" -gt "$table" ] && exit 0
            case "$status" in
                Up*|healthy*)
                    case "$status" in
                        *unhealthy*) col=$R ;;
                        *)           col=$G ;;
                    esac ;;
                Restarting*|Dead*) col=$R ;;
                Exited*|Created*)  col=$D ;;
                *)                 col=$Y ;;
            esac
            printf '  %-20.20s %b%-26.26s%b %.*s\n' "$name" "$col" "$status" "$OFF" $((W - 52)) "$image"
        done
    hidden=$(( ${total:-0} - table ))
    [ "$hidden" -gt 0 ] && printf '\033[2m  … %d more containers\033[0m\n' "$hidden"

    if [ "$stats_rows" -gt 0 ]; then
        printf '\n'
        # one stats sample drives both the rows and the Σ; some daemons
        # (the kiosk's, cgroup reporting) always answer 0B / 0B — if so, drop
        # the memory column instead of printing a wall of zeros
        stats=$(RUNP $DC stats --no-stream --format '{{.Name}}\t{{.CPUPerc}}\t{{.MemUsage}}' 2>/dev/null)
        if printf '%s' "$stats" | grep -qv '0B / 0B'; then
            printf '%s' "$stats" | head -"$stats_rows" \
                | while IFS="	" read -r name cpu mem; do
                    printf '\033[2m  %-20.20s %6s  %s\033[0m\n' "$name" "$cpu" "$mem"
                done
        else
            printf '%s' "$stats" | head -"$stats_rows" \
                | while IFS="	" read -r name cpu _; do
                    printf '\033[2m  %-20.20s cpu %6s\033[0m\n' "$name" "$cpu"
                done
        fi
        sumcpu=$(printf '%s' "$stats" | awk -F'\t' '{ gsub("%", "", $2); s += $2 } END { printf "%.1f", s }')
        printf '\033[2m  %-20s cpu %5s%%\033[0m\n' "Σ $(printf '%s' "$stats" | grep -c .)" "$sumcpu"
        # disk usage, where the daemon (and the sudoers scope) allows it
        df=$(RUNP $DC system df --format '{{.Type}}\t{{.Size}}' 2>/dev/null) && [ -n "$df" ] && {
            dimg=$(printf '%s' "$df" | awk -F'\t' '$1 == "Images" {print $2}')
            dcon=$(printf '%s' "$df" | awk -F'\t' '$1 == "Containers" {print $2}')
            dvol=$(printf '%s' "$df" | awk -F'\t' '$1 == "Local Volumes" {print $2}')
            printf '\033[2m  disk  images %s · containers %s · volumes %s\033[0m\n' "${dimg:-?}" "${dcon:-?}" "${dvol:-?}"
        }
    fi
    printf '\033[2m %s containers · read-only view\033[0m\n' "${total:-0}"
    printf '\033[K\033[J'
    sleep 10
done
