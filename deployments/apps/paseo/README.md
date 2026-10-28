# Paseo

One Compose project contains Web and native daemon services. Each daemon owns its
password, home volume (sessions and Codex login), and workspace volume. Web mounts
neither. Add a daemon by copying a Compose service and declaring separate external
volumes and environment keys; no instance abstraction or registry is involved.

Run hooks from this directory:

```sh
go run . build
go run . verify --web-url http://127.0.0.1:16780 --daemon-url http://127.0.0.1:16770
```

Build reads `images/` and a verified public program snapshot (`--artifacts` selects
one explicitly). It excludes credentials, checks versions and hashes, and prints
local image digests. Update the pins and snapshot together for an upgrade, then
set the desired image references in `config.sops.env` and recreate with Compose.

For **new** daemon data only, create the two named volumes, select them in the
encrypted environment, then run this from the repository root before starting:

```sh
just compose paseo --podman-run-args='--cap-add=CHOWN --cap-add=FOWNER --cap-add=DAC_OVERRIDE' \
  run --rm --user 0 --entrypoint /opt/infra/initialize demo --new
```

The one-off initializer needs ownership permissions; the daemon drops them.
Existing volumes need no initialization or data copy. Keep their ownership and
contents. Use native Codex login and Paseo pairing in the selected container.
[Takeover](../../docs/migration.md) covers backup and exclusive access to the data.

Verify reads `config.sops.env` (`--secret-file` selects another file). It checks
HTTP authentication and a native authenticated WebSocket session list, without a
model request or host login changes. `--password-key` and `--hostnames-key` select
another daemon's environment keys. Allowed hostnames and browser origins belong
to the app's environment; Gateway does not change them.
