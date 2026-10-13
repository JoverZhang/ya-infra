# Verification

Use the smallest check set that exercises the changed interface. Start with
`python3 checks/cli` for command/configuration changes and `python3 checks/gateway`
for routing/controller changes; both use isolated fixtures without live deployments
or cloud writes.
Run these commands from `deployments/`, or prefix their paths with `deployments/`
from the repository root. The CLI check exercises both Just entry points and
prints a JSON result to stdout for change evidence.
For backend storage/lifecycle changes, run `python3 checks/paseo-storage` for
offline checks of missing-volume, replacement and adoption safeguards.

## Runtime effects

| Check | Effect |
| --- | --- |
| `just svc paseo-web verify` | Local runtime inspection |
| `just gateway verify <route>` | Real HTTPS/authentication/WebSocket probes; records local verification state, without cloud writes |
| `just svc paseo verify` | Calls deploy, writes temporary markers and restarts `demo`; inspects other running instances |
| `just svc cloudflare-tunnel verify` | Calls deploy and checks idempotence; can replace a container if its specification differs |
| `just svc paseo smoke <instance>` | Real Codex model usage and a temporary workspace file |

Deployment specifications include manager/shared-runtime source as well as config/image
identity. A source-only change can therefore replace a container on its next
deploy and invalidate a verifier's unchanged-container expectation.

For backend/image changes, exercise runtime isolation/persistence and public
connection behavior; add the Codex smoke check when agent execution is affected.
A ready Tunnel edge alone does not establish origin delivery. Read `status` and
probe the relevant Route to determine current availability.

## Evidence

Record observations and limits in the change/commit report. Earlier deployment
evidence is available with `git log -p -- checks/README.md`; it is historical,
not a live-state guarantee. Exercise at least two live instances before claiming
peer isolation.
