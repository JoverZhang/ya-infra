# Personal inbox deployment

Build from the repository root, then create the external state volume:

```sh
podman build -t localhost/personal-inbox:local -f apps/inbox/Containerfile apps/inbox
podman volume create personal-inbox-data
```

Use `config.example.yaml` as the shape of ignored `config.sops.yaml`. Choose a
random access password of at least 16 characters. The server reads its account
from `github_token` in the encrypted configuration. Never put tokens in browser
storage or URLs. Use a classic PAT with the scopes described in
[the application](../../../../apps/inbox/README.md). Local aliases can omit project names.

From this app directory, with the checkout root `.sops.yaml` age policy configured:

```sh
sops --config ../../../.sops.yaml encrypt --filename-override config.sops.yaml \
  --output config.sops.yaml config.example.yaml
sops config.sops.yaml
sops exec-file config.sops.yaml \
  'podman secret create --replace ya-infra-inbox-config {}'
```

An empty token permits local setup and displays a source-unavailable task; it does
not claim an empty GitHub inbox. Add the token with `sops config.sops.yaml`, prepare
the native config secret again and recreate the container.

Native secrets are readable runtime data in Podman's store. Updating a secret
requires recreating its consumer. `config.sops.env` optionally overrides Compose
image and external volume names; use `config.example.env` as its shape.

Start from the repository root with `just compose inbox up -d`; inspect native
Compose logs and open the configured loopback address. Both SQLite and the site
volume are external; Compose does not create or delete them implicitly. The site
volume must already exist, even if no site checks are configured. Back up SQLite
with a consistent SQLite backup and keep encrypted config with its age recovery key.

The shared site volume is mounted at `/repo` in both apps so git-sync's symlink
resolves correctly. Inbox uses host networking for the loopback site probes.
Before adding a Gateway route, choose its hostname and retain the Inbox login;
use Gateway's existing diff/apply workflow. No application systemd unit is needed.
