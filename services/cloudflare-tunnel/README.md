# Cloudflare Tunnel

This module takes over the existing `cloudflare-tunnel` connector. It keeps the
same remotely managed tunnel and its Cloudflare-side route configuration.
`versions.toml` pins the existing cloudflared **2026.7.2** image by digest;
`config.toml` controls resource limits and the loopback metrics port.

```bash
cd ~/infra
just adopt cloudflare-tunnel cloudflare-tunnel
just build cloudflare-tunnel
just deploy cloudflare-tunnel
just status cloudflare-tunnel
just verify cloudflare-tunnel
```

`adopt` reads the existing token without displaying it and stores it at
`~/.config/infra/cloudflare-tunnel/token` (mode 0600). Deployment passes it through
standard input into a labeled Podman secret, mounted read-only at
`/run/secrets/tunnel-token` with uid/gid 65532 and mode 0400. The new connector uses
`--token-file`; its process arguments and environment contain no tunnel token.
The local credential file and Podman's secret storage both stay outside Git.

The managed container is `infra-cloudflare-tunnel`, with `restart=always`,
uid/gid 65532, a read-only image filesystem, no capabilities and
`no-new-privileges`. Host networking preserves existing routes to host loopback
services. This connector can access host networking; the separate Paseo runtime
uses its own restricted bridge. Metrics and readiness bind only to
`127.0.0.1:20242`.

On the first migration, the new connector joins the same tunnel as a second
replica. Only after its `/ready` endpoint succeeds is the old connector stopped.
The old `cloudflare-tunnel` container is kept for rollback. Later replacements
restore the prior managed container if startup fails. An unchanged deploy keeps
the current container. Existing routes remain configured in Cloudflare:

| Hostname | Origin |
| --- | --- |
| `privateapi.example.com` | `http://127.0.0.1:28080` |
| `cpe.example.com` | `http://localhost:27310` |

```bash
just logs cloudflare-tunnel
just restart cloudflare-tunnel
just stop cloudflare-tunnel
```

To temporarily roll back this migration, stop the managed connector and start
the retained legacy container through the same rootless socket:

```bash
just stop cloudflare-tunnel
podman --remote --url unix:///run/user/$(id -u)/podman/podman.sock start cloudflare-tunnel
```

This is an immediate rollback. The managed connector's `always` policy may start
it again at the next boot. No route or credential is deleted by these commands.

## Gateway publication

The recovered connector token starts the tunnel. The separate
[gateway](../../gateway/README.md) manages hostname routes and DNS using a scoped
Cloudflare management API token. The connector module owns only the running
`cloudflared` container, its resources and its connector credential.

```bash
just gateway setup
just gateway plan paseo1
just gateway apply paseo1
```

The gateway reads the current remote configuration and merges only the selected
Route. Existing `privateapi.example.com`, `cpe.example.com`, origin options and the
catch-all rule are preserved. Each Paseo instance requires its own daemon password.

The old script name remains as a forwarding entry point:

```bash
bash services/cloudflare-tunnel/publish-paseo-wizard.sh demo paseo1.example.com
```

It requires an already declared Route and invokes gateway `apply`; it does not
edit backend settings or offer a second hostname source. DNS and certificate
propagation may take time. If publication succeeds but the public checks fail,
rerun `just gateway verify paseo1` after propagation.

Sources: [Cloudflare published application setup](https://developers.cloudflare.com/tunnel/get-started/),
[cloudflared run parameters](https://developers.cloudflare.com/tunnel/reference/run-parameters/).
