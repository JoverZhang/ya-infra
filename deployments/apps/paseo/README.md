# Paseo

One Kustomization assembles the Web UI and daemon workloads. Each daemon has a
Deployment, Service, Secret, home PVC, and workspace PVC. Duplicate these native
resources to add a daemon; add its Secret to `ksops.yaml`. Give it a unique password
and volume claims. Web has no daemon volumes. Set allowed hostnames and browser
origins in the daemon Secret; Gateway does not alter application configuration.

```sh
go -C deployments build -o .bin/paseo ./apps/paseo
deployments/.bin/paseo --help
deployments/.bin/paseo build
```

Build uses a verified public program snapshot from the cache or installed programs
with matching versions. `--artifacts` selects a snapshot explicitly. It excludes
credentials and rejects escaping symlinks. Image recipes and helpers are under
`images/`. The hook prints local image digests; it does not publish, import, or edit
manifests. For an upgrade, update version pins, prepare a matching snapshot, build,
and update the affected image references. A Web-only change updates only Web.

The init container requires a verified restore marker on both volumes. For a new
daemon only, explicitly add `--new` to its init command. It writes initialization
markers; remove the flag after the first successful start. [Migration](../../docs/migration.md)
owns restore verification and activation.

Use native Paseo pairing and the upstream Web UI. Login runs explicitly inside the
selected daemon after storage is ready. Keep each login cache with its daemon.
`verify --web-url URL --daemon-url URL` checks HTTP authentication and an authenticated
WebSocket session list. It sends no model request. `--secret-file` selects another
daemon's encrypted native Secret. Use loopback forwarding or a declared daemon hostname.
