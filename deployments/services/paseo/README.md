# Paseo environments

## Instances

Add `[services.paseo.instances.<name>]` to the invocation configuration, using
`config.example.toml` for its shape and an unused loopback port. Deploy and run
`login <name>` separately for every instance, even when sharing one ChatGPT
account: each home needs its own refreshable login session; cloning a live
authentication cache creates refresh conflicts. Account usage remains shared.

Home/workspace volumes persist across container replacement. Instance names
select those volumes, so renaming an instance creates a different environment.
For public access, add its [Route](../../gateway/README.md#routes).

`just svc paseo volumes <name>` checks the mounts and records volume identity in
user-level runtime state without restarting the instance. Deploy/restart refuse
missing or recreated volumes; only a new environment gets empty volumes.
Keep this identity record when moving the deployment. These guards do not prevent
direct Podman deletion or deletion of files by an agent: export both volumes with
`podman volume export` while stopped, keep archives private (home includes login
credentials), and retain a copy outside the host.

## Image upgrades

1. Prepare matching installed Node, Paseo and complete standalone Codex
   distributions. [prepare-artifacts](prepare-artifacts) defines the accepted
   layout and verifies snapshots; building currently requires those local
   distributions or an existing verified cache.
2. Update the version pins and assign new backend/shared-UI image tags. Pin
   other instances' `image_ref` to their old backend image during the trial.
3. Back up the selected instance as above. Build both images,
   deploy `demo` and the shared UI following its [client policy](../paseo-web/README.md#frontend-upgrades),
   then run the relevant [checks](../../checks/README.md).
   The shared frontend is global: check its compatibility with still-old backends
   before rolling out the remaining instances.
4. Roll back by restoring prior image references and deploying. Container startup
   rollback preserves volumes but cannot undo application data migrations;
   a migrated database may require the matching volume backup.
