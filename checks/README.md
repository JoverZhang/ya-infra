# Deployment verification

```bash
just verify paseo
just smoke paseo demo
just verify cloudflare-tunnel
python3 checks/gateway
just gateway plan paseo1
# After publishing the Route:
just gateway verify paseo1
```

`verify paseo` restarts the disposable `demo`, creates temporary persistence
markers and removes them afterwards. Other running configured instances are
checked without restarting them. `smoke` makes one Codex turn through Paseo's
authenticated WebSocket interface and verifies the actual generated file rather
than trusting the agent's completion status. Its temporary file is removed.

## Verified on 2026-10-08

- One active Paseo instance: `demo`, loopback port 16770. Paseo 0.9.1,
  Codex 0.159.2, Node 25.9.0, with the complete Codex execution distribution.
- Non-root runtime, read-only image, zero effective capabilities,
  no-new-privileges, configured CPU/memory/PID limits and only its own two volumes.
- Unauthenticated and wrong-password API requests denied; its password accepted.
  Bundled web UI served and authenticated WebSocket/agent operations worked.
- Host workspace and Podman socket absent, tested host management ports blocked,
  public HTTPS reachable. An unchanged deploy preserved the container ID.
- Workspace and local session storage survived a restart. The actual Codex
  read/write turn passed, and its Paseo session was present after the restart.
- Cloudflared 2026.7.2 reused the existing image digest and tunnel token. Four edge
  connections were established. Both existing ingress entries and the fallback
  were preserved. The legacy connector is stopped and retained for rollback.
- Connector process has zero effective/bounding capabilities, no-new-privileges,
  loopback-only metrics, a private secret mount and no token argument/environment.
  An unchanged managed connector deployment preserved the container ID.
- Existing origins behaved the same before/after migration: port 28080 responded
  with HTTP 200; port 27310 refused connections before migration and afterwards.

## Gateway checks on 2026-10-09

- Fifteen offline checks passed: preservation of unrelated configuration and
  rule options/order, DNS and origin conflicts, path-rule protection, drift
  during deployment, backend health failure, idempotence, DNS failure/retry,
  failed public verification, declaration validation and private credentials.
- `paseo1.example.com → paseo/demo` is declared. Plan and status work without the
  management API token and report remote state as unknown. Missing credentials
  stopped apply before backend replacement or cloud writes.
- Demo redeployment retained its Codex login. Runtime isolation and restart
  persistence checks passed again. The declared hostname was accepted locally;
  an undeclared DNS hostname was denied. Real Codex inference through Paseo
  produced and read the expected workspace file, then the temporary file was
  removed. The bundled web UI and password authentication passed locally.
- Python syntax, shell syntax and an exact comparison of the credential
  wizard's shared library to the skill template passed. Shellcheck is not
  installed. The interactive credential wizard was inspected statically.
- Initial checks had no management API credential, so no live gateway API write
  was exercised. Public verification was attempted and failed during TLS
  with `UNEXPECTED_EOF_WHILE_READING` before publication. Subsequent credential
  repair and successful publication are recorded below.

## Credential diagnosis on 2026-10-09

The subsequently supplied token verified as active. Zone lookup returned
HTTP 403/code 9109; Tunnel configuration and a direct DNS read using the supplied
Zone ID returned HTTP 401/code 10000. The dashboard settings supplied by the
owner contained Zone Custom Assets in place of Zone Read, and displayed an IP
Include value of `0.0.0.0`. After the owner reported token edits, the failures
persisted. Inspecting the provider message in memory established the cause:
Cloudflare explicitly rejected this server's outgoing IPv4 address under the
token's Client IP Address Filtering condition. The message and token secret were
not logged. After the owner corrected the dashboard settings, Zone, Tunnel and
DNS reads passed. A transient token-verification transport failure passed on
retry; the saved token verified as active.

The revised credential check reports independent results, permissions and
resource scopes, keeps secrets/provider messages out of errors, and labels
unexercised write permissions explicitly. Confirmed IP denials now report
`BLOCKED` and a validated allow-list CIDR without suggesting a missing Zone
permission. Twenty-three offline checks pass, including hidden Tunnel failures,
actionable Zone errors, IPv4/IPv6 IP denial classification, credential redaction
(including IPv6 scope IDs), account token verification and read-only diagnostics.

## Public publication verified on 2026-10-09

- Live Tunnel configuration and DNS writes succeeded for
  `paseo1.example.com → paseo/demo → http://127.0.0.1:16770`, exercising both required
  Edit permissions. DNS is a proxied CNAME to the configured Tunnel.
- Removing the new ingress entry from the returned configuration reproduced the
  complete pre-publication configuration, including existing origin settings and
  the catch-all. The Tunnel connector remained ready.
- HTTPS web UI returned 200. API requests without a password and with a wrong
  password returned 401; the instance password returned 200. The real Paseo CLI
  connected over the public authenticated WebSocket. The Route is recorded as
  `verified`.
- HTTP probes showed 403 with Python's default User-Agent and 200 with an explicit
  verifier User-Agent. The public check now identifies itself as
  `infra-gateway-verifier/1.0` and passes all status and WebSocket checks.

Two-instance runtime isolation has not been exercised yet. No visual browser
inspection or machine reboot was performed. Existing public hostnames returned
Cloudflare HTTP 403 from this runner, so full external request delivery to those
origins was not established by the migration checks.
