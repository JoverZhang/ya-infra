# Development site

Compose runs upstream git-sync and a Node development server. The directory keeps
its existing name so callers and the external volume need no rename. Private
repository, branch, public hostname and URL base belong in ignored `config.sops.env`,
shaped by `config.example.env`. Keep private names out of tracked files and PRs.

```sh
podman volume create static-site-data
# Prepare the repository secret below, then build the local web image.
just compose static-site build web
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

Vibe Annotations reads the page's React component information. No app plugin or
MCP service is installed here. For a public URL, enable that site in the browser
extension; see [its troubleshooting guide](https://www.vibe-annotations.com/docs/troubleshooting).

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
