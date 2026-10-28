# Backup, takeover, and recovery

## Before switching

Record the old image digest, container name, startup owner, mounts, logical UID/GID,
ports, and encrypted input. Keep private inspect output and credentials encrypted.
Save the old startup declaration so rollback does not depend on a running container.

Use an encrypted backup outside the live data directory, with an off-host copy of
the age key. Verify that the archive decrypts and contains the expected data before
stopping the old deployment. Stop writers for a final consistent archive. For TBG,
an online SQLite backup is also useful; run `PRAGMA integrity_check` on the snapshot.

Use the existing data directly during a same-host Compose takeover. Select the
exact external volume names or existing bind paths. Do not rename, recreate, or
recursively change ownership of working data. External volumes survive Compose
removal, but they still need backup against corruption or direct volume deletion.

## Switch one application

1. Prepare encrypted configuration and any explicit app setup. Check loopback
   bindings, image identity, existing mounts, and the backup.
2. Stop and disable or mask the old startup owner. Keep a copy of its declaration.
   For a standalone container, disable its restart policy before stopping it.
3. Start Compose with the existing data. Never run two writers on the same home,
   database, or browser profile. TBG must have only one Telegram poller.
4. Verify the app's local and public behavior, then enable its new user unit.
   Exercise unit restart; report separately whether a host reboot was tested.

| App | Preserve and check |
| --- | --- |
| Paseo | Both named volumes, Codex login, sessions, workspace files, password rejection, authenticated WebSocket access |
| TBG | SQLite integrity and binding counts, server config, host client endpoint; no unsolicited test messages |
| Browserlab | Profile data, UID/GID, desktop/CDP access, read-only workspace, loopback listeners |
| Gateway | Follow its [takeover procedure](../gateway/README.md#boot-and-takeover); preserve origins and ownership records |

## Roll back

Stop the new user unit first; its Compose down keeps external data and secrets.
Restore the old startup declaration and restart policy, then start the old owner.
Keep the same data if the app version and format did not change. If an upgrade
changed the data format, restore the verified backup before starting the old image.
Recheck local endpoints and public routes before retiring recovery artifacts.
