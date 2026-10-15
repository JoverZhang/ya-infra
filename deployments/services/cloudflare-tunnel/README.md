# Cloudflared connector

This is the Tunnel data plane. Host networking preserves access to the existing
host-loopback origins; changing that network requires revisiting every origin.
Remote routing is managed by the [Gateway](../../gateway/README.md).

## Image upgrades

Update the pinned cloudflared digest/version, run `build` to pull that image,
then `deploy` with the existing connector credential. Check edge readiness and
[public delivery](../../checks/README.md) before considering the rollout complete.
Replacing the connector leaves Cloudflare-side routing in place.

## Adoption and cutover

`adopt` recovers the existing connector credential and records the legacy
container identity. Deployment retires that container only after the managed
connector is ready. Keep the stopped legacy container during cutover; rollback
can stop the managed connector and start the recorded legacy container, retaining
the current remote configuration.
