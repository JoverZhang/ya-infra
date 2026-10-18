# Telegram gateway

TBG runs its unmodified upstream image under the native unit described in the
[runtime guide](../podman/README.md#native-container-units). The existing `tbg`
CLI, client configuration and Codex notification plugin continue to use it.
Build the image in the TBG source project and pin its local ID in `config.toml`.

`server_config` points to the upstream private server YAML (0600). Its `listen`
must use loopback, `data_dir` must be `/var/lib/tbg`, and `health_url` must match
that listener. The file is mounted read-only; changing it requires `deploy`.
Bot tokens, administrators and notification bindings remain owned by TBG.

`data_volume` must name the existing initialized gateway volume. Deployment
and boot require its `gateway.sqlite`; missing data causes a failure rather
than initializing an empty gateway. Back up SQLite using its backup API before
image upgrades. Keep the volume, client/plugin configuration and notification
bindings when migrating; they carry identities, topics, history and delivery state.

`verify` checks the gateway's HTTP 405 response to GET. This confirms the local
protocol listener; Telegram delivery is a separate check using normal traffic.
