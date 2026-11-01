# Development site

Compose runs upstream git-sync and a Node development server. The directory keeps
its existing name so callers and the external volume need no rename. Private
repository, branch, public hostname and URL base belong in ignored `config.sops.env`,
shaped by `config.example.env`. Keep private names out of tracked files and PRs.

```sh
podman volume create static-site-data
podman volume create static-site-annotations
# Prepare the repository secret below, then build the local web image.
just compose static-site build web annotations
just compose static-site up -d
just compose static-site ps
```

## Source and runtime

Git-sync polls every 60 seconds and publishes `/repo/current`. Web reads that volume
readonly, copies source to its own `/home/node/site`, then runs `npm install` and
`npm run dev -- --host 0.0.0.0 --port 8080 --strictPort --base "$VITE_BASE_PATH"`.
The copy excludes `.git`, `node_modules` and `dist`. Dependencies and Vite caches
stay in the disposable container workspace; Git credentials never enter Web.

A checkout change or removal exits Web. Native init and `restart: unless-stopped`
end npm/Vite children and start a fresh workspace. Installation or server failures
retry after a short delay. While source or a working dev server is unavailable,
there is no static fallback; Gateway reports an unavailable origin. Fetch errors
retry without changing the selected checkout. Sync readiness checks only fetching.

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
acknowledged unfinished work, before watching new events. Verify source changes,
push the configured preview branch and verify deployment before resolving notes.
Use `/quit` to exit; the HTTP backend keeps accepting feedback without a client.
No Codex process is launched by deployment or its verification checks.

## Credentials and upgrades

Keep a read-only Git token in ignored `token.sops.env`, then prepare its native secret:

```sh
sops exec-env token.sops.env \
  'printf %s "$GITHUB_TOKEN" | podman secret create --replace ya-infra-static-site-github -'
```

Repository URLs must not contain credentials. Pin `SYNC_IMAGE` by digest; the web
Dockerfile pins Node and Go. After changing its runtime, rebuild Web and recreate
it. When switching from static serving, set `WEB_IMAGE` to the local dev image,
remove any legacy Caddy bind override, and delete owned `/repo/site` release output.
No release directory or old HTML seed participates in development serving.
