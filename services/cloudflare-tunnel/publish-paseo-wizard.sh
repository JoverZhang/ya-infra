#!/usr/bin/env bash
# Compatibility entry point. Hostnames are declared only in gateway/routes.
set -euo pipefail
if [[ $# -ne 2 ]]; then
  printf 'Usage: %s INSTANCE HOSTNAME\n' "$0" >&2
  exit 2
fi
infra_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
exec python3 "$infra_root/gateway/manage" legacy-publish "$1" "$2"
