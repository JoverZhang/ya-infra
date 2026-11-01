# Cloudflare Gateway

Go manages DNS and remotely managed Tunnel ingress. Rootless Podman Compose runs
cloudflared. Gateway commands read only `./config.sops.yaml` in the invocation
directory.

## Configure and run

Install Podman, **podman-compose**, SOPS, Go, and just. `podman compose` delegates to
an external provider; select podman-compose because this file uses native Podman
external secrets. Use the same provider for all Compose operations.

Use the [SOPS policy setup](../README.md#start-and-operate). Keep the private key outside Git.
Encrypt [config.example.yaml](config.example.yaml) as `config.sops.yaml` in your
deployment directory. The example contains both credentials: the management API
token is used only by Go; `tunnel_token` is used only by the connector.

```sh
sops --config .sops.yaml encrypt \
  --filename-override config.sops.yaml \
  --output config.sops.yaml deployments/gateway/config.example.yaml
just gateway setup --edit
just gateway check-credentials
just gateway diff
just gateway prepare
export PODMAN_COMPOSE_PROVIDER=podman-compose
podman compose -f deployments/gateway/compose.yaml up -d
podman compose -f deployments/gateway/compose.yaml ps
just gateway verify
```

`prepare [gateway]` writes `ya-infra-gateway-<name>` to the rootless Podman secret
store through stdin. It leaves an unchanged secret intact and refuses an unowned
same-name secret. Runtime secrets are decrypted data in Podman's store; they are
not plaintext files in the checkout. After token rotation, run prepare and
`podman compose -f deployments/gateway/compose.yaml up -d --force-recreate`;
existing containers keep their old secret until recreated. Removing a declaration
does not delete its runtime secret. Remove unused secrets explicitly with Podman.

The Compose file supplies the `public` connector. Add native services and external
secret entries for additional Gateways, with separate loopback metrics ports.
Keep the image digest pinned. An image upgrade changes Compose, then recreates
and verifies the connector; it does not change Cloudflare routes.

Routes use a Gateway name and an explicit HTTP(S) origin. Multiple hostnames can
use one origin; each Tunnel has one Gateway owner. `apply [hostname]` changes only
owned remote resources. Omission preserves them; `state: absent` requests deletion.
`status` reports observed routes; Compose `ps` reports containers. `verify` checks
public HTTP; use the app's own check for authenticated WebSocket behavior.

## Boot and takeover

Use [native runtime recovery](../README.md#restart-and-boot). Prepare the connector
secret before the first Compose start and after token changes; existing containers
restart with their stored secret. Boot does not need the management API or SOPS.

For the former connector, first preserve its inspect output, restart policy, token,
Gateway state, and encrypted input in a private backup. Start the Compose connector
alongside it: the supplied name and metrics port 20243 differ from the old defaults.
Host networking retains loopback origins. Test **every** current Tunnel origin,
including unmanaged routes, from that network context. Check new container health,
public routes, and an unchanged Gateway diff before disabling the old restart owner
and stopping the old connector. Keep it for recovery. To roll back, stop the new
Compose connector, restore the old restart policy, and start the old connector.
Inspect diff again if any origin changed after takeover.

For a previous two-file Gateway input, copy the decrypted `gateways` and `routes`
into the new encrypted entry and add each connector token. Keep all identities,
hostnames, origins, and the ownership state unchanged. Perform this with private
files or in memory; never print credentials. Retire the former entry files only
after the new input and takeover have been verified.

## Ownership and recovery

State remains at `$XDG_STATE_HOME/infra/gateway/<name>/` (default `~/.local/state`).
Keep `manage.lock`, `routes.json`, and `before-last-apply.json` when moving a
deployment. State proves ownership; matching remote values do not. Preparation
and remote apply share the Gateway lock.

Apply rechecks remote state before each write, preserves unrelated rules and
unknown options, and reads the result back. Tunnel and DNS writes are separate.
The pending record and DNS comment identify an interrupted creation. Retry the
same declaration after resolving the error. Inspect concurrent remote edits;
keep ownership records when resolving conflicts.

Back up state and encrypted input before route changes. For rollback, restore the
former origin in the declaration, inspect diff, then apply. Restore only managed
routes, not an entire old Tunnel configuration over another operator's changes.
`check-credentials` tests reads; an authorized apply tests writes. A failed read is
unknown, never evidence that a resource is absent.
