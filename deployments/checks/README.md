# Checks

Run from the repository root. Offline checks need Go; CLI integration also uses
just when present. They use disposable files and a local HTTP provider fixture.

```sh
go -C deployments test ./...
go -C deployments vet ./...
go -C deployments test -race ./...
```

Real rendering requires the pinned SOPS, age-keygen, Kustomize, and KSOPS binaries
on PATH. It creates a temporary age key and removes plaintext on exit.

```sh
go -C deployments test -tags=integration ./checks -run TestEncryptedRendering -v
```

Cluster checks require an explicit **disposable** context and the local-path
provisioner. They create a unique namespace, StorageClass, and retained PV. They
test native apply/diff, Secret masking, config rollout, PVC deletion, and rebind.
They remove their API resources. Retained bytes stay on the test node until the
operator removes the disposable cluster. No production app credentials are used.

```sh
go -C deployments test -tags=integration ./checks \
  -run TestClusterApplyAndRetention -cluster-context=disposable -v
```

Each integration check prints a compact evidence record. Save command, revision,
and result in the PR. [Migration](../docs/migration.md) owns the additional live
acceptance gates; fixture success does not prove reboot or production recovery.

After importing the pinned app images into that disposable cluster, run
`TestApplicationImages` with the same context option. It starts two new Paseo
daemons, Web, and a Browserlab profile with temporary data. It tests native
WebSocket access, separate workspaces, desktop/CDP readiness, read-only workspace
mounts, and graceful shutdown. It does not use production login or bot credentials.
