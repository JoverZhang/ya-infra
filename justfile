set shell := ["bash", "-euo", "pipefail", "-c"]
set positional-arguments

[private]
[no-cd]
default:
    @just --justfile {{quote(justfile_directory() / "deployments/justfile")}}

# Install pinned SOPS and age tools.
[no-cd]
bootstrap *args:
    @exec just --justfile {{quote(justfile_directory() / "deployments/justfile")}} bootstrap "$@"

# Forward native Compose arguments for an application.
[no-cd]
compose app *args:
    @exec just --justfile {{quote(justfile_directory() / "deployments/justfile")}} compose "$@"

# Manage Cloudflare routes.
[no-cd]
gateway *args:
    @exec just --justfile {{quote(justfile_directory() / "deployments/justfile")}} gateway "$@"
