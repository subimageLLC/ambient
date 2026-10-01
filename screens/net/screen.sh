#!/bin/sh
# faithful port of the kiosk's net window: gping gateway + wan
CONF_DIR="${AMBIENT_CONFIG_DIR:-$HOME/.config/ambient}/screens"
CONF="$CONF_DIR/net.conf"
WAN=1.1.1.1
[ -f "$CONF" ] && WAN=$(sed -n 's/^wan=//p' "$CONF" | tail -1)

GW=""
if command -v ip >/dev/null 2>&1; then
    GW=$(ip route 2>/dev/null | awk '/default/ {print $3; exit}')
elif command -v route >/dev/null 2>&1; then
    GW=$(route -n get default 2>/dev/null | awk '/gateway/ {print $2; exit}')
fi

command -v gping >/dev/null 2>&1 || {
    printf 'gping is required for the net screen\n'
    printf '  macOS: brew install gping   Debian: apt install gping\n'
    sleep 3600
    exit 1
}

exec gping ${GW:+"$GW"} "$WAN"
