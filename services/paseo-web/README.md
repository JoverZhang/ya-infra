# Shared Paseo web entry

Open [paseo.example.com](https://paseo.example.com) and add the environments you want
to use. The website is the shared client; each host is a separate Paseo daemon:

| Address | Role |
| --- | --- |
| `https://paseo.example.com` | Shared browser UI, served by `paseo-web/main` |
| `paseo1.example.com:443`, SSL enabled | `paseo/demo` environment |

In **Direct connection**, enter Host `paseo1.example.com`, Port `443`, enable
**Use SSL**, and enter the password from `~/.config/infra/paseo/demo/password`.
The advanced Connection URI is `tcp://paseo1.example.com:443?ssl=true`; keep the
password in its separate field.

The connection catalog and saved passwords live in each browser's localStorage
under the shared website origin. Different browsers configure their own catalog.
Sessions and files live in the selected backend container. Two people connecting
to the same host can access that same environment; separate hosts/containers
provide separate environments. The shared UI has no server-side user accounts or
agent data. Saved connections from the old `paseo1.example.com` website are not
copied automatically across browser origins.

## Deployment

```bash
just build paseo-web
just deploy paseo demo             # Regenerate its allowed browser origins
just gateway plan paseo-web
just gateway apply paseo-web
just verify paseo-web
just gateway verify paseo-web
just status paseo-web
just logs paseo-web
```

Declare the shared website in `gateway/routes/paseo-web.toml`. Paseo environments
derive allowed HTTPS origins from the `paseo-web` Routes, plus their own Routes.
Deploy an environment again after adding or changing a shared UI Route; the
Paseo login, home and workspace remain in its existing volumes. A blanket `*`
origin allowance is not used. Apply the environment Routes before the shared UI;
`just gateway apply` automatically uses this order. Shared UI public verification
also checks the declared environments, so they must already be available.

The image copies only the bundled web assets from the same verified program
snapshot used by `services/paseo`. It uses digest-pinned unprivileged NGINX,
UID/GID 101, a read-only root filesystem, zero capabilities, no-new-privileges,
CPU/memory/PID limits and an internal bridge network. It publishes only
`127.0.0.1:16780`, has no persistent data or credential mounts, and includes no
Codex executable. The Cloudflare Tunnel provides public HTTPS.

The frontend serves SPA history routes and static assets. `/api`, `/ws` and `/mcp`
are not daemon endpoints on the shared website; the browser connects to the
selected environment directly. Raw static HTML has no initial-daemon hint, so
the app does not try to connect to the frontend server as a daemon.

`endpoint main` and `verify-public main --hostname <declared-hostname>` implement
the standard gateway backend interface. `verify-public` checks HTTPS, JavaScript
assets, history fallback, absence of daemon endpoints, CORS preflight and
correct/wrong/no-password behavior. It uses the pinned Paseo client over a
WebSocket with the browser Origin and bearer subprotocol to read agents; a
different, unlisted Origin must receive HTTP 403. Secrets remain in memory/env,
outside command arguments and printed library errors.

Sources: [Paseo web UI](https://github.com/getpaseo/paseo/blob/main/public-docs/web-ui.md),
[NGINX unprivileged image](https://github.com/nginx/docker-nginx-unprivileged).
