#!/bin/sh
set -eu
CONFDIR=${AMBIENT_CONFIG_DIR:-$HOME/.config/ambient}/screens
CONF=$CONFDIR/clock.conf
mkdir -p "$CONFDIR"

printf '\nambient clock setup\n\n'
printf 'Location for weather — zipcode, city name, or lat,lon.\n'
printf '  examples: 95050   |   Santa Clara   |   37.35,-121.95\n'
printf '(this is sent to wttr.in as the query; nothing else about you is)\n\n'
printf 'location: '
read -r LOC
[ -n "$LOC" ] || { echo "empty location, aborting"; exit 1; }
ENC=$(printf '%s' "$LOC" | sed 's/ /%20/g')

printf 'checking wttr.in... '
if ! curl -s --max-time 10 "https://wttr.in/$ENC?format=j1" | head -c 1 | grep -q '{'; then
    echo "could not resolve \"$LOC\" — nothing saved"
    exit 1
fi
echo ok

printf 'label (optional, shown under the clock): '
read -r LABEL

printf 'clock format — 24h or 12h [24h]: '
read -r FMT
case "$FMT" in
    12*) FMT=12h ;;
    *) FMT=24h ;;
esac

TMP=$CONF.tmp
{
    echo "loc=$ENC"
    echo "label=$LABEL"
    echo "format=$FMT"
} >"$TMP"
mv "$TMP" "$CONF"
echo "saved $CONF"
