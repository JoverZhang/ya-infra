# Deployment verification

```bash
just svc paseo verify
just svc paseo smoke demo
just svc cloudflare-tunnel verify
just svc paseo-web verify
# Prints a credential-bearing link; paste it in the shared website:
just svc paseo pair demo
python3 checks/gateway
python3 checks/cli
just gateway plan paseo1
# After publishing the Route:
just gateway verify paseo1
just gateway verify paseo-web
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

## Shared web entry verified on 2026-10-09

- `paseo.example.com → paseo-web/main → http://127.0.0.1:16780` is published and
  recorded as `verified`. The image uses the verified Paseo 0.9.1 browser assets
  and a digest-pinned unprivileged NGINX base. The existing environment remains
  `paseo1.example.com → paseo/demo`.
- Removing the shared UI ingress entry reproduced the complete previous Tunnel
  configuration. Its DNS record is the intended proxied Tunnel CNAME; both
  Route backends are healthy. Demo's ChatGPT login remained available.
- The shared UI runs as UID/GID 101 with a read-only image, zero effective
  capabilities, no-new-privileges, resource limits and an internal bridge. It
  has no credential or persistent data mounts, workspace or Codex executable.
  An unchanged deployment retained its container ID.
- Public HTTPS, the JavaScript asset and SPA history fallback passed. The shared
  website serves no daemon endpoints and has no initial-daemon connection hint.
- Demo accepts the exact shared website Origin. Public CORS preflight passed;
  absent/wrong/correct passwords returned 401/401/200. The pinned Paseo client
  read agents over an authenticated WebSocket using the browser Origin and
  password subprotocol. An unlisted Origin was rejected with HTTP 403.
- All 27 offline checks passed, including declared origin derivation,
  environment-before-frontend apply order and transport retry behavior.
  Intermittent TLS interruptions occurred from this runner; the read-only
  shared UI HTTP probes retry transport failures up to three times, without
  retrying HTTP authentication failures or disabling certificate validation.

Two-instance runtime isolation has not been exercised yet. No visual browser
inspection or machine reboot was performed. Existing public hostnames returned
Cloudflare HTTP 403 from this runner, so full external request delivery to those
origins was not established by the migration checks.

## Direct pairing import verified on 2026-10-09

This records the former custom frontend import extension. The extension was
removed on 2026-10-10; current usage is Paseo's native Direct connection flow.

- The pairing link command was exercised from outside the repository. It
  emitted exactly one line containing the declared public host,
  port 443, TLS and the correct private instance password. An unrelated hostname
  was rejected without printing the link or password.
- The shared UI's pinned pairing dialog was extended to accept direct TCP URIs
  through its existing parser and authenticated probe/save operation. Bundle
  syntax, regenerated gzip and the new content hash passed. All verified source
  assets stayed byte-for-byte unchanged despite hard-linked build inputs; an
  incompatible dialog was rejected. Build and non-root runtime checks passed.
- Chromium imported the generated line, authenticated to the public demo
  WebSocket and saved host, TLS and password. The connection survived a reload;
  repeated import reused the saved environment. A malformed URI and a wrong
  password were rejected without saving a host.
- For browser interaction checks, Chromium loaded the exact deployed UI assets
  over loopback under the shared HTTPS Origin and connected to the real public
  backend. Direct browser navigation to Cloudflare timed out from this runner.
  The separate public HTTPS/assets/CORS/authentication/WebSocket checks passed
  and the frontend Route is recorded as `verified`.

The browser checks above exercise functional interactions; no manual visual
layout review or machine reboot was performed.

## Service command interface verified on 2026-10-10

- The public Just recipes are `svc` and `gateway`. `bin/svc` discovers module
  entrypoints and forwards arguments as separate values. Help, documentation and
  error hints use `just svc <service> <action>`.
- Ten isolated command checks passed through the real Just interface: module
  discovery, literal argument forwarding, service help, distinct environment
  credentials, credential-free listings and unavailable reasons, non-interactive
  behavior, name/number selection, single-target Enter default, cancellation,
  hostname aliases and unavailable container status. The 27 gateway checks also
  passed.
- The live demo was selected through a terminal and addressed explicitly from
  outside the repository. Both commands emitted exactly one correct link on
  stdout; metadata and prompts stayed on stderr without its password.
- Read-only status commands succeeded for all three modules: demo remained
  running and Codex authenticated, the shared UI was running and the Tunnel edge
  was ready. No build, deploy, stop or restart was run for this CLI change.

## Official frontend restored on 2026-10-10

- Removed `pairing.py`, `direct-pairing.js` and the frontend build hook. The
  shared UI was rebuilt and deployed with unchanged Paseo 0.9.1 distribution
  assets. All 57 deployed files matched the verified source snapshot byte for
  byte, including HTML, the original JavaScript filename and compressed assets.
- Chromium used the native Direct connection → Advanced → Connection URI flow:
  pasting the CLI output and pressing Enter applied host, port, SSL and password;
  Connect authenticated to the real public backend WebSocket and saved the
  connection. It remained saved after reload. A connection saved before the
  frontend replacement also survived and authenticated using the restored UI.
- Browser interactions loaded the exact deployed assets over loopback under the
  shared HTTPS Origin, with the backend WebSocket going to the real public host.
  Separate public HTTPS/assets/SPA/CORS/password/WebSocket checks passed and the
  shared frontend Route is recorded as `verified`.
- Frontend runtime restrictions passed. Only the frontend container was replaced;
  backend and Tunnel container IDs stayed unchanged and running, and the
  environment password was unchanged. No backend deployment, restart or model
  inference was performed for this restoration.
- All 10 command-interface and 27 gateway checks passed. Documentation and
  command help now describe native direct URI import. The private temporary
  browser profile used for the checks was deleted afterwards.
