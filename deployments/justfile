set shell := ["bash", "-euo", "pipefail", "-c"]
set positional-arguments

[private]
[no-cd]
default:
    @python3 {{quote(justfile_directory() / "bin/svc")}}

# Discover services, show their help, or run an action.
[no-cd]
svc service="" action="" *args:
    @python3 {{quote(justfile_directory() / "bin/svc")}} "$@"

# Manage public hostname routes.
[no-cd]
gateway action="" *args:
    @if [[ -z "$1" ]]; then python3 {{quote(justfile_directory() / "gateway/manage")}} --help; else python3 {{quote(justfile_directory() / "gateway/manage")}} "$@"; fi
