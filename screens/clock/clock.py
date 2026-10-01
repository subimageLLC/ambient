#!/usr/bin/env python3
import curses
import json
import locale
import os
import subprocess
import time
import urllib.request

try:
    locale.setlocale(locale.LC_ALL, "C.UTF-8")
except locale.Error:
    pass

CONF_DIR = os.path.join(
    os.environ.get("AMBIENT_CONFIG_DIR", os.path.expanduser("~/.config/ambient")), "screens"
)
CONF = os.path.join(CONF_DIR, "clock.conf")
AMB = os.environ.get("AMBIENT_EXE", "ambient")

DIGITS = {
    "0": ["███", "█ █", "█ █", "███"],
    "1": [" █ ", " █ ", " █ ", " █ "],
    "2": ["███", "  █", "███", "█  "],
    "3": ["███", "  █", "███", "  █"],
    "4": ["█ █", "█ █", "███", "  █"],
    "5": ["███", "█  ", "███", "  █"],
    "6": ["███", "█  ", "███", "█ █"],
    "7": ["███", "  █", "  █", "  █"],
    "8": ["███", "█ █", "███", "█ █"],
    "9": ["███", "█ █", "███", "  █"],
    ":": [" ", "█", " ", "█"],
}


def load_conf():
    conf = {"loc": "", "label": "", "format": os.environ.get("AMBIENT_CLOCK_FORMAT", "24h")}
    try:
        with open(CONF) as f:
            for line in f:
                k, _, v = line.strip().partition("=")
                if k in conf and v:
                    conf[k] = v
    except FileNotFoundError:
        pass
    return conf


def save_conf(conf):
    os.makedirs(CONF_DIR, exist_ok=True)
    tmp = CONF + ".tmp"
    with open(tmp, "w") as f:
        for k in ("loc", "label", "format"):
            f.write(f"{k}={conf[k]}\n")
    os.replace(tmp, CONF)


def big_time(text):
    rows = ["", "", "", ""]
    for ch in text:
        glyph = DIGITS.get(ch, ["   "] * 4)
        for r in range(4):
            rows[r] += " " + "".join(c * 2 for c in glyph[r]) + " "
    doubled = []
    for r in range(4):
        doubled.append(rows[r])
        doubled.append(rows[r])
    return doubled


def fetch_weather(loc):
    url = f"https://wttr.in/{loc}?format=j1"
    try:
        out = subprocess.run(
            [AMB, "fetch", "--ttl", "1800", "--name", "weather", url],
            capture_output=True, text=True, timeout=15,
        )
        return json.loads(out.stdout)
    except Exception:
        return None


def weather_lines(d):
    try:
        cur = d["current_condition"][0]
        desc = cur.get("weatherDesc", [{}])[0].get("value", "")
        lines = [f"Now      {cur['temp_C']}C  feels {cur['FeelsLikeC']}C  {desc}"]
        w = d.get("weather", [])
        if w:
            lines.append(f"Today    {w[0].get('mintempC', '?')}C / {w[0].get('maxtempC', '?')}C")
        if len(w) > 1:
            lines.append(f"Tomorrow {w[1].get('mintempC', '?')}C / {w[1].get('maxtempC', '?')}C")
        return lines
    except Exception:
        return ["weather unavailable"]


def prompt(stdscr, y, label):
    curses.echo()
    curses.curs_set(1)
    stdscr.timeout(-1)
    stdscr.addstr(y, 2, " " * 76, curses.color_pair(4))
    stdscr.addstr(y, 2, label, curses.color_pair(5))
    stdscr.refresh()
    buf = stdscr.getstr(y, 2 + len(label), 60).decode("utf-8").strip()
    stdscr.timeout(1000)
    curses.noecho()
    curses.curs_set(0)
    return buf


def set_location(stdscr, conf):
    h, w = stdscr.getmaxyx()
    loc = prompt(stdscr, h - 3, "location (zipcode/city/lat,lon): ")
    if not loc:
        return None
    enc = loc.replace(" ", "%20")
    stdscr.addstr(h - 3, 2, " " * 60, curses.color_pair(4))
    stdscr.addstr(h - 3, 2, "checking...", curses.color_pair(4))
    stdscr.refresh()
    try:
        req = urllib.request.Request(f"https://wttr.in/{enc}?format=j1", headers={"User-Agent": "ambient"})
        with urllib.request.urlopen(req, timeout=10) as r:
            d = json.load(r)
        cur = d["current_condition"][0]
    except Exception:
        stdscr.addstr(h - 3, 2, f'could not resolve "{loc}" — esc to cancel', curses.color_pair(3))
        stdscr.refresh()
        time.sleep(2)
        return None
    label = prompt(stdscr, h - 3, "label (optional): ")
    conf["loc"] = enc
    conf["label"] = label
    save_conf(conf)
    return cur


def draw(stdscr):
    curses.curs_set(0)
    stdscr.timeout(1000)
    curses.start_color()
    curses.use_default_colors()
    curses.init_pair(1, curses.COLOR_CYAN, -1)
    curses.init_pair(2, curses.COLOR_GREEN, -1)
    curses.init_pair(3, curses.COLOR_RED, -1)
    curses.init_pair(4, 8, -1)
    curses.init_pair(5, curses.COLOR_WHITE, -1)

    conf = load_conf()
    wx = None
    wx_at = 0
    blink = True

    while True:
        h, w = stdscr.getmaxyx()
        stdscr.erase()

        stdscr.addstr(0, 2, time.strftime("%a %b %d %Y"), curses.color_pair(4))
        if conf["format"] == "12h":
            t = time.strftime("%I:%M")
            ampm = time.strftime(" %p")
        else:
            t = time.strftime("%H:%M")
            ampm = ""
        rows = big_time(t)
        for i, row in enumerate(rows):
            stdscr.addstr(2 + i, 2, row, curses.color_pair(5) | curses.A_BOLD)
        if ampm and blink:
            stdscr.addstr(2, 2 + len(rows[0]) + 2, ampm, curses.color_pair(4))
        blink = not blink

        y = 11
        if conf["loc"]:
            if time.time() - wx_at > 60 or (wx is None and wx_at == 0):
                wx = fetch_weather(conf["loc"])
                wx_at = time.time()
            if wx:
                for line in weather_lines(wx):
                    stdscr.addstr(y, 3, line, curses.color_pair(5))
                    y += 1
                if conf["label"]:
                    stdscr.addstr(y, 3, f"({conf['label']})", curses.color_pair(4))
            else:
                stdscr.addstr(y, 3, "weather unreachable", curses.color_pair(4))
        else:
            stdscr.addstr(y, 3, "press s to set your location for weather", curses.color_pair(1) | curses.A_BOLD)
            y += 2
            stdscr.addstr(y, 3, "(zipcode, city name, or lat,lon — only the query reaches wttr.in)", curses.color_pair(4))

        try:
            stdscr.addstr(h - 1, 2, " s set location · q quit ", curses.color_pair(4))
        except curses.error:
            pass
        stdscr.refresh()

        k = stdscr.getch()
        if k in (ord("q"), ord("Q")):
            break
        elif k in (ord("s"), ord("S")):
            if set_location(stdscr, conf) is not None:
                wx = None
                wx_at = 0


def main():
    curses.wrapper(draw)


if __name__ == "__main__":
    main()
