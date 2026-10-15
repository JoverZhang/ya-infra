# Public gateway controller

## Routes

Declare `[gateway.routes.<name>]` with the backend's `module/instance` identity.
`plan <name>` previews remote changes; `apply <name>` deploys the backend and
publishes its exact hostname and proxied Tunnel CNAME. Applying all Routes puts
Paseo environments before the shared UI, whose check exercises those backends.
Conflicting DNS/origins are rejected. Removing a declaration leaves its remote
DNS and ingress intact; withdrawal needs an explicit change.

Cloudflare replaces the whole Tunnel configuration on update. The controller
merges only the selected Route, preserves unrelated rules/options, and rechecks
remote drift before writing. Tunnel and DNS writes are separate; there is no
transaction covering dashboard edits or these resources.

## Credentials

The connector token joins the Tunnel; Route/DNS management needs a separate API
token. `setup` owns the dashboard/token-creation procedure. `check-credentials`
separates validity, read permissions, resource scope and explicit IP filtering
failures. An active token can still be rejected by its client-IP rule; correct
that rule using the reported address rather than inferring missing permissions.
Edit permissions are exercised by `apply`, not proven by read checks.

## Recovery

`dns-pending`: retry `apply` after fixing DNS access. `published` with failing
public checks: retry `verify` after fixing delivery/authentication.
`status` reports the last verification, not proof of current public availability.
[Checks](../checks/README.md) explains verification effects.

Private pre-apply snapshots and Route state are managed by [controller.py](controller.py).
Review current remote state before restoring anything; a full snapshot restore
can discard unrelated dashboard changes. Live diagnostic/status failures are
rechecked when needed rather than recorded here as permanent project facts.
