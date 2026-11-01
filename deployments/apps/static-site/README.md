# Static site

Native Compose runs git-sync and Caddy. Repository, branch, image digests and ports
come from ignored `config.sops.env`, shaped by `config.example.env`. No private
repository name or public hostname belongs in tracked source or PR examples.

```sh
podman volume create static-site-data
# Prepare a read-only repository token as native secret ya-infra-static-site-github.
just compose static-site up -d
just compose static-site logs --tail 50
```

Prepare a private ignored `token.sops.env` containing `GITHUB_TOKEN` and use:

```sh
sops exec-env token.sops.env \
  'printf %s "$GITHUB_TOKEN" | podman secret create --replace ya-infra-static-site-github -'
```
 A separate repository-scoped read token is sufficient; the
static Web container never receives Git credentials. Pin pulled images by digest
in the encrypted environment. Repository URLs must not contain embedded credentials.

The first successful synchronization publishes `/repo/current`. Subsequent fetches
run every 60 seconds and atomically switch the link. Failed fetches retry and keep
the last successful checkout. `sync-error` records a failure and is cleared by a
successful sync. Inbox checks only presence/size, never the diagnostic contents.
A stopped sync process is detected through its loopback readiness endpoint.

Caddy only serves the published worktree, hides `.git`, disables directory browsing
and asks browsers to revalidate unversioned assets. Both consumers mount the full
sync volume readonly so absolute worktree symlinks remain valid. A browser can still
make requests across two revisions during an update; use versioned asset names
when a page requires assets from one exact revision.

The loopback page port is the existing Gateway's HTTP origin. The sync readiness
port is local only and must not be published through Gateway. Choose whether the
page needs access restriction before exposing it. Initial startup may return 404
until the first checkout is published. No Web container restart is needed on updates.
