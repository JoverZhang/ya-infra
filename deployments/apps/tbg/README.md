# Telegram Bot Gateway

`config.sops.yaml` is the native server configuration, encrypted from
[config.example.yaml](config.example.yaml). Optional `config.sops.env` selects the
existing external data volume. The image must already exist locally.

Run from this directory with the upstream `tbg` host client installed:

```sh
go run . setup
go run . verify
```

Setup preserves the server document in the owned Podman secret
`ya-infra-tbg-server` and derives `$XDG_CONFIG_HOME/tbg/client.yaml` from its loopback
listen address. Identical content is adopted; unrelated host edits and unowned
secret names are refused. The client ownership record remains under
`$XDG_STATE_HOME/ya-infra/tbg/`. A failed secret write can follow a completed client
write; fix the error and retry setup before starting or recreating the server.

Setup is explicit. Run it again after server-config changes, then recreate the
container to mount the updated secret. The user unit runs Compose only.

Compose requires an existing, nonempty `gateway.sqlite`. Only for a deliberately
new bot database, run `just compose tbg run --rm gateway 'exec tbg-gateway'` from
the repository root, then stop it before normal startup. This explicit command
bypasses the existing-database guard. Stop the old poller before takeover, keep the
same database and bindings, and follow [backup and rollback](../../docs/migration.md).

Verify expects HTTP 405 from `/v1/whoami`. This proves local protocol reachability,
not Telegram delivery. Send a test message only when explicitly requested.
