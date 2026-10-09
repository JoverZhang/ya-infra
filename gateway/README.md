# Shared public gateway

`public.toml` identifies the existing remotely managed Cloudflare Tunnel and its
account and zone. These identifiers are public references, not credentials.
`routes/<name>.toml` maps one exact hostname to a backend instance:

```toml
gateway = "public"
hostname = "paseo1.example.com"
backend = "paseo/demo"
```

The gateway is the control module. The existing connector carries requests to
the backend's loopback endpoint; no extra proxy or Kubernetes installation is
needed. Routes share the gateway but keep separate backend containers, files,
sessions and passwords. Sharing an OpenAI login account still shares account usage.

## Commands

```bash
cd ~/infra
just gateway setup                 # One-time, human API token creation
just gateway plan paseo1           # Read-only remote preview
just gateway apply paseo1          # Deploy backend, merge route, DNS, public checks
just gateway status paseo1         # Local health, last publication, remote diff
just gateway verify paseo1         # HTTPS, password enforcement and WebSocket
```

Omitting the Route name selects all declared Routes, applied sequentially.
`setup` configures the shared gateway once and previews the Routes without
publishing them. Without a management credential, `plan` and `status` show the
desired declarations but explicitly report remote state as unknown. `apply`
fails before deploying or changing anything. `verify` can check a manually
published Route without a management API token.

## Credentials

The existing `~/.config/infra/cloudflare-tunnel/token` is the connector token;
`cloudflared` already uses it to join the tunnel. Route and DNS edits use a
separate management API token. The setup wizard opens
[Cloudflare API Tokens](https://dash.cloudflare.com/profile/api-tokens/) and asks
for a custom user token with these permissions:

| Scope | Permission | Resource restriction |
| --- | --- | --- |
| Account | Cloudflare One Connector: cloudflared / Edit (older label: Cloudflare Tunnel / Edit) | The account in `public.toml` |
| Zone | DNS / Edit | `example.com` |
| Zone | Zone / Read | `example.com` |

The secret is entered with hidden input and saved to
`~/.config/infra/gateway/public/credentials.env`, mode 0600, outside Git. The
wizard checks read access to the account's Tunnel configuration, zone and DNS;
write permissions are exercised by `apply`. `CLOUDFLARE_API_TOKEN` or
`CF_API_TOKEN` environment variables are also accepted when the private file is
absent. Credential directories follow `XDG_CONFIG_HOME` if set. API headers,
token values and provider response bodies are not printed; credentials are not
placed in command arguments. The browser password for Paseo remains separate.

## Apply and recovery

Before deployment, `apply` reads the current Tunnel configuration and exact-name
DNS records. A foreign origin, path-specific/multiple ingress entries, or DNS
that differs from the intended proxied Tunnel CNAME is a conflict. An existing
matching proxied CNAME is reused. A previously recorded gateway-owned origin may
be updated. This controller does not replace conflicting DNS, delete records or
prune removed Route files.

After backend deployment and its health check, remote configuration and DNS are
read again to catch intervening edits. The whole Tunnel configuration must be
sent by Cloudflare's update API, so the controller copies it and merges only the
selected hostname. New exact hostnames precede wildcard rules; unrelated rule
order, root settings, origin options and the fallback are retained. A private
snapshot of the configuration before the latest apply is saved alongside state
at `~/.local/state/infra/gateway/public/`. The gateway takes a local process lock;
the Cloudflare API does not provide an atomic transaction across these resources,
and a dashboard edit immediately after the final read remains a concurrency gap.

Tunnel and DNS updates are separate operations. If DNS creation fails after
ingress is ready, state records `dns-pending`; rerun `apply` to resume without
rewriting unchanged ingress. The controller avoids an automatic full-config
rollback that could discard someone else's dashboard edits. If cloud publication
works but a public check fails, state remains `published`, and `verify` can be
retried after propagation. Only successful HTTPS, correct/wrong/no-password and
authenticated WebSocket checks produce `verified`. `status` labels saved state
as the last publication, rather than treating it as a fresh public availability
check. Removing a Route declaration alone does not withdraw its remote route.

## Backend contract

Each backend lives at `services/<service>/manage` and accepts:

| Command | Responsibility |
| --- | --- |
| `endpoint <instance>` | Print JSON with loopback HTTP(S) `url` and absolute `health_path`; no engine or cloud write |
| `deploy <instance>` | Idempotently deploy the selected instance, preserving data |
| `verify-public <instance> --hostname <hostname>` | Check the backend's real public protocol and authentication |

The Route owns hostnames; the backend owns ports and resource settings. Paseo
derives its allowed hostnames from the Route catalog and exports its own port,
so these values are not copied into multiple configuration files. Add a second
instance and its login before publishing another Route. Browserlab migration
remains a separate discussion.

Offline controller checks run with `python3 checks/gateway`. They exercise
preservation, conflict detection, deployment-time drift, retry after partial
publication and failure of public verification. Live Cloudflare writes require
the human-created management credential.

Sources: [Create API tokens](https://developers.cloudflare.com/fundamentals/api/get-started/create-token/),
[Token permissions](https://developers.cloudflare.com/fundamentals/api/reference/permissions/),
[Tunnel configuration API](https://developers.cloudflare.com/api/resources/zero_trust/subresources/tunnels/subresources/cloudflared/subresources/configurations/),
[DNS record API](https://developers.cloudflare.com/api/resources/dns/subresources/records/methods/create/).
