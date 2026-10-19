# Deployments

Kubernetes runs applications; SOPS protects source configuration; Gateway manages Cloudflare.
Each `apps/<app>/` owns native resources, image inputs, and optional explicit Go hooks.

| Task | Read |
| --- | --- |
| First use, encrypted rendering | [README](README.md) |
| Add an application | [Example](examples/app/kubernetes/) |
| Host setup | [Bootstrap](bootstrap.go), [native server input](kubernetes/bootstrap/) |
| Image, data, or app hooks | The app's `README.md`, `main.go`, and `images/` |
| Tunnel or DNS | [Gateway](gateway/README.md) |
| Validation | [Checks](checks/README.md) |
| Existing deployment cutover or recovery | [Migration](docs/migration.md) |
