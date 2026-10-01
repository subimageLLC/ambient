#!/usr/bin/env python3
import curses
import locale
import os
import re
import subprocess
import time
from collections import deque

try:
    locale.setlocale(locale.LC_ALL, "C.UTF-8")
except locale.Error:
    pass

CONF_DIR = os.path.join(
    os.environ.get("AMBIENT_CONFIG_DIR", os.path.expanduser("~/.config/ambient")), "screens"
)
CONF = os.path.join(CONF_DIR, "net.conf")

BRAILLE_BITS = [[1, 2, 4, 8], [16, 32, 64, 128]]


def detect_gateway():
    for cmd in (["ip", "route"], ["route", "-n", "get", "default"]):
        try:
            out = subprocess.run(cmd, capture_output=True, text=True, timeout=5).stdout
            if cmd[0] == "ip":
                for line in out.splitlines():
                    if line.startswith("default"):
                        return line.split()[2]
            else:
                for line in out.splitlines():
                    if "gateway" in line:
                        return line.split()[1]
        except Exception:
            pass
    return None


def load_wan():
    try:
        with open(CONF) as f:
            for line in f:
                k, _, v = line.strip().partition("=")
                if k == "wan" and v:
                    return v
    except FileNotFoundError:
        pass
    return "1.1.1.1"


def save_wan(host):
    os.makedirs(CONF_DIR, exist_ok=True)
    tmp = CONF + ".tmp"
    with open(tmp, "w") as f:
        f.write(f"wan={host}\n")
    os.replace(tmp, CONF)


def ping(host):
    try:
        out = subprocess.run(
            ["ping", "-c", "1", host], capture_output=True, text=True, timeout=2
        ).stdout
        m = re.search(r"time=([\d.]+) ms", out)
        if m:
            return float(m.group(1))
    except Exception:
        pass
    return None


def line_grid(vals, height, width):
    W, H = width * 2, height * 4
    canvas = [[0] * width for _ in range(height)]
    if len(vals) < 2:
        return [" " * width for _ in range(height)], None, None
    flat = [v for v in vals if v is not None]
    if len(flat) < 2:
        return [" " * width for _ in range(height)], None, None
    lo, hi = min(flat), max(flat)
    rng = (hi - lo) or 1
    present = [i for i, v in enumerate(vals) if v is not None]
    val_map = {i: vals[i] for i in present}
    pts = []
    n = len(vals)
    for i in present:
        x = round(i * (W - 1) / (n - 1))
        y = round((1 - (vals[i] - lo) / rng) * (H - 1))
        pts.append((x, y))

    def setdot(x, y):
        if 0 <= x < W and 0 <= y < H:
            canvas[y // 4][x // 2] |= BRAILLE_BITS[x % 2][y % 4]

    for (x0, y0), (x1, y1) in zip(pts, pts[1:]):
        if x1 == x0:
            for y in range(min(y0, y1), max(y0, y1) + 1):
                setdot(x0, y)
            continue
        for x in range(x0, x1, 1 if x1 > x0 else -1):
            y = round(y0 + (x - x0) * (y1 - y0) / (x1 - x0))
            setdot(x, y)
        setdot(x1, y1)
    rows = ["".join(chr(0x2800 + c) for c in r) for r in canvas]
    return rows, lo, hi


def stats(hist):
    flat = [v for v in hist if v is not None]
    if not flat:
        return "no data"
    loss = 100 * (1 - len(flat) / len(hist))
    cur = flat[-1]
    avg = sum(flat[-20:]) / len(flat[-20:])
    peak = max(flat)
    return f"{cur:6.1f} ms   avg {avg:6.1f}   peak {peak:6.1f}   loss {loss:.0f}%"


def prompt(stdscr, y, label, default=""):
    curses.echo()
    curses.curs_set(1)
    stdscr.timeout(-1)
    stdscr.addstr(y, 2, " " * 74, curses.color_pair(4))
    stdscr.addstr(y, 2, f"{label} [{default}]: ", curses.color_pair(5))
    stdscr.refresh()
    buf = stdscr.getstr(y, 2 + len(label) + len(default) + 6, 40).decode("utf-8").strip()
    stdscr.timeout(700)
    curses.noecho()
    curses.curs_set(0)
    return buf


def draw(stdscr):
    curses.curs_set(0)
    stdscr.timeout(700)
    curses.start_color()
    curses.use_default_colors()
    curses.init_pair(1, curses.COLOR_CYAN, -1)
    curses.init_pair(2, curses.COLOR_GREEN, -1)
    curses.init_pair(3, curses.COLOR_RED, -1)
    curses.init_pair(4, 8, -1)
    curses.init_pair(5, curses.COLOR_WHITE, -1)

    wan = load_wan()
    gw = detect_gateway()
    targets = [("gateway", gw), ("wan", wan)]
    hist = {name: deque(maxlen=200) for name, _ in targets}

    while True:
        h, w = stdscr.getmaxyx()
        stdscr.erase()
        stdscr.addstr(0, 2, "network", curses.color_pair(1) | curses.A_BOLD)

        panel_h = (h - 4) // 2
        chart_h = max(panel_h - 3, 3)
        chart_w = max(w - 6, 10)

        for name, host in targets:
            hist[name].append(ping(host) if host else None)

        y = 2
        for name, host in targets:
            if not host:
                stdscr.addstr(y, 3, f"{name}: not detected", curses.color_pair(3))
                y += panel_h
                continue
            rows, lo, hi = line_grid(list(hist[name]), chart_h, chart_w)
            for i, row in enumerate(rows):
                stdscr.addstr(y + i, 3, row, curses.color_pair(1))
            stdscr.addstr(y, 5, f" {name} · {host} ", curses.color_pair(4))
            s = stats(hist[name])
            col = 2 if not hist[name][-1] is None else 3
            try:
                stdscr.addstr(y + chart_h, 3, f" {s} ", curses.color_pair(col) | curses.A_BOLD)
                if hi is not None:
                    stdscr.addstr(y, 5 + 14 + len(host) + 2, f" {hi:.0f}", curses.color_pair(4))
                    stdscr.addstr(y + chart_h - 1, 5 + 14 + len(host) + 2, f" {lo:.0f}", curses.color_pair(4))
            except curses.error:
                pass
            y += panel_h + 1

        try:
            stdscr.addstr(h - 1, 2, " s set wan target · q quit ", curses.color_pair(4))
        except curses.error:
            pass
        stdscr.refresh()

        k = stdscr.getch()
        if k in (ord("q"), ord("Q")):
            break
        elif k in (ord("s"), ord("S")):
            host = prompt(stdscr, h - 3, "wan host", wan)
            if host and ping(host) is not None:
                wan = host
                save_wan(host)
                hist["wan"].clear()
            elif host:
                stdscr.addstr(h - 3, 2, " " * 74, curses.color_pair(4))
                stdscr.addstr(h - 3, 2, f"no reply from {host}", curses.color_pair(3))
                stdscr.refresh()
                time.sleep(2)


def main():
    curses.wrapper(draw)


if __name__ == "__main__":
    main()
