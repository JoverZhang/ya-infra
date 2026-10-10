set shell := ["bash", "-euo", "pipefail", "-c"]
set positional-arguments

[private]
default:
    @python3 bin/svc

# Discover services, show their help, or run an action.
svc service="" action="" *args:
    @python3 bin/svc "$@"

# Manage public hostname routes.
gateway action="" *args:
    @if [[ -z "$1" ]]; then python3 gateway/manage --help; else python3 gateway/manage "$@"; fi
