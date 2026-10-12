"""Cloudflare management credentials and the small API surface used by gateway."""
import ipaddress
import json
import os
from pathlib import Path
import stat
import urllib.error
import urllib.parse
import urllib.request


class MissingCredentials(RuntimeError):
    pass


class APIError(RuntimeError):
    def __init__(self, method, path, status, codes, hint="", *, reason=None, client_ip=None):
        self.status = status
        self.codes = codes
        self.reason = reason
        endpoint = urllib.parse.urlsplit(path).path
        message = f"Cloudflare {method} {endpoint} failed: HTTP {status}, error codes {codes}"
        if reason == "client_ip":
            message += ". Blocked by token Client IP Address Filtering"
            if client_ip is not None:
                message += f"; allow {client_ip}/{client_ip.max_prefixlen} or remove the entire IP filter rule"
            else:
                message += "; remove the entire IP filter rule or allow this server's outgoing IP"
            message += " and save the token. This response does not establish missing permissions"
        elif hint and status in (401, 403):
            message += ". Check " + hint
        if reason != "client_ip" and status in (401, 403):
            message += ". Also check Client IP Address Filtering and token restrictions"
        super().__init__(message)


def error_codes(response):
    # Never print provider messages, which can echo credentials or request data.
    errors = response.get("errors", []) if isinstance(response, dict) else []
    if not isinstance(errors, list):
        return []
    return [x["code"] for x in errors
            if isinstance(x, dict) and type(x.get("code")) is int]


def error_reason(response):
    """Recognize the provider's explicit IP denial without exposing its message."""
    prefix = "Cannot use the access token from location:"
    errors = response.get("errors", []) if isinstance(response, dict) else []
    if not isinstance(errors, list):
        return None, None
    for error in errors:
        message = error.get("message") if isinstance(error, dict) else None
        if isinstance(message, str) and message.startswith(prefix):
            candidate = message[len(prefix):].strip()
            try:
                # IPv6 scope IDs may contain arbitrary text; never echo them.
                if "%" in candidate:
                    raise ValueError
                address = ipaddress.ip_address(candidate)
            except ValueError:
                address = None
            return "client_ip", address
    return None, None


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

    def permission_hint(self, method, path):
        endpoint = urllib.parse.urlsplit(path).path
        if endpoint == "/zones":
            return (f"Zone → Zone → Read; Zone Resources must include {self.gateway['zone']} "
                    f"in account {self.gateway['account_id']}")
        if endpoint == self.configuration_path():
            access = "Read or Edit" if method == "GET" else "Edit"
            return (f"Account → Cloudflare One Connector: cloudflared → {access} "
                    f"(or Cloudflare Tunnel → {access}); Account Resources must include "
                    + self.gateway["account_id"])
        if endpoint.startswith("/zones/") and endpoint.endswith("/dns_records"):
            access = "Read or Edit" if method == "GET" else "Edit"
            return f"Zone → DNS → {access}; Zone Resources must include {self.gateway['zone']}"
        return ""

    def request(self, method, path, body=None):
        data = json.dumps(body).encode() if body is not None else None
        req = urllib.request.Request("https://api.cloudflare.com/client/v4" + path, data=data, method=method,
                                     headers={"Authorization": "Bearer " + self.token,
                                              "Content-Type": "application/json"})
        try:
            with self.opener.open(req, timeout=20) as r:
                response = json.load(r)
                status = r.status
        except urllib.error.HTTPError as e:
            # Only numeric codes, known causes and validated IPs reach errors.
            try:
                response = json.loads(e.read())
            except (ValueError, TypeError, AttributeError):
                response = {}
            finally:
                e.close()
            reason, client_ip = error_reason(response)
            raise APIError(method, path, e.code, error_codes(response), self.permission_hint(method, path),
                           reason=reason, client_ip=client_ip) from None
        except (urllib.error.URLError, TimeoutError, ConnectionError):
            endpoint = urllib.parse.urlsplit(path).path
            raise RuntimeError(f"Cloudflare {method} {endpoint} request could not complete; rerun plan before retrying") from None
        if not response.get("success"):
            reason, client_ip = error_reason(response)
            raise APIError(method, path, status, error_codes(response), self.permission_hint(method, path),
                           reason=reason, client_ip=client_ip)
        return response["result"]

    def verify_token(self):
        try:
            return self.request("GET", "/user/tokens/verify")
        except APIError as user_error:
            if user_error.status not in (401, 403):
                raise
            # Account-owned tokens have a separate verification endpoint.
            try:
                return self.request("GET", f"/accounts/{self.gateway['account_id']}/tokens/verify")
            except APIError:
                raise user_error from None

    def zone_id(self):
        if not self._zone_id:
            query = urllib.parse.urlencode({"name": self.gateway["zone"], "account.id": self.gateway["account_id"]})
            result = self.request("GET", "/zones?" + query)
            if len(result) != 1 or result[0].get("account", {}).get("id") != self.gateway["account_id"]:
                raise RuntimeError(f"{self.gateway['zone']} was not found in account {self.gateway['account_id']}; "
                                   "check Zone Resources and the gateway account reference")
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
