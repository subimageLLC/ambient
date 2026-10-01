#!/usr/bin/env bash
# faithful port of the kiosk's screen-clock (toilet big time + wttr.in rows)
# deltas from the kiosk: LOC/label from clock.conf, caches under AMBIENT_CACHE_DIR,
# portable stat, empty loc = clock without weather.
FONT="${FONT:-mono12}"
CONF_DIR="${AMBIENT_CONFIG_DIR:-$HOME/.config/ambient}/screens"
CONF="$CONF_DIR/clock.conf"
LOC=""
CITY=""
[ -f "$CONF" ] && while IFS='=' read -r k v; do
    case "$k" in
        loc) LOC=$v ;;
        label) CITY=$v ;;
    esac
done <"$CONF"

FG="\033[38;5;244m"
DIM="\033[2m"
GRN="\033[1;32m"
RED="\033[1;31m"
YEL="\033[1;33m"
CYN="\033[36m"
WHT="\033[1;37m"
OFF="\033[0m"
WFILE="${AMBIENT_CACHE_DIR:-$HOME/.cache/ambient}/weather.json"
AFILE="${AMBIENT_CACHE_DIR:-$HOME/.cache/ambient}/weather-alerts.json"

missing_deps() {
    local m=""
    command -v toilet >/dev/null 2>&1 || m="$m toilet"
    command -v jq >/dev/null 2>&1 || m="$m jq"
    printf '%s' "$m"
}

install_hint() {
    if command -v brew >/dev/null 2>&1; then echo "brew install$1"
    elif command -v apt-get >/dev/null 2>&1; then echo "sudo apt install$1"
    elif command -v dnf >/dev/null 2>&1; then echo "sudo dnf install$1"
    elif command -v pacman >/dev/null 2>&1; then echo "sudo pacman -S$1"
    else echo "install$1 with your package manager"
    fi
}

while [ -n "$(missing_deps)" ]; do
    M=$(missing_deps)
    printf '\033[H\033[2J'
    printf ' clock: missing%s\n\n' "$M"
    printf '  %s\n\n' "$(install_hint "$M")"
    printf ' this pane recovers automatically once installed\n'
    sleep 30
done

mtime() { stat -c %Y "$1" 2>/dev/null || stat -f %m "$1" 2>/dev/null || echo 0; }

c2f() {
    awk -v c="$1" 'BEGIN{printf "%d", c * 9 / 5 + 32}'
}

weather() {
    local now=$(date +%s)
    if [ ! -s "$WFILE" ] || [ $(( now - $(mtime "$WFILE") )) -gt 1800 ]; then
        curl -sf --max-time 12 "https://wttr.in/$LOC?format=j1" -o "$WFILE.new" 2>/dev/null \
            && mv "$WFILE.new" "$WFILE"
    fi
    [ -s "$WFILE" ] || return 1

    local nowF desc hi lo tmhi tmlo tmdesc
    nowF=$(jq -r '.current_condition[0].temp_F' "$WFILE" 2>/dev/null)
    desc=$(jq -r '.current_condition[0].weatherDesc[0].value' "$WFILE" 2>/dev/null | xargs)
    hi=$(c2f "$(jq -r '.weather[0].maxtempC' "$WFILE")")
    lo=$(c2f "$(jq -r '.weather[0].mintempC' "$WFILE")")
    tmhi=$(c2f "$(jq -r '.weather[1].maxtempC' "$WFILE")")
    tmlo=$(c2f "$(jq -r '.weather[1].mintempC' "$WFILE")")
    tmdesc=$(jq -r '.weather[1].hourly[4].weatherDesc[0].value // .weather[1].hourly[0].weatherDesc[0].value // "?"' "$WFILE" 2>/dev/null | xargs)

    W_OUT=()
    W_PLAIN=()
    W_OUT+=("$DIM$(printf '%-10s' Now)$OFF\033[37m${nowF}°$OFF  $DIM${desc}$OFF")
    W_PLAIN+=("$(printf '%-10s' Now)${nowF}°  $desc")
    W_OUT+=("$DIM$(printf '%-10s' Today)$OFF\033[33m▲ ${hi}°$OFF  \033[36m▼ ${lo}°$OFF")
    W_PLAIN+=("$(printf '%-10s' Today)▲ ${hi}°  ▼ ${lo}°")
    W_OUT+=("$DIM$(printf '%-10s' Tomorrow)$OFF\033[33m▲ ${tmhi}°$OFF  \033[36m▼ ${tmlo}°$OFF  $DIM${tmdesc}$OFF")
    W_PLAIN+=("$(printf '%-10s' Tomorrow)▲ ${tmhi}°  ▼ ${tmlo}°  $tmdesc")

    ALERT_OUT=""
    ALERT_PLAIN=""
    if [ ! -s "$AFILE" ] || [ $(( now - $(mtime "$AFILE") )) -gt 600 ]; then
        curl -sf --max-time 12 -H "User-Agent: ambient-kiosk" \
            "https://api.weather.gov/alerts/active?point=$LOC" -o "$AFILE.new" 2>/dev/null \
            && mv "$AFILE.new" "$AFILE"
    fi
    if [ -s "$AFILE" ]; then
        local sev ev ends
        sev=$(jq -r '[.features[].properties.severity] | max_by(if . == "Extreme" then 4 elif . == "Severe" then 3 elif . == "Moderate" then 2 else 1 end) // empty' "$AFILE" 2>/dev/null)
        ev=$(jq -r '[.features[].properties.event] | join(", ")' "$AFILE" 2>/dev/null)
        ends=$(jq -r '.features[0].properties.ends // empty' "$AFILE" 2>/dev/null)
        if [ -n "$ev" ]; then
            local col=$YEL
            [ "$sev" = Extreme ] || [ "$sev" = Severe ] && col=$RED
            local when=""
            [ -n "$ends" ] && when=" until $(date -d "$ends" +%H:%M 2>/dev/null || date -jf "%Y-%m-%dT%H:%M:%S%z" "$ends" +%H:%M 2>/dev/null)"
            ALERT_OUT="$col⚠ ${ev}${when}$OFF"
            ALERT_PLAIN="⚠ ${ev}${when}"
        fi
    fi
    return 0
}

tick=0
W_OUT=(); W_PLAIN=(); ALERT_OUT=""; ALERT_PLAIN=""

trap ' : ' WINCH

stable=0
last_sz=""
while [ $stable -lt 2 ]; do
    cur_sz="$(tput cols)x$(tput lines)"
    if [ "$cur_sz" = "$last_sz" ]; then stable=$((stable + 1)); else stable=0; fi
    last_sz=$cur_sz
    sleep 0.15
done

while :; do
    C=$(tput cols) L=$(tput lines)
    T=$(date +%I:%M) D=$(date '+%A, %B %-d')

    if (( tick % 30 == 0 )); then
        if [ -n "$LOC" ]; then
            weather || { W_OUT=("$DIM weather unavailable$OFF"); W_PLAIN=(" weather unavailable"); ALERT_OUT=""; ALERT_PLAIN=""; }
        else
            W_OUT=("$DIM set location: ambient setup clock$OFF"); W_PLAIN=(" set location: ambient setup clock"); ALERT_OUT=""; ALERT_PLAIN=""
        fi
    fi

    mapfile -t art < <(toilet -f "$FONT" "$T")
    h=${#art[@]}
    w=0
    for line in "${art[@]}"; do (( ${#line} > w )) && w=${#line}; done

    nlines=$(( ${#W_OUT[@]} + ( ${#ALERT_PLAIN} > 0 ? 1 : 0 ) ))
    pad=$(( (L - h - 2 - nlines) / 2 )); (( pad < 0 )) && pad=0
    padb=$(( L - pad - h - 2 - nlines )); (( padb < 0 )) && padb=0
    left=$(( (C - w) / 2 )); (( left < 0 )) && left=0

    maxw=0
    for p in "${W_PLAIN[@]}"; do (( ${#p} > maxw )) && maxw=${#p}; done
    wl=$(( (C - maxw) / 2 )); (( wl < 0 )) && wl=0

    printf '\033[H'
    for ((i = 0; i < pad; i++)); do printf '\033[K\n'; done
    for line in "${art[@]}"; do
        printf '%*s' "$left" ''
        printf '%b%s%b\033[K\n' "$FG" "$line" "$OFF"
    done
    printf '\033[K\n'
    dl=$(( (C - ${#D}) / 2 )); (( dl < 0 )) && dl=0
    printf '%*s' "$dl" ''
    printf '%b%s%b\033[K\n' "$WHT" "$D" "$OFF"

    for i in "${!W_OUT[@]}"; do
        printf '%*s' "$wl" ''
        printf '%b\033[K\n' "${W_OUT[$i]}"
    done
    if [ -n "$ALERT_PLAIN" ]; then
        al=$(( (C - ${#ALERT_PLAIN}) / 2 )); (( al < 0 )) && al=0
        printf '%*s' "$al" ''
        printf '%b\033[K\n' "$ALERT_OUT"
    fi
    for ((i = 0; i < padb; i++)); do printf '\033[K'; (( i < padb - 1 )) && printf '\n'; done

    tick=$((tick + 1))
    sleep 1 &
    wait $!
done
