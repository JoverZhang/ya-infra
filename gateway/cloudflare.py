"""Cloudflare management credentials and the small API surface used by gateway."""
import json
import os
from pathlib import Path
import stat
import urllib.error
import urllib.parse
import urllib.request


class MissingCredentials(RuntimeError):
    pass


def credentials_file():
    return Path(os.environ.get("XDG_CONFIG_HOME", str(Path.home() / ".config"))) / "infra/gateway/public/credentials.env"


def api_token():
    p = credentials_file()
    if p.exists():
        if not p.is_file() or stat.S_IMODE(p.stat().st_mode) & 0o077:
            raise ValueError(f"Management credential file must be private (mode 0600): {p}")
        values = [line.partition("=")[2].strip() for line in p.read_text().splitlines()
                  if line.startswith("CLOUDFLARE_API_TOKEN=")]
        if len(values) != 1:
            raise ValueError("Credential file must contain one CLOUDFLARE_API_TOKEN entry")
        token = values[0]
    else:
        token = os.environ.get("CLOUDFLARE_API_TOKEN") or os.environ.get("CF_API_TOKEN")
    if not token:
        raise MissingCredentials(f"Cloudflare management credential missing. Run 'just gateway setup' or save it in {p}")
    if len(token) < 20 or any(x.isspace() for x in token):
        raise ValueError("Invalid management token format; credential values are not displayed")
    return token


class NoRedirect(urllib.request.HTTPRedirectHandler):
    def redirect_request(self, req, fp, code, msg, headers, newurl):
        return None


class Client:
    def __init__(self, gateway, token=None):
        self.gateway = gateway
        self.token = token or api_token()
        self.opener = urllib.request.build_opener(NoRedirect)
        self._zone_id = None

    def request(self, method, path, body=None):
        data = json.dumps(body).encode() if body is not None else None
        req = urllib.request.Request("https://api.cloudflare.com/client/v4" + path, data=data, method=method,
                                     headers={"Authorization": "Bearer " + self.token,
                                              "Content-Type": "application/json"})
        try:
            with self.opener.open(req, timeout=20) as r:
                response = json.load(r)
        except urllib.error.HTTPError as e:
            # Provider errors may echo inputs; show codes, never response bodies.
            try:
                codes = [x.get("code") for x in json.loads(e.read()).get("errors", [])]
            except (ValueError, TypeError):
                codes = []
            raise RuntimeError(f"Cloudflare {method} failed: HTTP {e.code}, error codes {codes}. Check account, Tunnel, DNS and Zone permissions") from None
        except (urllib.error.URLError, TimeoutError, ConnectionError):
            raise RuntimeError(f"Cloudflare {method} request could not complete; rerun plan before retrying") from None
        if not response.get("success"):
            codes = [x.get("code") for x in response.get("errors", [])]
            raise RuntimeError(f"Cloudflare {method} rejected the request; error codes {codes}")
        return response["result"]

    def zone_id(self):
        if not self._zone_id:
            query = urllib.parse.urlencode({"name": self.gateway["zone"], "account.id": self.gateway["account_id"]})
            result = self.request("GET", "/zones?" + query)
            if len(result) != 1 or result[0].get("account", {}).get("id") != self.gateway["account_id"]:
                raise RuntimeError("The configured zone was not found in the gateway account")
            self._zone_id = result[0]["id"]
        return self._zone_id

    def configuration_path(self):
        return f"/accounts/{self.gateway['account_id']}/cfd_tunnel/{self.gateway['tunnel_id']}/configurations"

    def configuration(self):
        result = self.request("GET", self.configuration_path())
        if result.get("source", "cloudflare") != "cloudflare":
            raise RuntimeError("This gateway requires the existing remotely managed tunnel")
        return result

    def update_configuration(self, config):
        return self.request("PUT", self.configuration_path(), {"config": config})

    def records(self, hostname):
        query = urllib.parse.urlencode({"name": hostname, "per_page": 100})
        return self.request("GET", f"/zones/{self.zone_id()}/dns_records?{query}")

    def create_record(self, hostname):
        return self.request("POST", f"/zones/{self.zone_id()}/dns_records", {
            "type": "CNAME", "name": hostname,
            "content": self.gateway["tunnel_id"] + ".cfargotunnel.com",
            "proxied": True, "ttl": 1, "comment": "Managed by infra/gateway"})
