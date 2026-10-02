#!/bin/sh
set -eu
CONFDIR=${AMBIENT_CONFIG_DIR:-$HOME/.config/ambient}/screens
CONF=$CONFDIR/clock.conf
mkdir -p "$CONFDIR"

command -v jq >/dev/null 2>&1 || {
    echo "jq is required to set up the clock (brew install jq / apt install jq)"
    exit 1
}

printf '\nambient clock setup\n\n'
printf 'Location for weather — zipcode, city name, or lat,lon.\n'
printf '  examples: 95050   |   Santa Clara   |   37.35,-121.95\n'
printf '(resolved to coordinates via wttr.in; alerts come from api.weather.gov,\n'
printf 'which only understands lat,lon — nothing else about you is sent)\n\n'
printf 'location: '
read -r LOC
[ -n "$LOC" ] || { echo "empty location, aborting"; exit 1; }
ENC=$(printf '%s' "$LOC" | sed 's/ /%20/g')

printf 'checking wttr.in... '
J=$(curl -s --max-time 10 "https://wttr.in/$ENC?format=j1" || true)
LAT=$(printf '%s' "$J" | jq -r '.nearest_area[0].latitude // empty' 2>/dev/null)
LON=$(printf '%s' "$J" | jq -r '.nearest_area[0].longitude // empty' 2>/dev/null)
if [ -z "$LAT" ] || [ -z "$LON" ]; then
    echo "could not resolve \"$LOC\" — nothing saved"
    exit 1
fi
POINT="$LAT,$LON"
echo "ok ($POINT)"

printf 'label (optional, e.g. town name): '
read -r LABEL

TMP=$CONF.tmp
{
    echo "loc=$POINT"
    echo "label=$LABEL"
} >"$TMP"
mv "$TMP" "$CONF"
echo "saved $CONF"
