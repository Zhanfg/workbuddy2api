#!/usr/bin/env python3
"""Sample a freshly started workbuddy2api binary's idle RSS/PSS.

Uses a throwaway config, empty auths directory, no secrets and no real WorkBuddy
accounts. Only the process memory totals are printed; no HTTP payloads/logs.
The result is NOT a representative production workload.
"""
from __future__ import annotations

import argparse
import json
import os
from pathlib import Path
import socket
import statistics
import subprocess
import tempfile
import time
import urllib.error
import urllib.request


def _port() -> int:
    with socket.socket() as sock:
        sock.bind(("127.0.0.1", 0))
        return sock.getsockname()[1]


def _measure(pid: int, key: str, source: str) -> int | None:
    try:
        for line in Path(f"/proc/{pid}/{source}").read_text().splitlines():
            if line.startswith(key + ":"):
                return int(line.split()[1])
    except (OSError, PermissionError):
        return None
    return None


def main() -> int:
    p = argparse.ArgumentParser(description=__doc__)
    p.add_argument("binary", type=Path, help="prebuilt cmd/server binary")
    p.add_argument("--seconds", type=float, default=8.0)
    args = p.parse_args()
    if args.seconds < 1:
        p.error("--seconds must be >= 1")
    program = args.binary.resolve()
    if not program.is_file():
        p.error("binary not found")
    with tempfile.TemporaryDirectory(prefix="wb2api-idle-") as td:
        root = Path(td)
        (root / "auths").mkdir()
        (root / "data").mkdir()
        port = _port()
        # Fresh, non-sensitive config. Never use or print a production config.
        (root / "config.json").write_text(json.dumps({
            "listen": f"127.0.0.1:{port}",
            "auth_dir": "./auths",
            "state_file": "./data/state.json",
        }), encoding="utf-8")
        proc = subprocess.Popen(
            [str(program), "-config", str(root / "config.json")],
            cwd=root, stdin=subprocess.DEVNULL,
            stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL,
            env={**os.environ, "GOTOOLCHAIN": "local"},
        )
        try:
            deadline = time.monotonic() + 20
            while time.monotonic() < deadline:
                if proc.poll() is not None:
                    raise RuntimeError(f"Go process terminated at startup: exit {proc.returncode}")
                try:
                    with urllib.request.urlopen(
                        f"http://127.0.0.1:{port}/healthz", timeout=0.5
                    ):
                        break
                except urllib.error.HTTPError:
                    # Health can legitimately be 503 with an empty account pool.
                    break
                except (OSError, urllib.error.URLError):
                    time.sleep(0.2)
            else:
                raise TimeoutError("Go process did not bind the local listener")

            rss, pss, swap = [], [], []
            time.sleep(1)
            end = time.monotonic() + args.seconds
            while time.monotonic() < end:
                a = _measure(proc.pid, "VmRSS", "status")
                b = _measure(proc.pid, "Pss", "smaps_rollup")
                c = _measure(proc.pid, "VmSwap", "status")
                if a is not None:
                    rss.append(a)
                if b is not None:
                    pss.append(b)
                if c is not None:
                    swap.append(c)
                time.sleep(0.2)
            if not rss:
                raise RuntimeError("unable to sample /proc")
            print(f"binary={program.name} samples={len(rss)}")
            print(f"RSS_MiB min={min(rss)/1024:.2f} "
                  f"median={statistics.median(rss)/1024:.2f} max={max(rss)/1024:.2f}")
            if pss:
                print(f"PSS_MiB min={min(pss)/1024:.2f} "
                      f"median={statistics.median(pss)/1024:.2f} max={max(pss)/1024:.2f}")
            if swap:
                print(f"VmSwap_MiB max={max(swap)/1024:.2f}")
        finally:
            proc.terminate()
            try:
                proc.wait(timeout=5)
            except subprocess.TimeoutExpired:
                proc.kill()
                proc.wait()
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
