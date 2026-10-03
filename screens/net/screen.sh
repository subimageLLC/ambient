#!/bin/sh
# faithful port of the kiosk's net window: gping gateway + wan
CONF_DIR="${AMBIENT_CONFIG_DIR:-$HOME/.config/ambient}/screens"
CONF="$CONF_DIR/net.conf"
WAN=1.1.1.1
[ -f "$CONF" ] && WAN=$(sed -n 's/^wan=//p' "$CONF" | tail -1)
: "${WAN:=1.1.1.1}"   # hand-edited/empty conf must not hand gping an empty target

GW=""
if command -v ip >/dev/null 2>&1; then
    GW=$(ip route 2>/dev/null | awk '/default/ {print $3; exit}')
elif command -v route >/dev/null 2>&1; then
    GW=$(route -n get default 2>/dev/null | awk '/gateway/ {print $2; exit}')
fi

while ! command -v gping >/dev/null 2>&1; do
    HINT='install gping with your package manager'
    command -v brew >/dev/null 2>&1 && HINT='brew install gping'
    command -v apt-get >/dev/null 2>&1 && HINT='sudo apt install gping'
    command -v dnf >/dev/null 2>&1 && HINT='sudo dnf install gping'
    command -v zypper >/dev/null 2>&1 && HINT='sudo zypper install gping'
    command -v pacman >/dev/null 2>&1 && HINT='sudo pacman -S gping'
    printf '\033[H\033[2J'
    printf ' net: missing gping\n\n'
    printf '  %s\n\n' "$HINT"
    printf ' this pane recovers automatically once installed\n'
    sleep 30
done

exec gping ${GW:+"$GW"} "$WAN"
