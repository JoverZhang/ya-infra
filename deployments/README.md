# User infrastructure

Rootless Podman deployments for Paseo/Codex, its shared browser UI and Cloudflare
gateway, plus the Telegram gateway and Browserlab.

## Configure and inspect

Commands read exactly `./config.toml` in the invocation directory, including when
using an external justfile. Copy [config.example.toml](config.example.toml) there
and set the deployment values; this repository ignores the actual file.

```bash
cp deployments/config.example.toml config.toml
# Edit config.toml before running deployment actions.
just svc
just svc paseo status
just svc paseo pair --list
just svc podman install
```

These examples run from the repository root. The root justfile forwards to this
module; running here directly or using `just -f /path/to/ya-infra/justfile ...`
also keeps the invocation directory. When adopting an existing deployment, copy
its private `infra.toml` to `config.toml` in that directory and keep its instance
names and user-level credentials/state intact.

The public interface is `just svc <module> <action> ...` and
`just gateway <action> ...`; module help supplies the available actions.
Switching configuration files retains the same Linux-user container names,
credentials and state namespace. The [runtime installer](services/podman/README.md)
configures socket-based boot recovery; user lingering must be enabled.
Python 3.11+, Bash and Just are required; tbg and Browserlab also use Podman Quadlet.

## Extend

- Add an environment through the [Paseo guide](services/paseo/README.md#instances).
- Add a module as `services/<name>/manage`; [bin/svc](bin/svc) discovers it and
  forwards arguments unchanged. Keep lifecycle actions and validation inside
  that module. For public routing, implement `endpoint`, `deploy` and
  `verify-public` against the interface enforced by [gateway/model.py](gateway/model.py).
- Add a public Route through the [Gateway guide](gateway/README.md#routes).

Start with the [agent task map](AGENTS.md) for task-specific reading. Use
[Checks](checks/README.md) to choose validation with appropriate side effects.
