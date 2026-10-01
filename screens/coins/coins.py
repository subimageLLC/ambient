#!/usr/bin/env python3
import curses
import json
import locale
import os
import time
import urllib.error
import urllib.request

try:
    locale.setlocale(locale.LC_ALL, "C.UTF-8")
except locale.Error:
    pass

DEFAULT_IDS = [
    ("bitcoin", "BTC"),
    ("ethereum", "ETH"),
    ("solana", "SOL"),
]
CATALOG = [
    ("majors", "bitcoin", "BTC", "Bitcoin"),
    ("majors", "ethereum", "ETH", "Ethereum"),
    ("majors", "solana", "SOL", "Solana"),
    ("majors", "binancecoin", "BNB", "BNB"),
    ("majors", "ripple", "XRP", "XRP"),
    ("majors", "dogecoin", "DOGE", "Dogecoin"),
    ("majors", "cardano", "ADA", "Cardano"),
    ("majors", "chainlink", "LINK", "Chainlink"),
    ("majors", "avalanche-2", "AVAX", "Avalanche"),
    ("majors", "polkadot", "DOT", "Polkadot"),
    ("majors", "litecoin", "LTC", "Litecoin"),
    ("majors", "near", "NEAR", "NEAR Protocol"),
    ("majors", "aptos", "APT", "Aptos"),
    ("majors", "sui", "SUI", "Sui"),
    ("majors", "the-open-network", "TON", "Toncoin"),
    ("majors", "tron", "TRX", "TRON"),
    ("majors", "stellar", "XLM", "Stellar"),
    ("majors", "cosmos", "ATOM", "Cosmos Hub"),
    ("majors", "internet-computer", "ICP", "Internet Computer"),
    ("majors", "hedera-hashgraph", "HBAR", "Hedera"),
    ("majors", "filecoin", "FIL", "Filecoin"),
    ("majors", "arweave", "AR", "Arweave"),
    ("majors", "render-token", "RNDR", "Render"),
    ("majors", "the-graph", "GRT", "The Graph"),
    ("ethereum", "uniswap", "UNI", "Uniswap"),
    ("ethereum", "aave", "AAVE", "Aave"),
    ("ethereum", "maker", "MKR", "Maker"),
    ("ethereum", "lido-dao", "LDO", "Lido DAO"),
    ("ethereum", "curve-dao-token", "CRV", "Curve DAO"),
    ("ethereum", "rocket-pool", "RPL", "Rocket Pool"),
    ("ethereum", "arbitrum", "ARB", "Arbitrum"),
    ("ethereum", "optimism", "OP", "Optimism"),
    ("ethereum", "polygon-ecosystem-token", "POL", "Polygon"),
    ("ethereum", "worldcoin-wld", "WLD", "Worldcoin"),
    ("ethereum", "eigenlayer", "EIGEN", "EigenLayer"),
    ("ethereum", "pepe", "PEPE", "Pepe"),
    ("ethereum", "shiba-inu", "SHIB", "Shiba Inu"),
    ("ethereum", "venice-token", "VVV", "Venice Token"),
    ("base", "aerodrome-finance", "AERO", "Aerodrome"),
    ("base", "degen-base", "DEGEN", "Degen"),
    ("base", "based-brett", "BRETT", "Brett"),
    ("base", "virtual-protocol", "VIRTUAL", "Virtual Protocol"),
    ("solana", "jupiter-exchange-solana", "JUP", "Jupiter"),
    ("solana", "jito-governance-token", "JTO", "Jito"),
    ("solana", "pyth-network", "PYTH", "Pyth Network"),
    ("solana", "raydium", "RAY", "Raydium"),
    ("solana", "dogwifcoin", "WIF", "dogwifcoin"),
    ("solana", "bonk", "BONK", "Bonk"),
    ("hyperliquid", "hyperliquid", "HYPE", "Hyperliquid"),
    ("privacy", "monero", "XMR", "Monero"),
    ("privacy", "zcash", "ZEC", "Zcash"),
]
CATALOG_SYM = {cid: sym for _, cid, sym, _ in CATALOG}
CONFIG = os.path.join(os.environ.get("AMBIENT_CONFIG_DIR", os.path.expanduser("~/.config/ambient")), "screens", "coins.conf")


def load_coins():
    ids = []
    try:
        with open(CONFIG) as f:
            for line in f:
                line = line.strip()
                if not line or line.startswith("#"):
                    continue
                parts = line.split()
                if len(parts) == 2:
                    ids.append((parts[0], parts[1]))
    except FileNotFoundError:
        pass
    return ids or DEFAULT_IDS


def save_coins(ids):
    os.makedirs(os.path.dirname(CONFIG), exist_ok=True)
    tmp = CONFIG + ".tmp"
    with open(tmp, "w") as f:
        f.write("# ambient coins — one per line: <coingecko-id> <SYMBOL>\n")
        for cid, sym in ids:
            f.write(f"{cid} {sym}\n")
    os.replace(tmp, CONFIG)


def catalog_view(flt):
    rows = []
    q = flt.lower()
    last_net = None
    for net, cid, sym, name in CATALOG:
        if q and q not in cid and q not in sym.lower() and q not in name.lower() and q not in net:
            continue
        if not q and net != last_net:
            rows.append(("header", net))
            last_net = net
        rows.append(("coin", net, cid, sym, name))
    return rows


def commit_selection(chosen):
    ids = []
    for _, cid, _, _ in CATALOG:
        if cid in chosen:
            ids.append((cid, CATALOG_SYM[cid]))
    for cid, sym in load_coins():
        if cid in chosen and all(cid != i for i, _ in ids):
            ids.append((cid, sym))
    if not ids:
        ids = DEFAULT_IDS
    save_coins(ids)
    return ids


def portfolio_editor(stdscr, chosen):
    flt = ""
    cur = 0
    base = 0
    while True:
        rows = catalog_view(flt)
        coin_idx = [i for i, r in enumerate(rows) if r[0] == "coin"]
        if coin_idx and cur >= len(coin_idx):
            cur = len(coin_idx) - 1
        h, w = stdscr.getmaxyx()
        body = max(h - 3, 1)
        disp = coin_idx[cur] if coin_idx else 0
        if disp < base:
            base = disp
        if disp >= base + body:
            base = disp - body + 1
        stdscr.erase()
        stdscr.addstr(0, 1, " pick your coins ", curses.color_pair(1) | curses.A_BOLD | curses.A_REVERSE)
        if flt:
            stdscr.addstr(0, 20, f" filter: {flt}", curses.color_pair(4))
        y = 1
        for i in range(base, min(len(rows), base + body)):
            r = rows[i]
            if r[0] == "header":
                stdscr.addstr(y, 2, f"── {r[1]} ", curses.color_pair(1))
            else:
                _, net, cid, sym, name = r
                mark = "●" if cid in chosen else "○"
                iscur = coin_idx and i == coin_idx[cur]
                col = curses.color_pair(2 if cid in chosen else 5) | (curses.A_BOLD if iscur else 0)
                line = f"{mark} {sym:<6} {name[:26]:<26}"
                try:
                    stdscr.addstr(y, 2, line, col | (curses.A_REVERSE if iscur else 0))
                    stdscr.addstr(y, 2 + len(line) + 1, net, curses.color_pair(4) | (curses.A_REVERSE if iscur else 0))
                except curses.error:
                    pass
            y += 1
        foot = " type to filter · space pick · enter save · esc cancel "
        stdscr.addstr(h - 1, 1, foot[: w - 10], curses.color_pair(4))
        try:
            stdscr.addstr(h - 1, w - 9, f"{len(chosen)} picked", curses.color_pair(4))
        except curses.error:
            pass
        stdscr.refresh()
        k = stdscr.getch()
        if k == 27:
            return None
        elif k in (curses.KEY_UP, ord("k")) and coin_idx:
            cur = (cur - 1) % len(coin_idx)
        elif k in (curses.KEY_DOWN, ord("j")) and coin_idx:
            cur = (cur + 1) % len(coin_idx)
        elif k == ord(" ") and coin_idx:
            cid = rows[coin_idx[cur]][2]
            if cid in chosen:
                chosen.discard(cid)
            else:
                chosen.add(cid)
        elif k in (10, curses.KEY_ENTER):
            return chosen
        elif k in (8, 127, curses.KEY_BACKSPACE):
            flt = flt[:-1]
        elif 32 <= k < 127:
            flt += chr(k).lower()

CACHE_DIR = os.environ.get("AMBIENT_CACHE_DIR", os.path.expanduser("~/.cache/ambient"))
MARKETS_CACHE = os.path.join(CACHE_DIR, "screen-coins.json")
CHART_CACHE = os.path.join(CACHE_DIR, "screen-coins-chart.json")
LOG = os.path.join(CACHE_DIR, "screen-coins.log")
EIGHTHS = " ▁▂▃▄▅▆▇█"
BRAILLE_BITS = [[1, 2, 4, 8], [16, 32, 64, 128]]
RETRY_AT = {}
LAST_NET = [0.0]
MIN_CHART_GAP = 6


def logfail(msg):
    try:
        if os.path.exists(LOG) and os.path.getsize(LOG) > 1_000_000:
            os.rename(LOG, LOG + ".old")
        with open(LOG, "a") as f:
            f.write(f"{time.strftime('%Y-%m-%d %H:%M:%S')} {msg}\n")
    except Exception:
        pass


def fetch(url, cache, maxage, min_gap=0):
    try:
        if os.path.exists(cache) and time.time() - os.path.getmtime(cache) < maxage:
            with open(cache) as f:
                return json.load(f)
    except Exception:
        pass
    now = time.time()
    if RETRY_AT.get(url, 0) > now or (min_gap and now - LAST_NET[0] < min_gap):
        pass
    else:
        LAST_NET[0] = now
        try:
            req = urllib.request.Request(url, headers={"User-Agent": "ambient-kiosk"})
            with urllib.request.urlopen(req, timeout=10) as r:
                data = json.load(r)
            os.makedirs(CACHE_DIR, exist_ok=True)
            with open(cache, "w") as f:
                json.dump(data, f)
            RETRY_AT.pop(url, None)
            return data
        except urllib.error.HTTPError as e:
            wait = 30 if e.code == 429 else 10
            try:
                wait = max(wait, int(e.headers.get("Retry-After") or 0))
            except (TypeError, ValueError):
                pass
            RETRY_AT[url] = now + wait
            logfail(f"HTTP {e.code} — next try in {wait}s — {url}")
        except Exception as e:
            RETRY_AT[url] = now + 10
            logfail(f"{type(e).__name__}: {e} — {url}")
    try:
        with open(cache) as f:
            return json.load(f)
    except Exception:
        return None


def fmt(p):
    if p is None:
        return "?"
    if p >= 1000:
        return f"{p:,.0f}"
    if p >= 1:
        return f"{p:,.2f}"
    return f"{p:.4f}"


def compact(v):
    for div, suf in ((1e12, "T"), (1e9, "B"), (1e6, "M"), (1e3, "K")):
        if v >= div:
            return f"{v / div:.1f}{suf}"
    return f"{v:.0f}"


def chg(v):
    if v is None:
        return "?", 0
    return f"{v:+.2f}%", 1 if v >= 0 else -1


def bucket_max(vals, n):
    if not vals:
        return []
    if len(vals) <= n:
        return vals
    out = []
    size = len(vals) / n
    for i in range(n):
        seg = vals[int(i * size):max(int((i + 1) * size), int(i * size) + 1)]
        out.append(max(seg))
    return out


def vol_grid(vals, height, width):
    vals = bucket_max(vals, width)
    if not vals:
        return [" " * width for _ in range(height)]
    lo, hi = min(vals), max(vals)
    rng = (hi - lo) or 1
    rows = [[] for _ in range(height)]
    for v in vals:
        e = int(((v - lo) / rng) * (height * 8 - 1))
        full, part = divmod(e, 8)
        top = height - 1 - full
        for r in range(height):
            if r < top:
                rows[r].append(" ")
            elif r == top:
                rows[r].append(EIGHTHS[part])
            else:
                rows[r].append("▒")
    return ["".join(r) for r in rows]


def line_grid(vals, height, width):
    W, H = width * 2, height * 4
    canvas = [[0] * width for _ in range(height)]
    if len(vals) < 2:
        return [" " * width for _ in range(height)], None, None, None
    lo, hi = min(vals), max(vals)
    rng = (hi - lo) or 1
    pts = []
    n = len(vals)
    for i, v in enumerate(vals):
        x = round(i * (W - 1) / (n - 1))
        y = round((1 - (v - lo) / rng) * (H - 1))
        pts.append((x, y))

    def setdot(x, y):
        if 0 <= x < W and 0 <= y < H:
            canvas[y // 4][x // 2] |= BRAILLE_BITS[x % 2][y % 4]

    for (x0, y0), (x1, y1) in zip(pts, pts[1:]):
        if x1 == x0:
            for y in range(min(y0, y1), max(y0, y1) + 1):
                setdot(x0, y)
        else:
            x = x0
            while x != x1:
                y = round(y0 + (x - x0) * (y1 - y0) / (x1 - x0))
                setdot(x, y)
                x += 1 if x1 > x0 else -1
            y = y0
            while y != y1:
                x = round(x0 + (y - y0) * (x1 - x0) / (y1 - y0))
                setdot(x, y)
                y += 1 if y1 > y0 else -1
            setdot(x1, y1)

    rows = ["".join(chr(0x2800 + c) for c in r) for r in canvas]
    last_row = int(pts[-1][1] // 4)
    return rows, lo, hi, last_row


def chart_data(coin_id, days):
    url = (
        f"https://api.coingecko.com/api/v3/coins/{coin_id}/market_chart"
        f"?vs_currency=usd&days={days}"
    )
    return fetch(url, CHART_CACHE + f".{coin_id}.{days}", 300, min_gap=MIN_CHART_GAP)


def draw(stdscr):
    curses.curs_set(0)
    stdscr.timeout(500)
    curses.start_color()
    curses.use_default_colors()
    curses.init_pair(1, curses.COLOR_CYAN, -1)
    curses.init_pair(2, curses.COLOR_GREEN, -1)
    curses.init_pair(3, curses.COLOR_RED, -1)
    curses.init_pair(4, 8, -1)
    curses.init_pair(5, curses.COLOR_WHITE, -1)
    try:
        curses.init_pair(6, 237, -1)
        curses.init_pair(7, 240, -1)
    except curses.error:
        curses.init_pair(6, 8, -1)
        curses.init_pair(7, 8, -1)

    sel = 0
    tfs = [("D", 1), ("W", 7), ("M", 30), ("Y", 365)]
    tfi = 0
    fetched = 0

    if not os.path.exists(CONFIG):
        chosen = portfolio_editor(stdscr, {"bitcoin", "ethereum", "solana"})
        if chosen is None:
            chosen = {"bitcoin", "ethereum", "solana"}
        commit_selection(chosen)

    while True:
        IDS = load_coins()
        if sel >= len(IDS):
            sel = 0
        api_markets = (
            "https://api.coingecko.com/api/v3/coins/markets?vs_currency=usd&ids="
            + ",".join(i for i, _ in IDS)
            + "&price_change_percentage=24h,7d"
        )
        h, w = stdscr.getmaxyx()
        markets = fetch(api_markets, MARKETS_CACHE, 55)
        if markets:
            fetched = max(fetched, os.path.getmtime(MARKETS_CACHE))
        by_id = {c.get("id"): c for c in (markets or [])}

        coin_id, sym = IDS[sel]
        cd = chart_data(coin_id, tfs[tfi][1])
        prices = [p[1] for p in (cd or {}).get("prices", [])]
        vols = [v[1] for v in (cd or {}).get("total_volumes", [])]

        ph, vh = 11, 2
        cw = max(w - 14, 10)
        price_rows, lo, hi, last_row = line_grid(prices, ph, cw)
        vol_rows = vol_grid(vols, vh, cw)
        grid_rows = {int((1 - f) * (ph * 4 - 1) // 4) for f in (0.25, 0.5, 0.75)}

        stdscr.erase()
        stdscr.addstr(0, 0, f" {sym} ", curses.color_pair(1) | curses.A_BOLD | curses.A_REVERSE)
        c = by_id.get(coin_id)
        if c:
            s, d = chg(c.get("price_change_percentage_24h_in_currency"))
            arrow = "▲" if d > 0 else "▼"
            pair = curses.color_pair(2 if d > 0 else 3) | curses.A_BOLD
            stdscr.addstr(0, 6, f" ${fmt(c.get('current_price'))}", curses.color_pair(5) | curses.A_BOLD)
            stdscr.addstr(0, 6 + 12, f" {arrow} {s} 24h", pair)
        for i, (lab, _) in enumerate(tfs):
            attr = curses.A_REVERSE if i == tfi else curses.color_pair(4)
            x = w - 4 - (len(tfs) - i) * 3
            stdscr.addstr(0, x, f"[{lab}]", attr)

        y = 2
        for r, row in enumerate(price_rows):
            for x, ch in enumerate(row):
                if ch == "\u2800":
                    if r == last_row:
                        stdscr.addch(y + r, 1 + x, "╌", curses.color_pair(6))
                    elif r in grid_rows:
                        stdscr.addch(y + r, 1 + x, "·", curses.color_pair(6))
                else:
                    stdscr.addch(y + r, 1 + x, ch, curses.color_pair(1))
        if hi is None:
            stdscr.addstr(y + ph // 2, 3, "no chart data — retrying (rate-limited?)", curses.color_pair(4))
        else:
            stdscr.addstr(y, 2 + cw + 1, f"${fmt(hi)}", curses.color_pair(4))
            stdscr.addstr(y + ph // 2, 2 + cw + 1, f"${fmt((hi + lo) / 2)}", curses.color_pair(4))
            stdscr.addstr(y + ph - 1, 2 + cw + 1, f"${fmt(lo)}", curses.color_pair(4))
        y += ph
        stdscr.addstr(y, 1, "─" * (cw + 2), curses.color_pair(4))
        y += 1
        for r, row in enumerate(vol_rows):
            stdscr.addstr(y + r, 1, row, curses.color_pair(7))
        if vols:
            stdscr.addstr(y, 2 + cw + 1, compact(max(vols)), curses.color_pair(4))
        y += vh + 1

        for i, (cid, s) in enumerate(IDS):
            c = by_id.get(cid)
            attr = curses.A_REVERSE | curses.color_pair(1) | curses.A_BOLD if i == sel else curses.color_pair(5)
            stdscr.addstr(y + i, 1, f" {s:<6}", attr)
            if c:
                stdscr.addstr(y + i, 9, f"${fmt(c.get('current_price')):>12}", curses.color_pair(5) | (curses.A_BOLD if i == sel else 0))
                s24, d24 = chg(c.get("price_change_percentage_24h_in_currency"))
                s7, d7 = chg(c.get("price_change_percentage_7d_in_currency"))
                stdscr.addstr(y + i, 24, f"{'▲' if d24 > 0 else '▼'} {s24:>8}", curses.color_pair(2 if d24 > 0 else 3))
                stdscr.addstr(y + i, 37, f"{'▲' if d7 > 0 else '▼'} {s7:>8} 7d", curses.color_pair(2 if d7 > 0 else 3))

        age = int(time.time() - os.path.getmtime(MARKETS_CACHE)) if os.path.exists(MARKETS_CACHE) else 9999
        stale = " STALE" if age > 300 else ""
        t = time.strftime("%H:%M", time.localtime(os.path.getmtime(MARKETS_CACHE))) if os.path.exists(MARKETS_CACHE) else "--:--"
        foot = f" ↑↓ select · a pick coins · x remove · tab timeframe · r refresh · q quit "
        stdscr.addstr(h - 1, 1, foot[: w - 2], curses.color_pair(4))
        info = f"coingecko {t}{stale}"
        try:
            stdscr.addstr(h - 1, max(1, w - len(info) - 1), info, curses.color_pair(4))
        except curses.error:
            pass
        stdscr.refresh()

        k = stdscr.getch()
        if k in (ord("q"), ord("Q")):
            break
        elif k in (curses.KEY_UP, ord("k")):
            sel = (sel - 1) % len(IDS)
        elif k in (curses.KEY_DOWN, ord("j")):
            sel = (sel + 1) % len(IDS)
        elif k in (ord("a"), ord("A")):
            chosen = {cid for cid, _ in load_coins()}
            res = portfolio_editor(stdscr, chosen)
            if res is not None:
                commit_selection(res)
            sel = 0
        elif k in (ord("x"), ord("X")) and len(IDS) > 1:
            keep = [e for i, e in enumerate(IDS) if i != sel]
            save_coins(keep)
            if sel >= len(keep):
                sel = len(keep) - 1
        elif k in (9, ord("t"), ord("]"), ord("[")):
            tfi = (tfi + 1) % len(tfs)
        elif k in (ord("d"), ord("D")):
            tfi = 0
        elif k in (ord("w"), ord("W")):
            tfi = 1
        elif k in (ord("m"), ord("M")):
            tfi = 2
        elif k in (ord("y"), ord("Y")):
            tfi = 3
        elif k in (ord("r"), ord("R")):
            try:
                os.unlink(MARKETS_CACHE)
            except FileNotFoundError:
                pass
            os.system(f"rm -f {CHART_CACHE}.*")


def main():
    os.makedirs(CACHE_DIR, exist_ok=True)
    curses.wrapper(draw)


if __name__ == "__main__":
    main()
