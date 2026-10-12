# Shared Paseo client

## Connections

The shared UI serves static client assets; the selected daemon owns sessions and
files. Everyone connected to the same daemon shares that environment. Saved
connections/passwords belong to the browser's localStorage origin, so changing
website origins does not migrate them.

Generate a URI with `just svc paseo pair <instance>`. It carries the instance
password: capture it for checks without logging it. In the native UI, use
**Direct connection → Advanced → Connection URI**, paste, press **Enter** to
apply the fields, then **Connect**. **Paste pairing link** handles relay offers.

Use separate daemon hostnames for the native `/ws` flow. Path multiplexing
requires client/protocol support. The shared website itself serves no daemon
endpoints. Changing a shared-UI Route requires redeploying affected backends to
regenerate their exact allowed browser Origins.

## Frontend upgrades

Keep official bundled assets unchanged, using native connection import to avoid
maintaining a client patch. When changing the Paseo version, follow the
[Paseo upgrade guide](../paseo/README.md#image-upgrades).
For an NGINX-only upgrade, change its pinned base digest and image tag, rebuild
and deploy the shared UI, then verify static serving and public backend access
using [Checks](../../checks/README.md).
