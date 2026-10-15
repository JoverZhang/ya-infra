# Agent entry

Read [README.md](README.md) once, then load only the documents relevant to the
current task. Code, `config.example.toml` and module `--help` own field schemas,
argument lists and live values.

| Task | Read |
| --- | --- |
| Instances, login, backend image upgrades | [Paseo](services/paseo/README.md) |
| Browser connections, shared UI, frontend upgrades | [Web](services/paseo-web/README.md) |
| Routes, DNS, management-token failures | [Gateway](gateway/README.md) |
| Connector adoption, cutover, cloudflared upgrades | [Tunnel](services/cloudflare-tunnel/README.md) |
| Choosing verification or evaluating a change | [Checks](checks/README.md) |

Keep each decision or operational caveat in its owning document. Add a pointer
when a task needs it; use source/config/help for inexpensive lookups. Public
historical evidence belongs in Git history, outside the normal reading path.

User-level state holds application state and locks. Keep task artifacts in
the project's ignored `.infra/artifacts/` and development backups in
`.infra/backups/`; use temporary directories for intermediate work.
