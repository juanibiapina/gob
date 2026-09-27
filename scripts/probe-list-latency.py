#!/usr/bin/env python3
"""Measure gob list before, during, and after tmux regress without touching the user's daemon."""

import json
import os
import pathlib
import shutil
import socket
import subprocess
import sys
import tempfile
import time


def call(binary, *args, timeout=30):
    return subprocess.run([binary, *args], check=True, capture_output=True, text=True, timeout=timeout)


def measure(binary, count, interval=0):
    samples = []
    for _ in range(count):
        start = time.monotonic()
        call(binary, "list", "--json")
        samples.append((time.monotonic() - start) * 1000)
        time.sleep(interval)
    return samples


def report(label, values):
    sorted_values = sorted(values)
    p99 = sorted_values[max(0, int(len(values) * 0.99) - 1)]
    print(f"{label}: n={len(values)} p99={p99:.1f}ms max={max(values):.1f}ms", flush=True)
    return p99, max(values)


def main():
    binary = str(pathlib.Path(sys.argv[1] if len(sys.argv) > 1 else "dist/gob").resolve())
    tmux_dir = pathlib.Path(sys.argv[2] if len(sys.argv) > 2 else "/Users/juan.ibiapina/workspace/tmux/tmux")
    gmake = shutil.which("gmake")
    if not gmake or not (tmux_dir / "regress").is_dir():
        raise RuntimeError("gmake and the tmux regress directory are required")
    with tempfile.TemporaryDirectory(prefix="gob-latency-") as root:
        os.environ["XDG_RUNTIME_DIR"] = root + "/runtime"
        os.environ["XDG_STATE_HOME"] = root + "/state"
        os.makedirs(os.environ["XDG_RUNTIME_DIR"])
        os.makedirs(os.environ["XDG_STATE_HOME"])
        os.chdir(tmux_dir)
        job_id = None
        try:
            call(binary, "list", "--json")  # Start the isolated daemon before timing.
            report("before", measure(binary, 20))
            call(binary, "add", "--", gmake, "-C", "regress", "-j", "8")
            jobs = json.loads(call(binary, "list", "--json").stdout)
            job_id = jobs[0]["id"]
            p99, maximum = report("during", measure(binary, 120, 0.09))
            state = next(j["status"] for j in json.loads(call(binary, "list", "--json").stdout) if j["id"] == job_id)
            if state != "running":
                raise AssertionError(f"build ended before polling window completed: {state}")
            request = {"type": "stop_request", "payload": {"job_id": job_id}}
            sock = socket.socket(socket.AF_UNIX)
            sock.connect(os.environ["XDG_RUNTIME_DIR"] + "/gob/daemon.sock")
            start = time.monotonic()
            sock.sendall((json.dumps(request) + "\n").encode())
            ack = json.loads(sock.makefile().readline())
            ack_ms = (time.monotonic() - start) * 1000
            sock.close()
            if not ack["success"] or ack_ms > 250:
                raise AssertionError(f"stop acknowledgment took {ack_ms:.1f}ms: {ack}")
            print(f"stop acknowledged in {ack_ms:.1f}ms", flush=True)
            call(binary, "stop", job_id, timeout=25)
            report("after", measure(binary, 20))
            if p99 >= 100 or maximum >= 250:
                raise AssertionError(f"during list exceeded target: p99={p99:.1f}ms max={maximum:.1f}ms")
        finally:
            try:
                if job_id:
                    subprocess.run([binary, "stop", "--force", job_id], capture_output=True, timeout=25)
            finally:
                subprocess.run([binary, "shutdown"], capture_output=True, timeout=25)


if __name__ == "__main__":
    main()
