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
  with `UNEXPECTED_EOF_WHILE_READING`; the Route is not marked verified. Run the
  credential wizard, apply the Route and repeat its public checks before sharing.

## Credential diagnosis on 2026-10-09

The subsequently supplied token verified as active. Zone lookup returned
HTTP 403/code 9109; Tunnel configuration and a direct DNS read using the supplied
Zone ID returned HTTP 401/code 10000. The dashboard settings supplied by the
owner contained Zone Custom Assets in place of Zone Read, and displayed an IP
Include value of `0.0.0.0`. Zone Read must be added; if that IP value is an actual
filter rather than a placeholder, the rule must be removed for this setup.

The revised credential check reports independent results, permissions and
resource scopes, keeps secrets/provider messages out of errors, and labels
unexercised write permissions explicitly. Twenty offline checks pass, including
regressions for hidden Tunnel failures, actionable Zone errors, credential
redaction, account token verification and read-only diagnostics. Actual token
policy corrections and public publication are still pending.

Two-instance runtime isolation has not been exercised yet. No visual browser
inspection or machine reboot was performed. Existing public hostnames returned
Cloudflare HTTP 403 from this runner, so full external request delivery to those
origins was not established by the migration checks.
