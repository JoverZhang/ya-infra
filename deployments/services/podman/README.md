# User runtime

Run `just svc podman install` once for each Linux user. It enables the Podman
socket and boot service, and installs a drop-in that starts containers through
the socket. Container monitors then belong to the API service instead of the
boot task's failure cleanup. Errors from broken legacy containers remain visible.
The installer leaves running containers alone. Enable user lingering for boot
without an interactive login (`loginctl enable-linger "$USER"`).

## Native container units

TBG and Browserlab use Quadlet. `deploy` compiles the invocation directory's
`config.toml` into `~/.config/containers/systemd/infra-<module>.container` and
starts the resulting user service. Generated files are deployment artifacts;
edit the TOML source and redeploy. Units start independently at boot and do not
depend on a repository checkout. Module help owns the command list.

Legacy adoption requires a data backup, then stopping and renaming the old
container and setting its restart policy to `no`. Keep it for rollback until
the new deployment is verified. Deployment refuses an unmanaged name collision.

For upgrades, build/load the new image, pin its ID or registry digest in
`image_ref`, and deploy. Unchanged definitions preserve the running container.
A failed upgrade restores the previous unit definition; application data/schema
rollback requires the data backup. Snapshot valuable data before changing images.
