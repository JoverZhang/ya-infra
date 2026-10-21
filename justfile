set shell := ["bash", "-euo", "pipefail", "-c"]
set positional-arguments

[private]
[no-cd]
default:
    @just --justfile {{quote(justfile_directory() / "deployments/justfile")}}

# Install client tools and optional local cluster prerequisites.
[no-cd]
bootstrap *args:
    @exec just --justfile {{quote(justfile_directory() / "deployments/justfile")}} bootstrap "$@"

# Forward native kubectl arguments.
[no-cd]
kubectl *args:
    @exec just --justfile {{quote(justfile_directory() / "deployments/justfile")}} kubectl "$@"

# Manage Cloudflare routes.
[no-cd]
gateway *args:
    @exec just --justfile {{quote(justfile_directory() / "deployments/justfile")}} gateway "$@"
