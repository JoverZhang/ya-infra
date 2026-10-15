#!/usr/bin/env bash
set -euo pipefail
install -d -o 1000 -g 1000 -m 700 /home/paseo/.codex /home/paseo/.paseo
chown 1000:1000 /home/paseo /workspace
chmod 700 /home/paseo
# An optional trusted login helper may have bootstrapped the cache as root.
chown -R 1000:1000 /home/paseo/.codex
if [[ ! -f /home/paseo/.codex/config.toml ]]; then
    install -o 1000 -g 1000 -m 600 /opt/infra/codex.toml /home/paseo/.codex/config.toml
fi
