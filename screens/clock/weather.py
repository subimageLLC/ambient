import json
import sys

try:
    d = json.load(sys.stdin)
    cur = d["current_condition"][0]
    desc = cur.get("weatherDesc", [{}])[0].get("value", "")
    print(f'  Now      {cur["temp_C"]}C  feels {cur["FeelsLikeC"]}C  {desc}')
    w = d.get("weather", [])
    if w:
        print(f'  Today    {w[0].get("mintempC", "?")}C / {w[0].get("maxtempC", "?")}C')
    if len(w) > 1:
        print(f'  Tomorrow {w[1].get("mintempC", "?")}C / {w[1].get("maxtempC", "?")}C')
except Exception:
    print("  weather unavailable")
