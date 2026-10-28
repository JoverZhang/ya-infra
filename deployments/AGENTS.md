# Deployments

Native Compose owns applications; SOPS protects configuration; Go owns Gateway and explicit app hooks.

| Task | Read |
| --- | --- |
| Operate, add an app, or enable startup | [README](README.md) |
| App configuration, image upgrades, or hooks | That app's `README.md`, `compose.yaml`, and `main.go` |
| Cloudflare routes or connector | [Gateway](gateway/README.md) |
| Data, takeover, or rollback | [Migration](docs/migration.md) |
| Validate a change | [Checks](checks/README.md) |
