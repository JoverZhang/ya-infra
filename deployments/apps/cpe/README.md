# CPE

Compose runs the local delivery image, defaulting to `cpe:local`. The image owns
its binaries and Web assets. Mount the complete external `worker_scripts/`
directory readonly through `CPE_SCRIPTS_DIR`. Startup never builds or pulls an image.

Set `config.sops.env` from [the example](config.example.env). Store HDF5 under
`$XDG_DATA_HOME/ya-infra/cpe/data` (default `~/.local/share/ya-infra/cpe/data`), then
set its absolute path as `CPE_DATA_DIR`. Select all three existing external database
volumes. The prepared CPE database images use local `root`/`root` credentials;
their published ports are loopback only. Keep native CPE database URLs in sync
with these ports.

## Prepare configuration

Keep CPE's native TOML format. SOPS stores this unsupported document format as an
encrypted binary payload in `config.sops.json`. In the checkout root `.sops.yaml`,
set `path_regex` to `'\.sops\.(yaml|env|json)$'`; keep your existing age recipient.
Then run from this directory:

```sh
# Run once with an existing native config.toml; keep the source outside Git.
sops --config ../../../.sops.yaml encrypt --input-type binary --output-type json \
  --filename-override config.sops.json --output config.sops.json /path/to/config.toml

# Run after each config change, before creating or recreating application containers.
sops exec-file --output-type binary config.sops.json \
  'podman secret create --replace ya-infra-cpe-config {}'
```

SOPS passes decrypted TOML through a temporary FIFO. A decryption failure prevents
the secret command from running. Boot recovery reuses the stored Podman secret;
configuration changes require container recreation. If native `ENC(age:...)`
fields are used, also mount their configured key file readonly through a local
`compose.override.yaml`.

Daemon and Web mount the configuration secret and HDF5 readonly. For historical
Compare records with absolute file paths, add a readonly HDF5 bind at each original
container path in `compose.override.yaml`. The scripts mount also replaces image
references; keep references compatible with the image-built Procedure Graph assets. [Migration](../../docs/migration.md) covers backups and single-writer
ownership.

## Test scripts

This test environment uses a detached CPE worktree under `.sources/cpe/` at the
ya-infra root. Create it from the retained development repository:

```sh
git -C /path/to/cpe worktree add --detach /path/to/ya-infra/.sources/cpe origin/master
```

Set `CPE_SCRIPTS_DIR` to that worktree's `worker_scripts` directory. The worktree
shares Git metadata with the development repository; keep that repository available.
To update, fetch the repository, confirm the worktree is clean, and select a commit:

```sh
git -C .sources/cpe fetch origin
git -C .sources/cpe status --short
just compose cpe stop daemon web
git -C .sources/cpe switch --detach origin/master
just compose cpe up -d --no-deps daemon web
```

Stop daemon/Web before changing mounted scripts, then start them after updating.
Git does not publish a directory tree atomically. Check the selected revision's
image compatibility; a source update does not update application binaries or import
Algorithm records into the database. Other environments can select their own
external scripts directory.

## Operate and update

```sh
# Run from the ya-infra root.
just compose cpe up -d
just compose cpe ps
just compose cpe logs --tail 50

# Build the same revision selected for the scripts mount.
(cd .sources/cpe && just image build cpe:local)

# Run from the ya-infra root; leave database containers running.
just compose cpe up -d --no-deps --force-recreate daemon web
just compose cpe exec daemon cat /app/build.json
just compose cpe exec daemon cpe-client doctor
```

`restart` keeps the existing image. Recreate daemon and Web to use the new tag;
`build.json` records the actual source commit. Database/schema or Algorithm
identity changes still need the upstream release's migration steps.

Keep the existing Tunnel origin at the configured Web loopback port and confirm
proxy authentication. Database health gates Compose startup; daemon also waits
for `cpe-client doctor` because native boot recovery skips Compose health ordering.
