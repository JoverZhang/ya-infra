# Deployment verification

```bash
just verify paseo
just smoke paseo demo
just verify cloudflare-tunnel
# After a public route is configured:
just verify-public paseo demo
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

Two-instance runtime isolation has not been exercised yet. Public Paseo route
verification awaits the hostname and Cloudflare dashboard route. No visual
browser inspection or machine reboot was performed. Existing public hostnames
returned Cloudflare HTTP 403 from this runner, so full external request delivery
to those origins was not established by the migration checks.
