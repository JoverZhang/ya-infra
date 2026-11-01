# Development site

Compose runs Vite directly against the host worktree selected by `SITE_WORKTREE`.
The directory keeps its existing name so callers and external volumes need no rename. Private
repository, branch, public hostname and URL base belong in ignored `config.sops.env`,
shaped by `config.example.env`. Keep private names out of tracked files and PRs.

```sh
podman volume create static-site-data
podman volume create static-site-annotations
# Prepare the repository secret below, then build the annotations image.
just compose static-site build annotations
just compose static-site up -d
just compose static-site ps
```

## Source and runtime

Web bind-mounts the absolute `SITE_WORKTREE` at the same container path and runs
`npm install`, then Vite there. Host file saves update the shared preview through
HMR; commits, pushes, git-sync, copying and container restarts do not gate rendering.
Use this shared preview for feedback verification instead of starting another dev
server. Native init/restart owns the Vite process. Dependencies are shared with the
host worktree; rootless container UID 0 writes as the invoking host user.

The existing git-sync mirror and external source volume remain solely for the
Inbox's upstream monitoring. They do not supply live Web source. No Git credential
is mounted into Web. The source app must deny `.codex`, `.infra`, env/key/cert and
Git metadata from HTTP. The worktree must exist; a missing bind source fails.

Set `SITE_BASE_PATH` to an absolute URL path with a trailing slash, and set
`SITE_DEV_HOST` to the single public hostname. Vite permits that Host alongside
its local defaults; do not disable Host checking. Gateway must forward WebSockets
for HMR. The existing loopback page port remains its HTTP origin; do not expose
the sync readiness port. `NODE_ENV=development`, React dev metadata, source maps
and Fast Refresh remain available; no production build runs.

## Shared design feedback

The source app must mount Agentation in development, enable `useHashLocation`, and
proxy `/__annotations` to `AGENTATION_PROXY_TARGET`. Compose supplies that internal
target and `VITE_AGENTATION_ENDPOINT`; only the latter is browser configuration.
`SITE_ORIGIN` is the exact public origin allowed by the backend's CORS policy.

The annotations container runs HTTP only. SQLite lives in the external annotations
volume; stopping a client or recreating the container preserves feedback. Its host
port is loopback only. The public proxy needs Cloudflare Access on the preview
hostname: CORS is not authentication. Access is configured by the operator, not by
this app. Until enabled, the same-origin annotation API is publicly reachable.

Codex is started and closed manually. Do not put it in Compose, systemd, scheduled
tasks or a webhook. In the source worktree's local `.codex/config.toml`, configure
the adapter (not another HTTP server):

```toml
[mcp_servers.agentation]
command = "npx"
args = ["-y", "agentation-mcp@1.3.2", "server", "--mcp-only", "--http-url", "http://127.0.0.1:4747"]
startup_timeout_sec = 60
tool_timeout_sec = 360
```

The operator launches `codex --no-daemon` in that worktree, checks `/mcp`, and asks
it to process this project's feedback. Read unresolved backlog, including
acknowledged unfinished work, before watching new events. Verify source changes
and the shared preview before resolving notes. Commit and push preserve source;
they are not a prerequisite for rendering local edits.
Use `/quit` to exit; the HTTP backend keeps accepting feedback without a client.
No Codex process is launched by deployment or its verification checks.

## Credentials and upgrades

Keep a read-only Git token in ignored `token.sops.env`, then prepare its native secret:

```sh
sops exec-env token.sops.env \
  'printf %s "$GITHUB_TOKEN" | podman secret create --replace ya-infra-static-site-github -'
```

Repository URLs must not contain credentials. Pin `SYNC_IMAGE` and `WEB_IMAGE` by
digest. Web uses the official Node image; no custom copying runtime is built.
Changing the worktree path or runtime configuration requires recreating Web.
Keep the worktree available for native boot recovery. Closing Codex does not stop
the preview or annotation service, and no static fallback is served.
