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

# Update the configured CPE test deployment from its source worktree.
cpe-update:
    #!/usr/bin/env bash
    set -euo pipefail
    export PATH="${XDG_DATA_HOME:-$HOME/.local/share}/ya-infra/bin:$PATH"
    cpe_source="$(realpath -- {{quote(justfile_directory() / '.sources/cpe')}})"
    if [[ -n "$(git -C "$cpe_source" status --porcelain)" ]]; then
        echo 'CPE source has local changes; commit or discard them before updating.' >&2
        exit 1
    fi
    export CPE_UPDATE_SOURCE="$cpe_source"
    sops exec-env deployments/apps/cpe/config.sops.env 'test "${CPE_IMAGE:-cpe:local}" = cpe:local && test "$CPE_SCRIPTS_DIR" = "$CPE_UPDATE_SOURCE/worker_scripts"'
    git -C "$cpe_source" fetch origin
    just compose cpe stop daemon web
    git -C "$cpe_source" switch --detach origin/master
    if ! (cd "$cpe_source" && just image build cpe:local); then
        echo 'CPE build failed. Applications remain stopped; fix the error and rerun just cpe-update.' >&2
        exit 1
    fi
    just compose cpe up -d --no-deps --force-recreate daemon web
    just compose cpe exec -T daemon cpe-client doctor
    just compose cpe ps
