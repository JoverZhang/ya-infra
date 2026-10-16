#!/usr/bin/env sh
set -eu

# The desktop's unprivileged user must be able to traverse the image root.
chmod 0755 /

# Chromium binds CDP to loopback; relay it to the published container port.
socat \
  TCP-LISTEN:${BROWSERLAB_CDP_RELAY_PORT:-9223},fork,reuseaddr,bind=0.0.0.0 \
  TCP:127.0.0.1:${BROWSERLAB_CDP_CHROME_PORT:-9222} &

# Repair profile ownership retained from earlier rootless UID mappings.
chown -R "${PUID:-911}:${PGID:-911}" /config

exec /init "$@"
