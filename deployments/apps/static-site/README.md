# Static site

Compose runs git-sync with a Node build hook, and Caddy serves only the published
build output. Private repository, branch, base path, image and port values belong
in ignored `config.sops.env`, shaped by `config.example.env`. Keep private names
and hostnames out of tracked files and PR examples.

```sh
podman volume create static-site-data
# Prepare the native repository secret as described below.
just compose static-site build sync
just compose static-site up -d
just compose static-site ps
```

The sync image adds Node and a small Go hook to git-sync. `SYNC_IMAGE` selects the
upstream git-sync base; `SITE_SYNC_IMAGE` names the local built image. The Dockerfile
pins all base images by digest. Rebuild and recreate sync after a hook or base-image
change. `.dockerignore` excludes local configuration from the build context.

## Credentials

Prepare an ignored `token.sops.env` with a read-only repository token:

```sh
sops exec-env token.sops.env \
  'printf %s "$GITHUB_TOKEN" | podman secret create --replace ya-infra-static-site-github -'
```

The Web container never receives Git credentials. Repository URLs must not contain
credentials. Only trusted repository code may run here: npm install and build scripts
execute inside the sync container, which has the read token and sync volume.

## Build and publication

Every new checkout triggers this sequence in its own worktree:

```sh
npm install
npm run typecheck
# VITE_BASE_PATH is supplied from private SITE_BASE_PATH; default: /
npm run build
```

The hook withdraws the publication, deletes old releases and removes stale `dist/`
before running npm. It requires a nonempty `dist/index.html`, copies
`dist/` into `/repo/site/.release-*`, then atomically replaces `/repo/site/current`.
Caddy serves that link. `SITE_BASE_PATH` must be an absolute URL path with a trailing
slash, such as `/preview/`; Caddy also accepts assets under that prefix on the same
hostname. Source files, node_modules and Git metadata are outside the served root.

Git-sync polls every 60 seconds. Failed fetches keep retrying. Failed installs,
typechecks and builds remove the publication and its release output, and retry
after 60 seconds. Caddy returns 503 until a build succeeds; there is no fallback.
A build has a 15-minute timeout; stale source worktrees remain for 30 minutes.
Each successful build retains only its own release output. The page returns 503
during builds, including rebuilds after sync restarts. Hook timeouts leave it
unpublished; a later successful retry restores the page.
A killed copy can leave unpublished staging files; the next attempt removes them.
`/repo/sync-error` records fetch errors; `/repo/site/build-error` records build
failures. Inspect sync logs privately for details. Its readiness endpoint reports
checkout readiness, not build readiness. Inbox's existing fetch checks do not read
the build marker; check the page and build logs when diagnosing publication.

The existing external volume and token are reused. Select a source branch with
package.json. On a new installation the page returns 503 until a build succeeds.

```sh
just compose static-site up -d --no-deps --force-recreate sync
just compose static-site logs --tail 50 sync
just compose static-site exec -T sync test -s /repo/site/current/index.html
# Recreate Web after a Caddyfile or base-path change.
just compose static-site up -d --force-recreate web
```

The page port is the existing Gateway's HTTP origin. Keep sync's readiness port
local. No Web restart is needed for later repository updates. Browsers can request
files across a publication change; the link switch does not pin an entire browsing
session to one release.
