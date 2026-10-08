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

## Publish a Paseo instance

The recovered connector token starts the tunnel. No Cloudflare management API
credential has been supplied to this module. Adding a route therefore requires
the account's dashboard. A repeatable helper configures the selected hostname,
redeploys that instance and walks through the dashboard step:

```bash
bash services/cloudflare-tunnel/publish-paseo-wizard.sh demo paseo-demo.example.com
```

Only run this when that hostname is the intended route. The human step is to go to
**Networking → Tunnels → the existing tunnel → Routes → Add route → Published
application**. Choose that hostname and set its service URL to
`http://127.0.0.1:<instance-port>`. Keep the existing routes. Each Paseo instance
continues to require its own daemon password.

After saving the route, the helper checks public HTTPS, password enforcement and
an authenticated WebSocket connection. DNS and certificate propagation may take
time; rerun `just verify-public paseo demo` if the first check is too early.
No public route is created by the deployment scripts themselves.

Sources: [Cloudflare published application setup](https://developers.cloudflare.com/tunnel/get-started/),
[cloudflared run parameters](https://developers.cloudflare.com/tunnel/reference/run-parameters/).
