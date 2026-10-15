set shell := ["bash", "-euo", "pipefail", "-c"]
set positional-arguments

[private]
[no-cd]
default: svc

# Discover deployment services, show help, or run an action.
[no-cd]
svc *args:
    @just --justfile {{quote(justfile_directory() / "deployments/justfile")}} svc "$@"

# Manage public hostname routes.
[no-cd]
gateway *args:
    @just --justfile {{quote(justfile_directory() / "deployments/justfile")}} gateway "$@"
