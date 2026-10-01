#!/bin/sh
set -eu
BOLD=$(printf '\033[1m')
DIM=$(printf '\033[90m')
CYAN=$(printf '\033[36m')
RESET=$(printf '\033[0m')

AMB=${AMBIENT_EXE:-ambient}
CACHE=${AMBIENT_CACHE_DIR:-$HOME/.cache/ambient}
WAN=1.1.1.1
CONF=${AMBIENT_CONFIG_DIR:-$HOME/.config/ambient}/screens/net.conf
if [ -f "$CONF" ]; then
    WAN=$(sed -n 's/^wan=//p' "$CONF" | tail -1)
fi

gateway() {
    if command -v ip >/dev/null 2>&1; then
        ip route 2>/dev/null | awk '/default/ {print $3; exit}'
    elif command -v route >/dev/null 2>&1; then
        route -n get default 2>/dev/null | awk '/gateway/ {print $2; exit}'
    fi
}

GW=$(gateway || true)

pingstats() {
    host=$1
    out=$(ping -c 4 "$host" 2>&1 || true)
    avg=$(printf '%s\n' "$out" | awk -F= '/(avg|rtt)/ {split($2,a,"/"); print a[2]; exit}')
    loss=$(printf '%s\n' "$out" | awk '/packet loss/ {for (i=1;i<=NF;i++) if ($i ~ /%/) {print $i; exit}}')
    if [ -n "$avg" ]; then
        hist="$CACHE/net.$host.hist"
        mkdir -p "$CACHE"
        printf '%s\n' "$avg" >>"$hist"
        tail -40 "$hist" >"$hist.tmp" && mv "$hist.tmp" "$hist"
    fi
    printf '%s %s\n' "${avg:-}" "${loss:-?}"
}

printf '\n %snetwork%s\n\n' "$BOLD" "$RESET"

if [ -n "$GW" ]; then
    set -- $(pingstats "$GW")
    AVG=$1
    LOSS=$2
    printf '  gateway  %-16s %s ms   loss %s\n' "$GW" "${AVG:-—}" "$LOSS"
    if [ -n "${AVG:-}" ]; then
        tail -40 "$CACHE/net.$GW.hist" | "$AMB" chart volume --height 1 --width 40 | sed 's/^/           /'
    fi
else
    printf '  %sgateway not detected%s\n' "$DIM" "$RESET"
fi

printf '\n'
set -- $(pingstats "$WAN")
AVG=$1
LOSS=$2
printf '  wan      %-16s %s ms   loss %s\n' "$WAN" "${AVG:-—}" "$LOSS"
if [ -n "${AVG:-}" ]; then
    tail -40 "$CACHE/net.$WAN.hist" | "$AMB" chart volume --height 1 --width 40 | sed 's/^/           /'
fi

printf '\n %stailnet-free, cloud-free — just pings%s\n' "$DIM" "$RESET"
