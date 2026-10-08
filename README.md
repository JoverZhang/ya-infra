# User infrastructure

Deployment definitions for this Linux user. Commands use the existing rootless
Podman socket at `/run/user/$(id -u)/podman/podman.sock`; no second engine is created.

```bash
cd ~/infra
just --list
just build paseo
just deploy paseo demo
just status paseo
just verify paseo
just smoke paseo demo
just status cloudflare-tunnel
```

The initial deployment is the single `demo` instance. Additional instances use
the same template and separate data volumes. Each service owns its deployment
script, pinned versions and documentation under `services/<name>/`; the root
`justfile` supplies the common command interface. Browserlab migration will be
discussed separately.

| Definition | Location |
| --- | --- |
| Paseo build and instance configuration | `services/paseo/` |
| Cloudflare Tunnel connector configuration | `services/cloudflare-tunnel/` |
| Instance passwords | `~/.config/infra/paseo/<instance>/password` |
| Cloudflare connector credential | `~/.config/infra/cloudflare-tunnel/token` |
| Persistent home and workspace | Named Podman volumes, one pair per instance |
| Management locks | `~/.local/state/infra/paseo/` |

Only definitions and documentation belong in Git. Secrets, login caches and
application data stay outside this repository. The Podman socket and generic
`podman-restart.service` are already enabled, and this user has lingering enabled.

See [Paseo operations](services/paseo/README.md) and
[Cloudflare Tunnel operations](services/cloudflare-tunnel/README.md) for setup,
verification and recovery. Commit reviewed definition changes locally; secrets
and application state are never copied into this repository.
