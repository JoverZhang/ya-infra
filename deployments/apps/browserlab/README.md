# Browserlab

The browser Pod owns a profile PVC and a read-only host workspace mount. Set the
absolute workspace path, logical PUID/PGID, and node selector before applying.
The separate host-network relay binds desktop and CDP to loopback. Keep the browser
in the Pod network and enforce the ingress NetworkPolicy on the target cluster.

```sh
go -C deployments build -o .bin/browserlab ./apps/browserlab
deployments/.bin/browserlab build
deployments/.bin/browserlab verify
```

The image recipe pins the existing browser base, Node package, and OpenCLI version.
The Go helper retains the required upstream D-Bus workaround, starts the upstream
supervisor, and provides the TCP relay. Build reports a local digest. Update every
reference in `deployment.yaml` and `relay.yaml` after image import; apply and wait
for both Deployments. Use alternate loopback URLs for disposable verification.

The profile init container requires a verified restore marker. [Migration](../../docs/migration.md)
owns profile restore, access checks, and shutdown order. Desktop or CDP must not
become a public host listener during migration.
