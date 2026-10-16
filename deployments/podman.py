"""Shared rootless Podman operations; deployment policy stays in each module."""
import json
import os
from pathlib import Path
import subprocess
import time
import urllib.error
import urllib.request

ENGINE = ["podman", "--remote", "--url", f"unix:///run/user/{os.getuid()}/podman/podman.sock"]


def engine(*args, capture=True, check=True, input=None):
    return subprocess.run(ENGINE + list(args), text=True, input=input, capture_output=capture, check=check)


def inspect(run, kind, name, *, error=None):
    result = run(kind, "inspect", name, capture=True, check=False)
    if result.returncode:
        if run(kind, "exists", name, capture=True, check=False).returncode == 1:
            return None
        raise RuntimeError(error or result.stderr.strip() or f"Cannot inspect {kind} {name}")
    return json.loads(result.stdout)[0]


def require_owned(labels, expected, error):
    if not all(labels.get(key) == value for key, value in expected.items()):
        raise RuntimeError(error)


def label_args(labels):
    return [arg for pair in labels.items() for arg in ("--label", "=".join(pair))]


def private_directory(path):
    path.mkdir(parents=True, exist_ok=True, mode=0o700)
    path.chmod(0o700)


def wait_http_ready(url, timeout, error, *, status=200):
    deadline = time.monotonic() + timeout
    while time.monotonic() < deadline:
        try:
            with urllib.request.urlopen(url, timeout=2) as response:
                if response.status == status:
                    return
        except urllib.error.HTTPError as response:
            if response.code == status:
                return
        except (urllib.error.URLError, TimeoutError, ConnectionError):
            pass
        time.sleep(1)
    raise RuntimeError(error)


def source_text(manager):
    return Path(manager).read_text() + "\n" + Path(__file__).read_text()
