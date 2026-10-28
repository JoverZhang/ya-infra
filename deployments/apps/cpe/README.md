# CPE

Compose runs the existing delivery image, defaulting to `cpe:local`. Select another
local tag or digest with `CPE_IMAGE`; startup never builds or pulls an image.

Set `config.sops.env` from [the example](config.example.env). `CPE_PROJECT_DIR`
selects a configured checkout with `compose.yaml`, `config.toml`, `data`, and
`worker_scripts/{scripts,algorithms}`. MySQL and Mongo inherit that checkout's
Compose declarations. Select all three existing database volumes explicitly;
the original MySQL and Mongo config volumes can have anonymous names.

```sh
# Run from the ya-infra root.
just compose cpe up -d
just compose cpe ps
just compose cpe logs --tail 50
just compose cpe stop
```

Daemon and Web share readonly native configuration and catalog mounts. The data
directory is also mounted at its original absolute path for historical Compare
records. Image-owned Web and reference assets remain visible. This layout expects
the checkout's default `data` and `worker_scripts` paths; adjust mounts if its
native configuration uses other paths. If native CPE `ENC(age:...)` fields are in
use, add the configured key file as a readonly mount for both services.

`config.sops.env` encrypts deployment variables; it does not encrypt the existing
native `config.toml`. Keep that file under CPE's own configuration management.
Do not run the checkout's `just up`, `down`, or `reset` against these same database
volumes after takeover. [Migration](../../docs/migration.md) covers backup and
single-writer ownership.

Host networking preserves the native database URLs. Set `CPE_WEB_PORT` to the
checkout's Web port and keep its existing Tunnel origin. Confirm external access
authentication at the proxy. Database health gates initial application startup;
`unless-stopped` handles runtime restart and the configured runtime boot recovery.
Daemon also waits for `cpe-client doctor` before execution because native boot
recovery does not apply Compose dependency health ordering.
