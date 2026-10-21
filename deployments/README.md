# Deployments

Use native Kubernetes resources for applications. Use the Go CLI for host bootstrap,
kubectl forwarding, and Cloudflare routes. [AGENTS.md](AGENTS.md) routes task-specific reading.

## Start

Install Go and just. The Go module pins the toolchain; bootstrap pins client tools.
Run commands from the directory that will contain your entry configuration.

```sh
just bootstrap
# Add the printed client-tools directory to PATH in your shell profile.
age-keygen -o /private/path/age.key
export SOPS_AGE_KEY_FILE=/private/path/age.key
```

Set the public age recipient in [`.sops.yaml`](.sops.yaml). Keep the private key outside Git.
The repository supplies `*.example.yaml`; create each required `*.sops.yaml` locally.
Encrypted files can be versioned after a privacy review. Public examples are not live configuration.

```sh
sops --config deployments/.sops.yaml encrypt \
  --filename-override config.sops.yaml \
  --output config.sops.yaml deployments/config.example.yaml
sops config.sops.yaml
```

Only `./config.sops.yaml` is read. `source` is relative to the invocation directory.
Use the same encryption command with the owning example and output name for app,
Gateway, and local-server inputs. Edit sensitive values through SOPS.

```sh
just bootstrap --help
just bootstrap --local              # Optional local system service; requires sudo.
just kubectl get deploy,pod,svc,pvc -A
just gateway --help
```

An existing cluster needs only client tools and a named kubeconfig context. Select
its storage class in the app manifests. The local cluster Kustomization includes
local storage and the connector; it is not a prerequisite for other clusters.
Local bootstrap checks services, ports, CIDRs, and a 2 GiB free-space floor. Plan
additional capacity for application data. Review the distribution's
[host requirements](https://docs.k3s.io/installation/requirements) before installation.

## Manage native resources

```sh
just kubectl diff --server-side -k deployments/examples/app/kubernetes
just kubectl apply --server-side -k deployments/examples/app/kubernetes
just kubectl -n apps rollout status deployment/app-web
```

Create the `apps` namespace before using the example. `kubectl` preserves native
arguments, streams, terminal access, and exit status. An explicit `--context`
overrides the configured default. Hooks, decryption of resources, and readiness
checks are separate operations.

For encrypted applications, standalone Kustomize runs KSOPS locally. `ksops.yaml`
is a generator declaration. It reads encrypted native Secrets (`files`) or packs
an encrypted app file into a Secret (`secretFrom`). It is never an API resource.

```sh
umask 077
mktemp -d                          # Use the returned directory as <private-temp>.
kustomize build --enable-alpha-plugins --enable-exec \
  deployments/apps/tbg/kubernetes > <private-temp>/resources.yaml
just kubectl diff --server-side -f <private-temp>/resources.yaml
just kubectl apply --server-side -f <private-temp>/resources.yaml
just kubectl -n apps rollout status deployment/tbg
rm <private-temp>/resources.yaml
rmdir <private-temp>
```

Run the next command only after checking the preceding result. Stop if rendering
fails. Diff returns 0 for no change, 1 for changes, and greater values for errors.
Use the same completed manifest for diff and apply; keep native Secret masking
enabled. Remove the private directory after success, failure, or interruption.
The [integration checks](checks/README.md) automate this sequence with temporary
cleanup. Secret content hashes update Pod references; re-encryption alone has no effect.

Normal apply does not prune removed resources. Use explicit native deletion.
Application hooks are built from `apps/<app>/main.go` and run explicitly; consult
that app's help. [Migration](docs/migration.md) owns data, image import, cutover,
and rollback procedures. The included migration workloads start with zero replicas.
