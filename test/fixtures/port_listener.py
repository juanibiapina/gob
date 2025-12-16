#!/usr/bin/env python3
"""
Port listener for testing gob port tracking.
Usage: port_listener.py <port1> [port2] [port3] ...

- First port: listened on by main process
- Additional ports: each spawned as a child process
"""

import os
import signal
import socket
import subprocess
import sys


def listen_on_port(port):
    """Listen on a TCP port and block forever."""
    sock = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
    sock.setsockopt(socket.SOL_SOCKET, socket.SO_REUSEADDR, 1)
    sock.bind(("0.0.0.0", port))
    sock.listen(1)
    print(f"PID {os.getpid()} listening on port {port}", flush=True)
    while True:
        try:
            conn, _ = sock.accept()
            conn.close()
        except:
            break


def main():
    if len(sys.argv) < 2:
        print("usage: port_listener.py <port> [port...]", file=sys.stderr)
        sys.exit(1)

    ports = [int(p) for p in sys.argv[1:]]
    children = []

    # Spawn children for ports after the first
    for port in ports[1:]:
        child = subprocess.Popen([sys.executable, __file__, str(port)])
        children.append(child)

    # Handle shutdown - kill children
    def shutdown(sig, frame):
        for child in children:
            child.terminate()
        sys.exit(0)

    signal.signal(signal.SIGTERM, shutdown)
    signal.signal(signal.SIGINT, shutdown)

    # Listen on first port in main process
    listen_on_port(ports[0])


if __name__ == "__main__":
    main()
