# Browserlab

Browserlab uses the [native user-unit lifecycle](../podman/README.md#native-container-units).
The desktop and CDP relay bind only to loopback. Use SSH forwarding for remote
access. `verify` probes both the desktop and `/json/version`; use
`just svc browserlab exec opencli doctor` for in-container browser tooling.

`profile_dir` is the existing browser profile mounted at `/config`.
`workspace_dir` is mounted read-only at `/workspace/lab`. Both paths must exist;
preserve profile contents and ownership during migration. The entrypoint repairs
the LinuxServer user mapping in the profile before starting the desktop.

For an image upgrade, snapshot the profile while stopped, then build this directory
with `BROWSERLAB_BASE_IMAGE` set to a pinned LinuxServer Chromium digest and
`BROWSERLAB_OPENCLI_VERSION` set to the intended release:

```sh
podman --remote --url "unix://$XDG_RUNTIME_DIR/podman/podman.sock" build \
  --build-arg BASE_IMAGE="$BROWSERLAB_BASE_IMAGE" \
  --build-arg OPENCLI_VERSION="$BROWSERLAB_OPENCLI_VERSION" -t localhost/infra-browserlab:custom \
  deployments/services/browserlab
```

Set `image_ref` to the resulting immutable local image ID and deploy. Reuse the
same profile path and keep the previous image until desktop/CDP verification passes.
