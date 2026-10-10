# User infrastructure

Rootless Podman deployments for isolated Paseo/Codex environments, with a shared
browser UI and public access through Cloudflare Tunnel. Browserlab remains
outside the managed scope.

## Configure and inspect

Commands read exactly `./infra.toml` in the invocation directory, including when
using an external justfile. Copy [infra.example.toml](infra.example.toml) there
and set the deployment values; this repository ignores the actual file.

```bash
just -f ~/infra/justfile svc
just -f ~/infra/justfile svc paseo status
just -f ~/infra/justfile svc paseo pair --list
```

The public interface is `just svc <module> <action> ...` and
`just gateway <action> ...`; module help supplies the available actions.
Switching configuration files retains the same Linux-user container names,
credentials and state namespace. Existing Podman socket, restart service and
user lingering are prerequisites. Python 3.11+, Bash and Just are required.

## Extend

- Add an environment through the [Paseo guide](services/paseo/README.md#instances).
- Add a module as `services/<name>/manage`; [bin/svc](bin/svc) discovers it and
  forwards arguments unchanged. Keep lifecycle actions and validation inside
  that module. For public routing, implement `endpoint`, `deploy` and
  `verify-public` against the interface enforced by [gateway/model.py](gateway/model.py).
- Add a public Route through the [Gateway guide](gateway/README.md#routes).

Start with the [agent task map](AGENTS.md) for task-specific reading. Use
[Checks](checks/README.md) to choose validation with appropriate side effects.
