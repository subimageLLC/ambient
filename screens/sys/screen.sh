#!/bin/sh
if command -v btop >/dev/null 2>&1; then
    exec btop
elif command -v htop >/dev/null 2>&1; then
    exec htop
else
    exec top
fi
