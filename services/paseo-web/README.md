# Shared Paseo web entry

Open [paseo.example.com](https://paseo.example.com) and add the environments you want
to use. The website is the shared client; each host is a separate Paseo daemon:

| Address | Role |
| --- | --- |
| `https://paseo.example.com` | Shared browser UI, served by `paseo-web/main` |
| `paseo1.example.com:443`, SSL enabled | `paseo/demo` environment |

Generate a complete direct connection URI with one command, from any directory:

```bash
just -f ~/infra/justfile svc paseo pair
```

Choose an environment from the menu, or append `demo` to print its link directly.
Use `just svc paseo pair --list` to inspect candidates without printing passwords.
Copy the single output line, open the shared website, choose **Direct connection
→ Advanced → Connection URI**, and paste it. Press **Enter** (or close the
Advanced section) to apply the URI to the connection fields, then select
**Connect**. Paseo's **Paste pairing link** dialog handles relay offer links;
use Direct connection for this TCP URI. Refresh an already open tab after a
frontend deployment to load the current UI. The
`tcp://...:443?ssl=true&password=...` line includes the instance password; share
it only with the people who should use that
environment. For another instance, replace `demo` with its name. If an instance
has multiple declared public hostnames, choose the corresponding menu number or
pass `--hostname <hostname>`. Generating an explicit link reads local declarations
and the existing private password. The menu also reads the local container
status; no Cloudflare API request or deployment change is made. Stopped instances
can generate links for use after they start. Non-interactive calls must name the
environment (and hostname when ambiguous). Menus and errors go to stderr; only
the selected link goes to stdout. EOF or `q` cancels without printing a link.

For manual setup in **Direct connection**, enter Host `paseo1.example.com`, Port
`443`, enable **Use SSL**, and enter the password from
`~/.config/infra/paseo/demo/password`.

The connection catalog and saved passwords live in each browser's localStorage
under the shared website origin. Different browsers configure their own catalog.
Sessions and files live in the selected backend container. Two people connecting
to the same host can access that same environment; separate hosts/containers
provide separate environments. The shared UI has no server-side user accounts or
agent data. Saved connections from the old `paseo1.example.com` website are not
copied automatically across browser origins.

## Deployment

```bash
just svc paseo-web build
just svc paseo deploy demo             # Regenerate its allowed browser origins
just gateway plan paseo-web
just gateway apply paseo-web
just svc paseo-web verify
just gateway verify paseo-web
just svc paseo-web status
just svc paseo-web logs
```

Declare the shared website in `gateway/routes/paseo-web.toml`. Paseo environments
derive allowed HTTPS origins from the `paseo-web` Routes, plus their own Routes.
Deploy an environment again after adding or changing a shared UI Route; the
Paseo login, home and workspace remain in its existing volumes. A blanket `*`
origin allowance is not used. Apply the environment Routes before the shared UI;
`just gateway apply` automatically uses this order. Shared UI public verification
also checks the declared environments, so they must already be available.

The image copies the official bundled web assets unchanged from the same
verified program snapshot used by `services/paseo`, including the original
HTML, JavaScript filenames and gzip files. No frontend source patch or bundle
rewriting is applied. The direct connection URI uses Paseo's native import flow
on both the shared UI and daemon-served pages. Frontend deployments have no
persistent data to migrate; saved browser connections use Paseo's native format.

Versions stay pinned. For an upgrade, prepare a verified snapshot for the chosen
version, use new image tags, and test the backend and shared UI together on
`demo` before deploying the other environments. Keep the previous image tags
available for rollback. Native UI or protocol changes still need a compatibility
check, but there is no custom client patch to port.

The runtime uses digest-pinned unprivileged NGINX,
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
