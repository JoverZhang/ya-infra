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
just gateway plan paseo1
just gateway status paseo1
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
| Shared public gateway and hostname routes | `gateway/public.toml`, `gateway/routes/` |
| Instance passwords | `~/.config/infra/paseo/<instance>/password` |
| Cloudflare connector credential | `~/.config/infra/cloudflare-tunnel/token` |
| Cloudflare management API credential | `~/.config/infra/gateway/public/credentials.env` |
| Persistent home and workspace | Named Podman volumes, one pair per instance |
| Management locks | `~/.local/state/infra/paseo/` |
| Gateway publication state and last configuration snapshot | `~/.local/state/infra/gateway/public/` |

Only definitions and documentation belong in Git. Secrets, login caches and
application data stay outside this repository. The Podman socket and generic
`podman-restart.service` are already enabled, and this user has lingering enabled.

The initial public Route is `paseo1.example.com → paseo/demo`. Its hostname is
declared only in `gateway/routes/paseo1.toml`; the backend exports its loopback
port. The shared Cloudflare Tunnel carries traffic for all routes. This is a
local control module with Gateway/Route concepts; it adds no proxy or Kubernetes
runtime. Backend instances keep their own files, sessions, login and password.

The first Route was published and verified on 2026-10-09. Open
[paseo1.example.com](https://paseo1.example.com) and connect with the `demo` password
from `~/.config/infra/paseo/demo/password`.

For a fresh installation, set up the management credential once, then publish:

```bash
just gateway setup
just gateway apply paseo1
```

The recovered connector token already runs the tunnel. Publishing routes also
requires a scoped management API token; the setup wizard captures it privately.
Without that credential, `plan` shows the desired route and reports remote state
as unknown. Public availability is confirmed only after HTTPS, password and
WebSocket checks succeed.

See [Gateway operations](gateway/README.md),
[Paseo operations](services/paseo/README.md) and
[Cloudflare Tunnel operations](services/cloudflare-tunnel/README.md) for setup,
verification and recovery. Commit reviewed definition changes locally; secrets
and application state are never copied into this repository.
