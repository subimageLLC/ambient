#!/bin/sh
# events — what happened, when, and whether it is still broken.
# Replaces the old logs screen (logsum + lnav): a raw stream was noise
# theater — unreadable from across the room, unactionable up close. Small
# local pattern rules turn the journal into plain-language events instead.
# No agent, no cloud.
#   linux: systemd unit failures + crash loops (state from systemctl),
#          OOM kills, disk I/O errors, ssh brute-force
#   macOS: hum only — no systemd or kernel journal to mine
# hum: all other errors, grouped by source, one footer line.
# mute: ~/.config/ambient/screens/events-mute — one source per line
#   (unit names, proc names, "kernel:<driver>"); muted sources stop paging
#   but stay counted in the hum.
EXE="${AMBIENT_EXE:-ambient}"
CONF="${AMBIENT_CONFIG_DIR:-$HOME/.config/ambient}/screens"
MUTE="$CONF/events-mute"
C="\033[1;36m"; D="\033[2m"; G="\033[1;32m"; Y="\033[1;33m"; R="\033[1;31m"; OFF="\033[0m"

MUTE_AWK='
    BEGIN { while ((getline l < mf) > 0) { sub(/#.*/, "", l); gsub(/^[ \t]+|[ \t]+$/, "", l); if (l) mute[l] = 1 } }
'

human() {
    awk -v n="$1" 'BEGIN {
        if (n >= 1000000) printf "%.1fM", n / 1000000
        else if (n >= 10000) printf "%dk", n / 1000
        else printf "%d", n
    }'
}

if command -v journalctl >/dev/null 2>&1; then
    BACKEND=journal
elif [ "$(uname -s 2>/dev/null)" = "Darwin" ] && command -v log >/dev/null 2>&1; then
    BACKEND=unified
else
    BACKEND=""
fi

ssh_unit() {
    command -v journalctl >/dev/null 2>&1 || return 1
    journalctl --quiet --no-pager -u ssh  --since "-24 hours" -n 1 >/dev/null 2>&1 && { echo ssh;  return 0; }
    journalctl --quiet --no-pager -u sshd --since "-24 hours" -n 1 >/dev/null 2>&1 && { echo sshd; return 0; }
    return 1
}

# systemd unit events from the last 2h: fail/restart per unit, first+last seen.
# No _PID=1/_COMM=systemd field match here — some journalds (the kiosk's volatile
# journal, pre-reboot config change) skip trusted fields for pid-1's own
# messages; the awk prefix match on the "systemd[1]:" tag is the portable bit.
unit_events() {
    journalctl --since "-2 hours" --no-pager -o short 2>/dev/null | awk -v mf="$MUTE" "$MUTE_AWK"'
        {
            line = $0
            if (!sub(/^.*systemd\[[0-9]+\]: /, "", line)) next
            unit = line
            if (!sub(/: .*/, "", unit)) next
            msg = line
            sub(/^[^:]*: /, "", msg)
            if (unit in mute) next
            if (msg ~ /^Failed with result/) {
                why = msg
                sub(/^Failed with result ./, "", why)
                sub("\047\\.$", "", why)
                if (!(unit in first) || $3 < first[unit]) first[unit] = $3
                last[unit] = $3
                reason[unit] = why
                fail[unit] = 1
            } else if (msg ~ /restart counter is at/) {
                match($0, /restart counter is at [0-9]+/)
                n = substr($0, RSTART + 22, RLENGTH - 22) + 0
                if (n > rst[unit]) rst[unit] = n
                last[unit] = $3
            }
        }
        END {
            for (u in fail) {
                print "fail\t" first[u] "\t" last[u] "\t" u "\t" reason[u] "\t" (rst[u] ? rst[u] : 0)
            }
            for (u in rst) {
                if (!(u in fail) && rst[u] >= 3)
                    print "loop\t" last[u] "\t" last[u] "\t" u "\tcrash loop\t" rst[u]
            }
        }'
}

# kernel events: OOM kills and disk I/O errors, per target
kernel_events() {
    journalctl -k --since "-2 hours" --no-pager -o short 2>/dev/null | awk -v mf="$MUTE" "$MUTE_AWK"'
        /Out of memory: Killed process/ {
            p = $0
            sub(/^.*Killed process [0-9]+ \(/, "", p)
            sub(/\).*$/, "", p)
            if (("kernel:" p) in mute) next
            oomn[p]++; oomt[p] = substr($3, 1, 5)
        }
        /I\/O error, dev / {
            match($0, /dev [a-z]+[0-9]*/)
            d = substr($0, RSTART + 4, RLENGTH - 4)
            if (("kernel:" d) in mute) next
            ion[d]++; iot[d] = substr($3, 1, 5)
        }
        END {
            for (p in oomn) print "oom\t" oomt[p] "\t" oomt[p] "\t" p "\tOOM\t" oomn[p]
            for (d in ion)  print "io\t" iot[d] "\t" iot[d] "\t" d "\tdisk I/O\t" ion[d]
        }'
}

# ssh brute-force: top attacker in the last hour, if persistent
ssh_events() {
    unit=$1
    now=$(date +%H:%M)
    journalctl -u "$unit" --since "-1 hours" --no-pager -o short 2>/dev/null \
        | grep -oE 'Failed [^ ]+ .+ from ([0-9]+\.){3}[0-9]+' \
        | grep -oE '([0-9]+\.){3}[0-9]+' \
        | sort | uniq -c | sort -rn | head -1 \
        | awk -v mf="$MUTE" -v now="$now" "$MUTE_AWK"'
            $1 >= 20 && !($2 in mute) { print "1\t" now "\tlive\t" $2 "\tssh: " $1 " failed logins (1h)" }'
}

# hum: every other error, grouped by source, last hour (both platforms)
hum_grouped() {
    if [ "$BACKEND" = journal ]; then
        journalctl -p err --since "-1 hours" --no-pager -o short 2>/dev/null
    else
        log show --last 1h --style compact --predicate 'messageType == error' 2>/dev/null
    fi | awk -v mf="$MUTE" -v backend="$BACKEND" "$MUTE_AWK"'
        /^[A-Z][a-z][a-z] [ 0-9][0-9] [0-9][0-9]:[0-9][0-9]:[0-9][0-9] / {
            src = $5
            sub(/\[[0-9]+\]:$/, "", src)
            sub(/:$/, "", src)
            if (src == "kernel") src = "kernel:" $6
            n[src]++
            next
        }
        /^[0-9][0-9][0-9][0-9]-[0-9][0-9]-[0-9][0-9] [0-9][0-9]:[0-9][0-9]/ {
            src = $4
            sub(/\[.*$/, "", src)
            n[src]++
        }
        END { for (s in n) printf "%s\t%s\t%d\n", (s in mute ? "m" : "a"), s, n[s] }
    ' | sort -k3,3rn
}

LH="EVENTS "
sleep 1
lastW=0; lastROWS=0
while :; do
    printf '\033[H'
    W=$(tput cols 2>/dev/null || echo 80)
    ROWS=$(tput lines 2>/dev/null || echo 24)
    if [ "$W" != "$lastW" ] || [ "$ROWS" != "$lastROWS" ]; then
        printf '\033[H\033[2J'
        lastW=$W; lastROWS=$ROWS
    fi

    # ---- collect events (sev, time, state, kind, text) ----
    # records are PRINTED inside a subshell capture — string-appending via
    # $ev="$ev$(printf '...\n')" glues records together because command
    # substitution strips the trailing newline of each one
    ev="$(
    if [ "$BACKEND" = journal ]; then
        # systemd units
        while IFS="$(printf '\t')" read -r kind first last unit why rst; do
            [ -z "${unit:-}" ] && continue
            res=$(systemctl show "$unit" -p Result --value 2>/dev/null)
            load=$(systemctl show "$unit" -p LoadState --value 2>/dev/null)
            mins=$(( ($(date +%s) - $(date -d "$first" +%s 2>/dev/null || date +%s)) / 60 ))
            loop=""
            [ "${rst:-0}" -ge 3 ] && loop=", restarted ${rst}×"
            if [ "$load" = "not-found" ]; then
                printf '3\t%s\tfix\t%s\tfailed (%s)%s — unit gone\n' "$last" "$unit" "$why" "$loop"
            elif [ "$kind" = loop ] || { [ "$res" != "failed" ] && [ "${rst:-0}" -ge 3 ]; }; then
                printf '2\t%s\tloop\t%s\tcrash-looping, %s restarts\n' "$last" "$unit" "${rst:-?}"
            elif [ "$res" = "failed" ]; then
                printf '1\t%s\tdown\t%s\tfailed (%s)%s — down %dm\n' "$last" "$unit" "$why" "$loop" "$mins"
            else
                printf '3\t%s\tfix\t%s\tfailed (%s)%s — recovered in %dm\n' "$last" "$unit" "$why" "$loop" "$mins"
            fi
        done <<EOF2
$(unit_events)
EOF2
        # kernel: OOM kills, disk I/O errors
        while IFS="$(printf '\t')" read -r kind tlast _ target _ cnt; do
            [ -z "${target:-}" ] && continue
            if [ "$kind" = oom ]; then
                printf '1\t%s\tlive\t%s\tOOM killed, ×%s\n' "$tlast" "$target" "$cnt"
            elif [ "$kind" = io ]; then
                printf '1\t%s\tlive\t%s\tdisk I/O errors, ×%s\n' "$tlast" "$target" "$cnt"
            fi
        done <<EOF2
$(kernel_events)
EOF2
        # ssh
        u=$(ssh_unit || true)
        [ -n "$u" ] && ssh_events "$u"
    fi
    )"

    # ---- hum ----
    a_total=0; a_srcs=0; m_total=0
    top_a=""; top_a_n=0; top_m=""; top_m_n=0
    while IFS="$(printf '\t')" read -r flag src cnt; do
        [ -z "${src:-}" ] && continue
        if [ "$flag" = "m" ]; then
            m_total=$((m_total + cnt))
            if [ "$cnt" -gt "$top_m_n" ]; then top_m=$src; top_m_n=$cnt; fi
        else
            a_total=$((a_total + cnt)); a_srcs=$((a_srcs + 1))
            if [ "$cnt" -gt "$top_a_n" ]; then top_a=$src; top_a_n=$cnt; fi
        fi
    done <<EOF
$(hum_grouped)
EOF

    # ---- render ----
    total=0; nfix=0; nopen=0
    if [ -n "$ev" ]; then
        total=$(printf '%s\n' "$ev" | awk -F'\t' 'NF >= 4' | grep -c . || true)
        nfix=$(printf '%s\n' "$ev" | awk -F'\t' '$1 == 3' | grep -c . || true)
        nopen=$((total - nfix))
    fi

    printf '%b%s%b' "$C" "$LH" "$OFF"
    if [ "${nopen:-0}" -gt 0 ]; then
        printf '%b%s open · %s fixed · 2h window%b' "$R" "$nopen" "$nfix" "$OFF"
    elif [ "${nfix:-0}" -gt 0 ]; then
        printf '%b%s fixed · all quiet · 2h window%b' "$G" "$nfix" "$OFF"
    elif [ "$BACKEND" = unified ]; then
        printf '%bmacOS hum only — event rules need a linux journal%b' "$D" "$OFF"
    elif [ -z "$BACKEND" ]; then
        printf '%bn/a — needs a systemd journal (most linux) or macOS log(1); openrc/musl hosts have neither%b' "$D" "$OFF"
    else
        printf '%ball quiet · 2h window%b' "$G" "$OFF"
    fi
    printf '\033[K\n'

    maxrows=$((ROWS - 3))
    [ "$maxrows" -lt 1 ] && maxrows=1
    if [ "$total" -gt 0 ]; then
        printf '%s\n' "$ev" | sort -t"$(printf '\t')" -k1,1n -k2,2r | head -"$maxrows" \
        | while IFS="$(printf '\t')" read -r _ tm state kind text; do
            [ -z "${text:-}" ] && continue
            case "$state" in
                down) mark="× down"; g="$R" ;;
                live) mark="⚠ live"; g="$R" ;;
                fix)  mark="✓ fix "; g="$G" ;;
                *)    mark="↻ loop"; g="$Y" ;;
            esac
            # prefix " HH:MM  " + %-7s mark + space ≈ 20 cols; text gets the rest
            printf ' %b%s%b  %b%-7s%b %b%s%b\n' \
                "$D" "$tm" "$OFF" "$g" "$mark" "$OFF" "$g" "$(printf '%.*s' $((W - 20)) "$kind — $text")" "$OFF"
        done
        if [ "$total" -gt "$maxrows" ]; then
            printf ' %b… +%d more\033[K\n' "$D" "$((total - maxrows))"
        fi
    fi

    # hum footer
    printf ' %b%-7s%b' "$D" "hum" "$OFF"
    if [ -n "$top_a" ]; then
        printf '%b%s ×%s%b' "$D" "$top_a" "$(human "$top_a_n")" "$OFF"
        [ "$a_srcs" -gt 1 ] && printf ' %b· +%s src%b' "$D" "$((a_srcs - 1))" "$OFF"
    else
        printf '%bno error sources this hour%b' "$D" "$OFF"
    fi
    if [ "$m_total" -gt 0 ]; then
        printf ' %b· muted %s ×%s%b' "$D" "$top_m" "$(human "$top_m_n")" "$OFF"
    fi
    printf '\033[K'

    # ---- chip: name the worst unresolved event ----
    worst=""
    if [ -n "$ev" ]; then
        worst=$(printf '%s\n' "$ev" | sort -t"$(printf '\t')" -k1,1n -k2,2r \
            | awk -F'\t' '$1 == 1 { print $4 " " $5; exit }')
    fi
    if [ -n "$worst" ]; then
        "$EXE" chip events red 1 "⚠$(printf '%.*s' 24 "$worst")" 2>/dev/null
    elif printf '%s\n' "$ev" | awk -F'\t' '$1 == 2' | grep -q .; then
        loop=$(printf '%s\n' "$ev" | awk -F'\t' '$1 == 2' | head -1 | cut -f4)
        "$EXE" chip events orange 2 "⚠$(printf '%.*s' 24 "$loop") ×loop" 2>/dev/null
    elif [ "$a_total" -gt 20 ] && [ -n "$top_a" ]; then
        short=$(printf '%s' "$top_a" | sed 's/^kernel://' | cut -c1-14)
        "$EXE" chip events orange 2 "⚠${short} ×$(human "$top_a_n")" 2>/dev/null
    else
        "$EXE" chip events clear 2>/dev/null
    fi

    sleep 30
done
