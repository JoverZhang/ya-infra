"""Load gateway declarations and resolve backend modules without cloud access."""
from dataclasses import dataclass
import json
from pathlib import Path
import re
import subprocess
import sys
import tomllib
from urllib.parse import urlsplit
import uuid

ROOT = Path(__file__).resolve().parents[1]
IDENTIFIER = r"[a-z][a-z0-9]*(?:-[a-z0-9]+)*"
DNS_NAME = r"(?:[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?\.)+[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?"


@dataclass(frozen=True)
class Route:
    name: str
    gateway: str
    hostname: str
    backend: str


def read_toml(path):
    with path.open("rb") as f:
        return tomllib.load(f)


def public(root=ROOT):
    c = read_toml(root / "gateway/public.toml")
    if set(c) != {"provider", "zone", "account_id", "tunnel_id"} or not all(isinstance(x, str) for x in c.values()):
        raise ValueError("public.toml requires provider, zone, account_id and tunnel_id")
    if c["provider"] != "cloudflare-tunnel":
        raise ValueError("Only the existing Cloudflare Tunnel provider is supported")
    if not re.fullmatch(DNS_NAME, c["zone"]) or len(c["zone"]) > 253:
        raise ValueError("Invalid public gateway zone")
    if not re.fullmatch(r"[a-f0-9]{32}", c["account_id"]):
        raise ValueError("Invalid Cloudflare account ID")
    if str(uuid.UUID(c["tunnel_id"])) != c["tunnel_id"]:
        raise ValueError("Invalid Cloudflare tunnel ID")
    return c


def routes(root=ROOT):
    gateway = public(root)
    result = []
    seen = set()
    for path in sorted((root / "gateway/routes").glob("*.toml")):
        c = read_toml(path)
        if (not re.fullmatch(IDENTIFIER, path.stem) or set(c) != {"gateway", "hostname", "backend"}
                or not all(isinstance(x, str) for x in c.values())):
            raise ValueError(f"Invalid route declaration: {path.name}")
        if c["gateway"] != "public":
            raise ValueError(f"{path.stem}: unknown gateway")
        hostname = c["hostname"]
        if not re.fullmatch(DNS_NAME, hostname) or len(hostname) > 253:
            raise ValueError(f"{path.stem}: use an exact lowercase DNS hostname")
        if hostname != gateway["zone"] and not hostname.endswith("." + gateway["zone"]):
            raise ValueError(f"{path.stem}: hostname is outside the gateway zone")
        if hostname in seen:
            raise ValueError(f"Duplicate route hostname: {hostname}")
        if not re.fullmatch(IDENTIFIER + "/" + IDENTIFIER, c["backend"]):
            raise ValueError(f"{path.stem}: backend must be service/instance")
        seen.add(hostname)
        result.append(Route(path.stem, **c))
    return result


def select(name=None, root=ROOT):
    result = routes(root)
    if name:
        result = [r for r in result if r.name == name]
        if not result:
            raise ValueError(f"Unknown route: {name}")
    if not result:
        raise ValueError("No gateway routes declared")
    return result


def hostnames_for(backend, root=ROOT):
    return sorted(r.hostname for r in routes(root) if r.backend == backend)


def backend_command(route, command, *args, root=ROOT):
    service, instance = route.backend.split("/")
    module = root / "services" / service / "manage"
    if not module.is_file():
        raise ValueError(f"{route.name}: backend module does not exist")
    try:
        p = subprocess.run([sys.executable, str(module), command, instance, *args],
                           capture_output=True, text=True, timeout=180)
    except subprocess.TimeoutExpired:
        raise RuntimeError(f"{route.name}: backend {command} timed out") from None
    if p.returncode:
        raise RuntimeError(f"{route.name}: backend {command} failed: " + p.stderr.strip())
    return p.stdout


def endpoint(route, root=ROOT):
    c = json.loads(backend_command(route, "endpoint", root=root))
    url = urlsplit(c["url"])
    if (url.scheme not in ("http", "https") or url.hostname not in ("127.0.0.1", "localhost", "::1")
            or not url.port or url.username or url.password or url.query or url.fragment or url.path not in ("", "/")):
        raise ValueError(f"{route.name}: backend must export an HTTP(S) loopback endpoint")
    if not isinstance(c.get("health_path"), str) or not c["health_path"].startswith("/"):
        raise ValueError(f"{route.name}: backend must export a health_path")
    return c
