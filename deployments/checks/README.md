# Checks

Run from the repository root:

```sh
go -C deployments test ./...
go -C deployments vet ./...
go -C deployments test -race ./...
go -C deployments test -tags=integration -run TestGatewayConfigAndSecrets -v .
go -C deployments test -tags=integration ./checks -v
```

Integration needs SOPS, age-keygen, just, podman-compose, rootless Podman, and the
pinned Node image in [the Compose fixture](testdata/compose.yaml). TBG setup also needs its host client.
Checks use temporary keys, synthetic credentials, unique external volumes and
secrets. They verify forwarding, secret preparation, idempotency, missing-input
failures, read-only mounts, and retained data after native `down -v`. Each emits a
compact evidence record and cleans its resources; no live inputs or bot tokens are
used. [Migration](../docs/migration.md) owns live acceptance and rollback checks.
