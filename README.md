# User infrastructure

Deployment definitions for this Linux user. Commands use the existing rootless
Podman socket at `/run/user/$(id -u)/podman/podman.sock`; no second engine is created.

```bash
cd ~/infra
just --list
just svc
just svc paseo
just svc paseo build
just svc paseo deploy demo
just svc paseo status
just svc paseo pair demo
just svc paseo verify
just svc paseo smoke demo
just svc cloudflare-tunnel status
just gateway plan paseo1
just gateway status paseo1
```

The initial deployment is the single `demo` instance. Additional instances use
the same template and separate data volumes. Each service owns its deployment
script and documentation under `services/<name>/`; the root
`justfile` supplies the common command interface. Browserlab migration will be
discussed separately.

The two public entrypoints are `just svc <service> <action> [instance] [options]`
and `just gateway <action> [route]`. `bin/svc` discovers each module's `manage`
entrypoint and forwards its arguments without shell interpolation. Each module
owns its actions and help. Omitting the service lists available modules; omitting
the action shows that module's help. Service actions live under `svc`.

All deployment settings come from a single **`./infra.toml` in the directory
where you run the command**. The Just recipes keep that working directory even
when you use `-f` to select this repository's justfile. No parent-directory,
repository or home-directory configuration fallback is used. Missing files and
invalid TOML produce an error. Root command help does not require configuration.

To prepare another project directory:

```bash
cd /path/to/project
cp ~/infra/infra.example.toml ./infra.toml
chmod 600 ./infra.toml
# Edit the gateway identifiers, hostnames, ports and versions before deployment.
just -f ~/infra/justfile svc paseo pair --list
just -f ~/infra/justfile gateway plan paseo1
```

`infra.toml` is ignored by this repository; add it to your project's `.gitignore`
as well. `infra.example.toml` is the public template. Each command reads the
file afresh. Existing service names, container names, credential locations and
state locations are retained; choosing a file does not create a separate engine
or deployment namespace. Image-internal Codex and NGINX templates remain with
their modules. Tokens, passwords and login caches are private runtime files.

`just svc paseo pair` lists eligible environments and prompts for a number or
name. `just svc paseo pair --list` lists metadata without credentials;
`just svc paseo pair demo` prints just one connection link. Only a single target
can be selected with Enter; multiple targets require an explicit choice.

| Definition | Location |
| --- | --- |
| Paseo pinned build versions | `infra.toml`: `[services.paseo.versions]` |
| Paseo instance ports and resource limits | `infra.toml`: `[services.paseo.instances.<name>]` |
| Shared Paseo UI image and port | `infra.toml`: `[services.paseo-web]` |
| Cloudflare connector image and resource limits | `infra.toml`: `[services.cloudflare-tunnel]` |
| Shared public gateway | `infra.toml`: `[gateway.public]` |
| Hostname routes | `infra.toml`: `[gateway.routes.<name>]` |
| Instance passwords | `~/.config/infra/paseo/<instance>/password` |
| Cloudflare connector credential | `~/.config/infra/cloudflare-tunnel/token` |
| Cloudflare management API credential | `~/.config/infra/gateway/public/credentials.env` |
| Persistent home and workspace | Named Podman volumes, one pair per instance |
| Management locks | `~/.local/state/infra/paseo/` |
| Gateway publication state and last configuration snapshot | `~/.local/state/infra/gateway/public/` |

Only code, example configuration and documentation belong in Git. Real deployment
settings stay in the local ignored file; secrets, login caches and application
data stay outside Git. The Podman socket and generic
`podman-restart.service` are already enabled, and this user has lingering enabled.

The initial public Route is `paseo1.example.com → paseo/demo`. Its hostname is
declared in `infra.toml` under `[gateway.routes.paseo1]`; the backend exports its
loopback port. The shared Cloudflare Tunnel carries traffic for all routes. This is a
local control module with Gateway/Route concepts; it adds no proxy or Kubernetes
runtime. Backend instances keep their own files, sessions, login and password.

The shared browser entry is [paseo.example.com](https://paseo.example.com). Run
`just svc paseo pair demo` and paste its one-line output into **Direct connection
→ Advanced → Connection URI**. Press **Enter** to apply the URI, then select
**Connect**. It includes the declared public hostname, port 443, TLS and that
instance's password. The shared UI serves unmodified official Paseo assets and
keeps each browser's connection catalog; the selected environment holds sessions
and files. Treat the generated link as the instance
password. See [shared UI operations](services/paseo-web/README.md).

With `infra.toml` prepared and the required program distributions installed,
set up the management credential once, then publish:

```bash
just gateway setup
just gateway apply paseo1
just svc paseo-web build
just gateway apply paseo-web
```

The recovered connector token already runs the tunnel. Publishing routes also
requires a scoped management API token; the setup wizard captures it privately.
Without that credential, `plan` shows the desired route and reports remote state
as unknown. Public availability is confirmed only after HTTPS, password and
WebSocket checks succeed.

See [Gateway operations](gateway/README.md),
[Paseo operations](services/paseo/README.md) and
[Cloudflare Tunnel operations](services/cloudflare-tunnel/README.md) for setup,
verification and recovery. Commit reviewed code and example changes locally; secrets
and application state are never copied into this repository.
