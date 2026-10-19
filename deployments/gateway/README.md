# Cloudflare Gateway

The CLI changes Cloudflare DNS and remotely managed Tunnel ingress. Kubernetes
runs the connector. See [config.example.yaml](config.example.yaml) for the schema.
Management and connector tokens have separate encrypted inputs.

```sh
just gateway setup                 # Required dashboard permissions.
just gateway setup --edit          # Edit gateway/config.sops.yaml with SOPS.
just gateway check-credentials
just gateway diff
just gateway apply app.example.com
just gateway status
just gateway verify app.example.com
```

Routes name a Gateway and an explicit origin URL. Multiple hostnames can use one
origin. Each Tunnel has one configured Gateway owner. `state: absent` requests
owned deletion; omission preserves remote resources. A failed read is unknown.
Verification checks the configured public HTTP path and status. Use the app's
check for authenticated WebSocket behavior.

## Ownership and recovery

The Go port retains the legacy location and record format:
`$XDG_STATE_HOME/infra/gateway/<name>/` (default `~/.local/state`). It contains
`manage.lock`, `routes.json`, and `before-last-apply.json`. Preserve this private
state when moving a deployment. It proves ownership; matching remote values do not.

Apply rechecks remote state before each write, preserves unrelated rules and
unknown options, and reads the result back. Tunnel and DNS writes are separate.
The pending record and DNS comment identify an interrupted creation. Retry the
same declaration after resolving the reported error. Concurrent remote edits
require inspection; do not erase ownership to bypass a conflict.

Back up state and desired encrypted input before a route change. The snapshot
contains the last pre-write Tunnel configuration and DNS observation. For rollback,
restore the former origin in the encrypted declaration, inspect diff, then apply.
Do not restore an entire old Tunnel configuration over another operator's changes.
Use the provider console to reconcile an ambiguous external edit and save the
verified ownership record before retrying.

`check-credentials` tests reads only. It reports endpoint-specific permissions and
IP-filter failures without provider message text or tokens. Writes are tested by
an authorized apply. [Migration](../docs/migration.md) owns connector network
compatibility and live cutover.
