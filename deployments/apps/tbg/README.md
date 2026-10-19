# Telegram Bot Gateway

The server uses its upstream image and a retained PVC. KSOPS puts the decrypted
`kubernetes/config.sops.yaml` into the mounted `server.yaml` Secret key. The first
migration keeps the host-loopback API, its port, and existing notification bindings.

```sh
go -C deployments build -o .bin/tbg ./apps/tbg
deployments/.bin/tbg setup
deployments/.bin/tbg verify
```

Setup requires the upstream `tbg` client. It decrypts `host/client.sops.yaml` to the
client's `$XDG_CONFIG_HOME/tbg/client.yaml`. It adopts an identical existing file;
subsequent calls update only its recorded content. It refuses an unrelated file or
external edits. Private ownership state is under `$XDG_STATE_HOME/ya-infra/tbg/`.
Setup keeps notification bindings unchanged.

Verify expects HTTP 405 from the existing read-only probe. This proves local
protocol reachability, not Telegram delivery. Check an ordinary notification after
cutover; send a test message only on explicit request. [Migration](../../docs/migration.md)
owns SQLite backup, single-poller cutover, and recovery.
