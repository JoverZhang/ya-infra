# Deployments

Rootless Podman Compose runs each `apps/<app>/compose.yaml`. SOPS protects source
configuration. [Gateway](gateway/README.md) owns Cloudflare DNS and Tunnel ingress.

## Start and operate

Install Go, just, Podman, and **podman-compose**. Run `just bootstrap` to install
pinned SOPS and age tools; add the printed directory to PATH. Copy
[`.sops.yaml`](.sops.yaml) to the checkout root and set your public age recipient
there. Keep the private key outside Git.

```sh
cd deployments/apps/paseo
sops --config ../../../.sops.yaml encrypt --filename-override config.sops.env \
  --output config.sops.env config.example.env
sops config.sops.env
cd ../../..
just compose paseo up -d
just compose paseo ps
just compose paseo logs --tail 50
```

The entry only selects the app directory and podman-compose. If `config.sops.env`
exists, SOPS supplies its values through the process environment. All remaining
arguments and exit codes belong to native Compose. Missing keys abort before
Compose runs. Native `config`, `inspect`, and verbose output can disclose values;
keep those outputs private. Local encrypted inputs are ignored in this public repo.

`config.sops.env` supplies Compose variables, such as image references, passwords,
ports, and host paths. An app can also own a native encrypted configuration file;
[TBG](apps/tbg/README.md) explains its server file and host client setup. Hooks are
explicit: run `go run . --help` from the app directory. Compose does not call them.

## Data and images

Named data volumes are external. Create new ones explicitly, or select existing
ones before startup. Missing bind sources fail instead of creating empty folders.
Compose `down -v` retains external volumes; direct Podman volume deletion does not.
[Backup and takeover](docs/migration.md) owns recovery and single-writer checks.

Build hooks print image digests. Set the chosen image in the app's encrypted
environment, recreate its containers, and run that app's verification. Existing
local images use `pull_policy: never`; a new host must build or load them first.

## Start at login or boot

[ya-infra-app@.service](ya-infra-app@.service) is a native systemd user template.
Set its checkout paths if your persistent checkout is not `~/ya-infra`. Complete
app setup first. The unit recreates containers at start and removes them at stop;
external data and secrets remain. Enable user lingering for startup before login.

SOPS must find its key without shell exports: use
`$XDG_CONFIG_HOME/sops/age/keys.txt` (default `~/.config/sops/age/keys.txt`, mode 0600),
or add `Environment=SOPS_AGE_KEY_FILE=/absolute/path/age.key` in a unit override.

```sh
install -Dm 0644 deployments/ya-infra-app@.service ~/.config/systemd/user/ya-infra-app@.service
systemctl --user daemon-reload
systemctl --user enable --now ya-infra-app@paseo.service
```

## Extend

Copy [the Compose example](examples/app/compose.yaml) into `apps/<name>/`, then set
its image, mounts, and loopback ports. Add `config.example.env` when interpolation
needs private values. Add `main.go` only for app-specific setup, build, or verification
that native tools do not cover. No central service registry needs an edit.

[Checks](checks/README.md) gives the repeatable validation commands.
