#!/bin/sh
# lint-scripts — syntax + shellcheck gate for every screen script.
# Shebangs decide the dialect: #!/bin/sh scripts are checked as POSIX sh
# (that is where dash/bashisms bite), bash scripts as bash. shellcheck is
# optional locally (skipped with a notice) but required in CI.
set -u
cd "$(dirname "$0")/.." || exit 1

FAIL=0
for f in screens/*/*.sh tests/*.sh; do
    [ -e "$f" ] || continue
    # syntax check with the right interpreter
    case $(head -1 "$f") in
        *bash) bash -n "$f" || FAIL=$((FAIL + 1)) ;;
        *)     sh -n "$f" || FAIL=$((FAIL + 1)) ;;
    esac
    if command -v shellcheck >/dev/null 2>&1; then
        shellcheck -S warning "$f" || FAIL=$((FAIL + 1))
    elif [ -n "${CI:-}" ]; then
        echo "lint-scripts: shellcheck required in CI but not found" >&2
        FAIL=$((FAIL + 1))
    fi
done

[ "$FAIL" = 0 ] && exit 0
echo "lint-scripts: $FAIL file(s) failed" >&2
exit 1
