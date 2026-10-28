# Browserlab

Compose binds the existing profile directory and a read-only host workspace.
Set their absolute paths and the profile owner's numeric UID/GID in
`config.sops.env`. Desktop and CDP ports bind only to host loopback.
The native device entry keeps `/dev/dri` access; omit it on hosts without render devices.

Run hooks from this directory:

```sh
go run . build
go run . verify
```

The image keeps the upstream desktop supervisor and D-Bus workaround. The
supervisor also runs the small CDP relay; no separate proxy deployment is needed.
The recipe pins Chromium's base, Node, and OpenCLI. Build prints a local digest;
update `BROWSERLAB_IMAGE`, recreate with Compose, and verify both endpoints.
`verify` accepts alternate desktop/CDP URLs for disposable checks.

Close the old browser before attaching its profile. Preserve its UID/GID and
verify profile data and read-only workspace access after startup. See
[backup and takeover](../../docs/migration.md) for recovery.
