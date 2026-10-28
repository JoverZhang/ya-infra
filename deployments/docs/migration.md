# Migration from the Podman manager

This file owns live cutover and recovery. [Implementation #13](https://github.com/JoverZhang/ya-infra/issues/13)
tracks acceptance. Source refactoring does not prove a completed production migration.
The supplied workloads have zero replicas. Keep them stopped until their gates pass.

## Preserve recovery before replacing the active checkout

Keep revision `8b64260`, its private TOML input, Gateway state, unit definitions,
and exact images. Create a separate checkout or Git bundle and verify it can be
read. Keep the primary checkout on that revision until startup owners no longer
call its scripts. A PR checkout can build and test without changing those owners.

Record these private values for each component outside Git:

| Record | Contents |
| --- | --- |
| Runtime | Container/image digest, unit name, restart owner, command, ports |
| Data | Source volume, container mount, logical UID/GID, target PVC |
| Configuration | Original private files, SOPS inputs, key recovery location |
| Recovery | Backup location/checksum, restore result, old startup command |
| Routes | Every Tunnel origin, including unmanaged rules, and ownership records |

Use a protected backup location separate from live PV data. Keep a copy off-host.
Copy rootless files through the container/user namespace so logical UIDs survive;
host subordinate UIDs are not portable. Test archives in disposable storage. Do
not start copied bot identities or Codex refresh credentials in parallel.

The target local PV root is the invoking user's absolute
`${XDG_DATA_HOME:-$HOME/.local/share}/ya-infra/pv` (use the default for relative XDG
values). Source cleanup must not remove it. Bootstrap rejects a later root change.
Retain protects a released PV; backups protect against lost or corrupt data.

## Prepare the target

1. Encrypt the entry, native server, and app inputs from their examples. Gateway
   has a separate [Compose procedure](../gateway/README.md). Keep old inputs for
   recovery. Check the selected context.
2. Run client bootstrap, then optional local bootstrap with sudo. Review ports,
   firewall, Pod/Service CIDRs, node resources, and storage placement first.
3. Prepare cluster namespaces and retained storage.
   Existing clusters use their own storage class and scheduling constraints.
4. Run the [public example and disposable retention check](../checks/README.md).
   Confirm the effective PV policy and rebind, not only the StorageClass setting.
5. Build only images with changed helpers; keep upstream versions unchanged. Export
   an OCI archive with `podman save --uncompressed --format oci-archive` and import
   it into the target runtime explicitly. Keep the original layer encoding so the
   manifest digest stays stable. For local k3s,
   use `sudo k3s ctr images import --digests <archive>`. Native image IDs from Podman are not
   cluster image references. Check the imported digest against every pinned reference.
   If the runtime registers only a tag, register its verified digest reference with
   `sudo k3s ctr images tag <tag> <repository>@<digest>` before starting the workload.
6. Start disposable workloads with new data and fake credentials. Check mounts,
   probes, signal handling, and the target CNI's network policy enforcement.

For an existing data set, create its PVC and use a one-shot restore Pod to trigger
`WaitForFirstConsumer` binding. The restore Pod mounts only that app's claims.
Restore and check data with the runtime's logical UID/GID. Keep application replicas
zero while this Pod owns the data. Stop the restore Pod before starting the app.
Write `.ya-infra-restored` in each restored Paseo volume or Browserlab profile only
after file counts, ownership, and expected state pass comparison. TBG instead checks
its database file; also run SQLite `PRAGMA integrity_check` before activation.

## Switch one component at a time

### Connector first

Use the [Gateway Compose takeover procedure](../gateway/README.md#boot-and-takeover).
Keep its origins unchanged until each application passes its own restore and
readiness checks. The connector now runs in rootless Podman, outside Kubernetes.

### Paseo

Start Web first and verify `/healthz` before switching only its route. Keep the
public Web and backend hostnames stable. For each daemon:

1. Stop and disable its old startup owner. Make the final home/workspace archives.
2. Restore its two claims. Compare sessions, files, credentials, ownership, and modes.
3. Remove the restore Pod. Set that daemon to one replica; render, diff, apply, wait.
4. Change only its Gateway route to its Service. Run its explicit verify hook.
5. Confirm an existing session and file access. Check two disposable daemons with
   separate new data for file/session isolation. Do not reuse a production login.

Native pairing and Web assets remain upstream behavior. Each daemon retains its own
login cache; shared account access does not mean sharing a writable home volume.

### TBG

Keep its host-network API and loopback endpoint. Run setup to reconcile the client
file; preserve notification bindings. Stop and disable the old bot unit before the
final SQLite backup. Restore into the claim, run `PRAGMA integrity_check`, and compare
expected existing records before starting exactly one replacement poller.

Set one replica, apply, wait, and run verify. Observe an ordinary notification to
prove delivery. The local HTTP 405 probe proves only protocol reachability. A test
notification requires a separate explicit request. Temporary port forwarding is
not a persistent host notification transport.

### Browserlab

Stop and disable the old browser before the final profile archive. Restore the
profile with its logical owner. Set the absolute read-only workspace path and
correct node selector. The browser stays in the Pod network; its separate relay
binds host desktop/CDP ports to `127.0.0.1`.

Start browser and relay after restore. Check desktop, `/json/version`, existing
browser tooling, profile state, the read-only workspace, and clean shutdown. Check
that other Pods cannot use CDP. Two browsers must never write the same profile.

## Recover a failed switch

Stop the replacement and its relays first. Preserve any new writes. Select and
record the recovery data point; do not silently use a stale backup. Restore the
old route origin before restarting an old connector that cannot resolve Service
DNS. Restart the old application from its preserved checkout/image and unit. Check
the same local/public behavior used for acceptance. Never run both writers or bot
pollers as a recovery test. Gateway state recovery is in [Gateway](../gateway/README.md).

Keep previous manifests and images for ordinary image rollback. Reapply the previous
manifest, wait, and verify. Database or profile format changes need the application's
own data recovery procedure. Keep application version upgrades separate from this move.

## Complete live acceptance

Record each component's restore, image import, one-writer cutover, app checks, and
recovery result in #13. During an agreed maintenance window, reboot and confirm
services/routes recover without a login. Back up the cluster datastore, server token,
configuration, keys, and application data; test isolated restoration using the
[distribution procedure](https://docs.k3s.io/datastore/backup-restore).

Retire old control paths only after their recovery gates pass. Remove legacy unit
startup separately from source changes. Keep unrelated Podman services intact.
Deleting old source never authorizes deleting volumes, profiles, images, or backups.
