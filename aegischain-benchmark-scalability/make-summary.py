#!/usr/bin/env python3
"""Build results/summary.csv from the raw logs of the scalability experiments.

Usage:  python3 make-summary.py <log-dir> > results/summary.csv

Reads <experiment>-<tag>-repN.log files (seed-*.log are ignored). From each log
it extracts the result line printed by the driver, for example:

  [level=1000000] CheckAccess   p50=447.5ms p95=563.9ms throughput=104.85 tx/s (ok=1950, errs=0, wall=18.59s)

Logs without a result line, or whose run failed completely, are NOT written to
the CSV; they are listed on stderr and the exit status is 1, so an incomplete or
invalid result set cannot be published by accident.
"""
import csv
import re
import sys
from pathlib import Path

LINE = re.compile(
    r"\[level=(?P<level>\d+)\]\s+(?P<op>\S+)\s+"
    r"p50=(?P<p50>\S+)\s+p95=(?P<p95>\S+)\s+"
    r"throughput=(?P<tp>[\d.]+)\s+tx/s\s+"
    r"\(ok=(?P<ok>\d+),\s*errs=(?P<errs>\d+),\s*wall=(?P<wall>\S+?)\)"
)
NAME = re.compile(r"^(?P<exp>[a-z]+)-(?P<tag>.+)-rep(?P<rep>\d+)\.log$")
UNIT_MS = {"ns": 1e-6, "µs": 1e-3, "us": 1e-3, "ms": 1.0, "s": 1e3, "m": 6e4, "h": 3.6e6}
PART = re.compile(r"(\d+(?:\.\d+)?)(ns|µs|us|ms|s|m|h)")


def to_ms(go_duration: str) -> float:
    """Convert a Go duration string (e.g. 1m2.5s, 447.5ms, 850µs) to milliseconds."""
    parts = PART.findall(go_duration)
    if not parts or "".join(n + u for n, u in parts) != go_duration:
        raise ValueError(f"unrecognized duration: {go_duration!r}")
    return sum(float(n) * UNIT_MS[u] for n, u in parts)


def main(argv):
    if len(argv) != 2:
        sys.exit(__doc__)
    log_dir = Path(argv[1])
    rows, bad = [], []
    for path in sorted(log_dir.glob("*-rep*.log")):
        m = NAME.match(path.name)
        if not m:
            continue
        result = None
        for line in path.read_text(errors="replace").splitlines():
            r = LINE.search(line)
            if r:
                result = r
        if result is None:
            bad.append(path.name)
            continue
        g = result.groupdict()
        rows.append({
            "experiment": m["exp"],
            "level": int(g["level"]),
            "rep": int(m["rep"]),
            "operation": g["op"],
            "p50_ms": round(to_ms(g["p50"]), 3),
            "p95_ms": round(to_ms(g["p95"]), 3),
            "throughput_tx_s": float(g["tp"]),
            "ok": int(g["ok"]),
            "errs": int(g["errs"]),
            "wall_s": round(to_ms(g["wall"]) / 1e3, 3),
        })
    rows.sort(key=lambda r: (r["experiment"], r["level"], r["rep"]))
    w = csv.DictWriter(sys.stdout, fieldnames=list(rows[0]) if rows else [])
    if rows:
        w.writeheader()
        w.writerows(rows)
    if bad:
        print(f"{len(bad)} log(s) without a valid result line (not in the CSV):", file=sys.stderr)
        for name in bad:
            print(f"  {name}", file=sys.stderr)
    return 1 if bad or any(r["errs"] for r in rows) else 0


if __name__ == "__main__":
    sys.exit(main(sys.argv))
