#!/bin/sh
set -eu
CONFDIR=${AMBIENT_CONFIG_DIR:-$HOME/.config/ambient}/screens
CONF=$CONFDIR/net.conf
mkdir -p "$CONFDIR"

printf '\nambient net setup\n\n'
printf 'Host the WAN pane pings to judge internet health.\n'
printf 'defaults to 1.1.1.1 — enter nothing to keep that.\n\n'
printf 'wan host [1.1.1.1]: '
read -r WAN
[ -n "$WAN" ] || WAN=1.1.1.1

printf 'checking %s... ' "$WAN"
if ! ping -c 1 "$WAN" >/dev/null 2>&1; then
    echo "no reply — nothing saved"
    exit 1
fi
echo ok

TMP=$CONF.tmp
echo "wan=$WAN" >"$TMP"
mv "$TMP" "$CONF"
echo "saved $CONF"
