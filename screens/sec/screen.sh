#!/bin/sh
# sec — full-screen security rail. Port of the kiosk's screen-sec (which lived as
# the clock's 25% sidepanel there); the unifi threat feed is optional —
# a private integration refreshes $AMBIENT_CACHE_DIR/unifi.json.
EXE="${AMBIENT_EXE:-ambient}"
CACHE="${AMBIENT_CACHE_DIR:-$HOME/.cache/ambient}"
UCACHE="$CACHE/unifi.json"
C="\033[1;36m"; D="\033[2m"; R="\033[1;31m"; G="\033[1;32m"; Y="\033[1;33m"; OFF="\033[0m"

ssh_journal() { # unit name that exists on this box, or ""
    command -v journalctl >/dev/null 2>&1 || return 1
    journalctl --quiet --no-pager -u ssh  --since "24 hours ago" -n 1 >/dev/null 2>&1 && { echo ssh;  return 0; }
    journalctl --quiet --no-pager -u sshd --since "24 hours ago" -n 1 >/dev/null 2>&1 && { echo sshd; return 0; }
    return 1
}

while :; do
    W=$(tput cols)
    # ports list shrinks with the pane: at most 6, fewer when short
    MAXPORTS=$(( $(tput lines) - 12 ))
    [ "$MAXPORTS" -gt 6 ] && MAXPORTS=6
    [ "$MAXPORTS" -lt 1 ] && MAXPORTS=1
    printf '\033[H\033[2J'
    printf '%bSEC%b %s\n' "$C" "$OFF" "$(date +%H:%M)"
    printf '%b' "$D"
    printf '─%.0s' $(seq 1 $(( W > 4 ? W - 2 : 10 )))
    printf '%b\n\n' "$OFF"

    # ---- threats (optional unifi feed) ----
    printf '%bthreats%b\n' "$D" "$OFF"
    fresh=0
    # -L: UCACHE may be a symlink into a shared cache (the kiosk's watcher setup)
    [ -s "$UCACHE" ] && [ $(( $(date +%s) - $(stat -L -c %Y "$UCACHE" 2>/dev/null || stat -f %m "$UCACHE" 2>/dev/null || echo 0) )) -lt 600 ] && fresh=1
    if [ "$fresh" -eq 1 ] && command -v jq >/dev/null 2>&1; then
        n=$(jq -r '[(.ips // [])[] | select((keys | length) > 0)] | length' "$UCACHE" 2>/dev/null)
        if [ "${n:-0}" -gt 0 ]; then
            cat=$(jq -r '[(.ips // [])[] | select((keys | length) > 0)] | (.[0].catName // .[0].category // .[0].name // "?")' "$UCACHE" 2>/dev/null)
            printf '  %b⚠ %s%b\n' "$R" "$n" "$OFF"
            printf '  %b%s%b\n' "$D" "$(printf "%.*s" $((W - 4)) "$cat")" "$OFF"
            "$EXE" chip sec orange 2 "⚠${n}threats" 2>/dev/null
        else
            printf '  %bnone%b\n' "$G" "$OFF"
            "$EXE" chip sec clear 2>/dev/null
        fi
    else
        printf '  %bunifi: not configured%b\n' "$D" "$OFF"
        "$EXE" chip sec clear 2>/dev/null
    fi

    # ---- ssh brute-force, last 24h ----
    printf '\n%bssh 24h%b\n' "$D" "$OFF"
    unit=$(ssh_journal || true)
    if [ -n "$unit" ]; then
        att=$(journalctl -u "$unit" --since "24 hours ago" --no-pager 2>/dev/null \
            | grep -oE 'Failed [^ ]+ .+ from ([0-9]+\.){3}[0-9]+' \
            | grep -oE '([0-9]+\.){3}[0-9]+' \
            | sort | uniq -c | sort -rn | head -1)
        if [ -n "$att" ]; then
            cnt=$(printf '%s' "$att" | awk '{print $1}')
            ip=$(printf '%s' "$att" | awk '{print $2}')
            printf '  %b%s%b\n' "$R" "$ip" "$OFF"
            printf '  %b×%s fails%b\n' "$D" "$cnt" "$OFF"
        else
            printf '  %bnone%b\n' "$G" "$OFF"
        fi
    else
        printf '  %bn/a — no systemd ssh journal on this host%b\n' "$D" "$OFF"
    fi

    # ---- exposed listening ports ----
    printf '\n%bexposed%b\n' "$D" "$OFF"
    if command -v ss >/dev/null 2>&1; then
        rows=$( { sudo -n ss -tulnpH 2>/dev/null || ss -tulnpH 2>/dev/null; } | awk '{
            addr = $5
            rest = $0
            if (sub(/^.*users:\(\("/, "", rest)) { sub(/".*$/, "", rest) }
            else rest = "?"
            port = addr
            sub(/^.*:/, "", port)
            disp = ""
            if (addr ~ /^0\.0\.0\.0:/ || addr ~ /^\[::\]:/ || addr ~ /^\*:/) disp = "*:" port
            else if (addr ~ /^100\./) disp = "t:" port
            if (disp != "") print disp, rest
        }' | sort -u)
    elif command -v lsof >/dev/null 2>&1; then
        # +c 0: lsof truncates COMMAND to 9 chars by default (ControlCe…);
        # \x20 escapes spaces in names (Adobe\x20Desktop\x20Service)
        rows=$(lsof +c 0 -nP -iTCP -sTCP:LISTEN 2>/dev/null | awk 'NR>1 {
            proc = $1; addr = $9
            gsub(/\\x20/, " ", proc)
            port = addr; sub(/^.*:/, "", port)
            disp = ""
            if (addr ~ /^\*:/ || addr ~ /^0\.0\.0\.0:/ || addr ~ /^\[::\]:/) disp = "*:" port
            else if (addr ~ /^100\./) disp = "t:" port
            if (disp != "") print disp, proc
        }' | sort -u)
    else
        rows=""
    fi
    if [ -n "$rows" ]; then
        lan=$(printf '%s\n' "$rows" | grep -c '^\*:' || true)
        tn=$(printf '%s\n' "$rows" | grep -c '^t:' || true)
        printf '  %b%s%b lan  %b%s%b ts\n' "$Y" "${lan:-0}" "$OFF" "$C" "${tn:-0}" "$OFF"
        printf '\n%bports%b\n' "$D" "$OFF"
        nl=0
        printf '%s\n' "$rows" | while read -r disp name; do
            [ -z "$disp" ] && continue
            nl=$((nl + 1))
            [ "$nl" -ge "$MAXPORTS" ] && break
            printf '  %b%-9s%b %s\n' "$Y" "$disp" "$OFF" "$(printf "%.*s" $((W - 12)) "$name")"
        done
        [ "${lan:-0}" -gt 0 ] || printf '  %bnone%b\n' "$D" "$OFF"
    else
        printf '  %bn/a — no ss or lsof on this host%b\n' "$D" "$OFF"
    fi

    sleep 30
done
