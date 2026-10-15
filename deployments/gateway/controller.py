"""Plan and apply one route while preserving unrelated Cloudflare configuration."""
from copy import deepcopy
from datetime import datetime, timezone
import json
import urllib.error
import urllib.request

from gateway import model


class Conflict(RuntimeError):
    pass


class State:
    def __init__(self, directory):
        self.directory = directory

    def read(self):
        p = self.directory / "routes.json"
        return json.loads(p.read_text()) if p.is_file() else {}

    def write(self, value):
        self.directory.mkdir(parents=True, exist_ok=True, mode=0o700)
        self.directory.chmod(0o700)
        p = self.directory / "routes.json"
        temporary = self.directory / "routes.json.new"
        temporary.write_text(json.dumps(value, indent=2) + "\n")
        temporary.chmod(0o600)
        temporary.replace(p)

    def snapshot(self, value):
        self.directory.mkdir(parents=True, exist_ok=True, mode=0o700)
        self.directory.chmod(0o700)
        p = self.directory / "before-last-apply.json"
        p.write_text(json.dumps(value, indent=2) + "\n")
        p.chmod(0o600)


def healthy(endpoint):
    try:
        with urllib.request.urlopen(endpoint["url"].rstrip("/") + endpoint["health_path"], timeout=3) as r:
            return r.status == 200
    except (urllib.error.URLError, TimeoutError, ConnectionError):
        return False


class Controller:
    def __init__(self, client, state, resolve=model.endpoint, backend=model.backend_command):
        self.client = client
        self.state = state
        self.resolve = resolve
        self.backend = backend

    def plan(self, route):
        endpoint = self.resolve(route)
        current = self.client.configuration()
        original = current.get("config") or {"ingress": [{"service": "http_status:404"}]}
        config = deepcopy(original)
        ingress = config.get("ingress")
        if not isinstance(ingress, list) or not ingress or not all(isinstance(x, dict) for x in ingress):
            raise ValueError("The existing tunnel ingress configuration is invalid")
        if ingress[-1].get("hostname") or ingress[-1].get("path") or not ingress[-1].get("service"):
            raise ValueError("The existing tunnel must end with a catch-all rule")
        matches = [x for x in ingress if x.get("hostname") == route.hostname]
        owned = self.state.read().get(route.hostname)
        if matches:
            if len(matches) != 1 or matches[0].get("path"):
                raise Conflict(f"{route.name}: existing hostname has path rules or multiple entries")
            rule = matches[0]
            if rule["service"] != endpoint["url"]:
                if not owned or owned.get("origin") != rule["service"] or owned.get("gateway") != route.gateway:
                    raise Conflict(f"{route.name}: existing hostname points to an origin not owned by this gateway")
                rule["service"] = endpoint["url"]
        else:
            # An exact new hostname precedes wildcard rules; existing order stays intact.
            ingress.insert(0, {"hostname": route.hostname, "service": endpoint["url"]})
        target = self.client.gateway["tunnel_id"] + ".cfargotunnel.com"
        records = self.client.records(route.hostname)
        if records:
            if (len(records) != 1 or records[0].get("type") != "CNAME"
                    or records[0].get("content", "").rstrip(".").lower() != target
                    or not records[0].get("proxied")):
                raise Conflict(f"{route.name}: existing DNS differs from the intended proxied Tunnel CNAME")
        return {"route": route, "endpoint": endpoint, "before": current, "config": config,
                "ingress_change": config != original, "dns_create": not records,
                "record": records[0] if records else None, "target": target}

    def apply(self, route):
        plan = self.plan(route)  # Conflict checks precede backend/cloud mutations.
        self.backend(route, "deploy")
        if not healthy(plan["endpoint"]):
            raise RuntimeError(f"{route.name}: local backend is not healthy; cloud configuration unchanged")
        latest = self.client.configuration()
        if latest.get("version") != plan["before"].get("version") or latest.get("config") != plan["before"].get("config"):
            raise Conflict(f"{route.name}: tunnel changed since the preview; rerun plan")
        # Recheck DNS immediately before changing the tunnel, too.
        records = self.client.records(route.hostname)
        if records != ([plan["record"]] if plan["record"] else []):
            raise Conflict(f"{route.name}: DNS changed since the preview; rerun plan")
        self.state.snapshot(plan["before"])
        state = self.state.read()
        previous = state.get(route.hostname)
        record = {"route": route.name, "gateway": route.gateway, "backend": route.backend,
                  "origin": plan["endpoint"]["url"], "cname": plan["target"], "phase": "publishing"}
        state[route.hostname] = record
        self.state.write(state)
        ingress_ready = not plan["ingress_change"]
        try:
            if plan["ingress_change"]:
                self.client.update_configuration(plan["config"])
                ingress_ready = True
            if plan["dns_create"]:
                dns = self.client.create_record(route.hostname)
            else:
                dns = plan["record"]
            record.update({"dns_id": dns["id"], "phase": "published"})
            self.state.write(state)
        except Exception:
            # DNS and Tunnel updates are separate operations. Keep partial progress
            # explicit and retryable; do not overwrite concurrent dashboard edits.
            if not ingress_ready:
                if previous:
                    state[route.hostname] = previous
                else:
                    state.pop(route.hostname, None)
            else:
                record["phase"] = "dns-pending"
            self.state.write(state)
            raise
        self.verify(route)
        return plan

    def verify(self, route):
        endpoint = self.resolve(route)
        if not healthy(endpoint):
            raise RuntimeError(f"{route.name}: local backend is not healthy")
        self.backend(route, "verify-public", "--hostname", route.hostname)
        state = self.state.read()
        entry = state.setdefault(route.hostname, {})
        entry.update({"route": route.name, "gateway": route.gateway, "backend": route.backend,
                      "origin": endpoint["url"], "phase": "verified",
                      "verified_at": datetime.now(timezone.utc).isoformat()})
        self.state.write(state)
