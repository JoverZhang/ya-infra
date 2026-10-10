# Paseo instances

Each instance runs a separate Paseo daemon and Codex CLI in a rootless container.
The image is shared; the home, workspace, authentication cache, password, network
and listening port are separate. The host's existing Paseo at port 6767 is not
managed by these commands.

Deployment settings live in the invocation directory's single `./infra.toml`:
`[services.paseo.versions]` pins programs and images, and
`[services.paseo.instances.<name>]` defines each environment. Run commands from
that directory, including when using an external justfile with `just -f`.

## Deployment

```bash
cd ~/infra
just svc paseo build
just svc paseo deploy demo
just svc paseo status
```

The deployed versions are Paseo **0.9.1**, Codex **0.159.2** and Node **25.9.0**.
The build snapshots this host's installed public program distributions, including
Codex's execution helper and bundled resources, onto a cached digest-pinned base.
Program artifacts and their SHA-256 manifest live under `~/.cache/infra/paseo/`;
they are checked before every build. No host login, configuration or conversation
data is included. The build also checks program execution as the runtime user.

This first build uses local artifacts because the upstream image download was
too slow. Rebuilding elsewhere requires these pinned installed distributions or
the verified artifact cache, plus the pinned base image. It is not a standalone
network-only installer. The resulting container includes Git, Python and Node.
Voice and dictation are disabled for this initial text trial.
Paseo's relay is disabled; remote access uses a direct Cloudflare Tunnel route.

The initial instance listens at `http://127.0.0.1:16770`.
In the browser, add the daemon as a direct connection
using the corresponding password. Passwords are generated once, kept in private
files, and never printed by the deployment command:

```bash
cat ~/.config/infra/paseo/demo/password
```

`deploy` checks resource ownership labels and refuses to take over unrelated
containers, volumes or networks. An unchanged deployment is a no-op; changed
configuration replaces only the selected instance. Its two named volumes are
kept. If the replacement fails to start, the previous container is restarted.
This rollback does not undo application migrations to persistent data.

`--restart=always` uses the existing generic Podman restart service at boot.
`stop` stops the instance now and keeps its data; deployment or the next boot
can start it again.

## Codex login

All initial instances may use the owner's ChatGPT account. Each instance must
complete its own login to obtain a separate refreshable authentication session:

```bash
just svc paseo login demo
just svc paseo auth-status demo
```

The login wizard opens the device authentication page. Enter the one-time code
printed by Codex and sign in to the owner's account. The CLI writes credentials
directly to the selected instance's home volume. Do not copy the owner's whole
`.codex` directory or distribute copies of one refresh-token session to parallel
instances. Model usage and account permissions still belong to the shared account.

The [OpenAI authentication guide](https://learn.chatgpt.com/docs/auth) documents
device login. The [auth maintenance guide](https://learn.chatgpt.com/docs/auth/ci-cd-auth)
explains token-refresh conflicts and why refreshed caches must be persisted.

## Add a family instance

Add an instance table to `./infra.toml` and choose an unused port. Names use
lowercase letters, digits and single hyphens. For example:

```toml
[services.paseo.instances.family]
port = 16772
cpus = 2
memory = "4g"
pids_limit = 512
```

Then run `just svc paseo deploy <name>` and `just svc paseo login <name>`. The two volumes
are `infra-paseo-<name>-home` and `infra-paseo-<name>-workspace`. Browser files live
under `/workspace`; local Paseo and Codex sessions live under `/home/paseo`.

For public access, add a Route table in the same file:

```toml
[gateway.routes.paseo2]
gateway = "public"
hostname = "paseo2.example.com"
backend = "paseo/family"
```

Then run `just gateway plan paseo2` and `just gateway apply paseo2`. Hostnames are
owned by Route declarations; deployment derives `PASEO_HOSTNAMES` from every
Route for that instance. Its port and resource settings live in the instance table.
The shared gateway credential is reused for future family instances.

## Runtime isolation

The daemon runs as uid/gid 1000:1000, with a read-only image filesystem, no Linux
capabilities and `no-new-privileges`. Only its home and workspace volumes are
mounted writable. `/tmp` is temporary. Volume permissions are initialized by a
separate trusted helper without networking. No host directories or engine socket
are mounted into the daemon.

Each instance has its own bridge network with `isolate=strict`. Container-side
blackhole routes block private LAN, link-local and CGNAT destinations; the
connected instance subnet remains reachable for its gateway and DNS. These rules
are a targeted protection for this host's internal services, not a complete
Internet egress allowlist or VM isolation. Runtime verification checks gateway,
host and peer reachability as well as public HTTPS connectivity.

Codex uses `danger-full-access` inside this outer container and does not request
command approvals. Its basic configuration is initialized once in the home
volume. It does not inherit the host's skills, MCP configuration or local history.

## Operations and upgrades

```bash
just svc paseo logs demo
just svc paseo stop demo
just svc paseo restart demo
just svc paseo verify
just svc paseo smoke demo
```

`verify` checks runtime restrictions, password authentication, host reachability
and restart persistence. It restarts `demo`. If additional instances are running,
it also checks their distinct volumes, passwords and peer reachability. The
initial deployment has one instance, so two-instance runtime isolation has not
yet been exercised. `smoke` uses the logged-in Codex account through Paseo and
verifies a real workspace file; it consumes a small model turn and removes its
temporary file afterwards.

`[services.paseo.versions]` records all program versions and the base digest. Build a new explicitly
named image and deploy it to `demo` first. Set an instance's optional `image_ref`
to keep or select a particular image version. Deploy the other instances after
validation. Rebuilding/replacing a container never deletes its data volumes.

Backing up both named volumes also backs up login credentials. Keep backups in
private storage outside this repository; stop the selected instance during a
volume export for a consistent backup. An old restored login cache may need a new
login. Volume deletion is deliberately not an operation exposed by this module.

## Remote browser access

Ports bind to host loopback. The [gateway](../../gateway/README.md) resolves the
appropriate port, deploys the backend with its declared hostnames, and configures
the Tunnel route and DNS. Each instance retains its own password. The initial
declaration is `paseo1.example.com → paseo/demo`; publishing it requires the
gateway's management credential.

Run `just svc paseo pair demo` and paste its single output line into **Direct
connection → Advanced → Connection URI** on the shared
[paseo.example.com](https://paseo.example.com) website. Press **Enter** to apply the
URI, then select **Connect**. The link includes that instance's public host, TLS
and password. Use the instance name for
each family environment; `--hostname <hostname>` selects among aliases.
Omit `demo` to choose from a menu, or use `--list` to see environments, public
hosts, local running status and reasons a link cannot yet be generated. An Enter
default is offered only for a unique target; multiple targets require a choice.
Only the chosen link is printed to stdout. Listing does not reveal passwords.

For manual **Direct connection** setup, add Host
`paseo1.example.com`, Port `443`, **Use SSL** enabled, with the `demo` password from
`~/.config/infra/paseo/demo/password`. The shared UI has its own
[deployment module](../paseo-web/README.md). Connection settings are saved in each
browser; sessions and files belong to the backend environment. Existing
daemon-served pages can still connect to their own environment.

Each deployment derives `PASEO_CORS_ORIGINS` from the declared shared UI Routes
and its own HTTPS hostnames. Redeploy an environment after changing those Routes
to apply its new origin list. Password admission and host checks still apply.

This backend exports `endpoint <instance>` as JSON containing `url` and
`health_path`, and `verify-public <instance> --hostname <declared-hostname>` for
HTTPS and WebSocket checks. With one Route, `just svc paseo verify-public demo` is a
convenience command; multiple Routes should be verified by gateway Route name.

Sources: [Paseo Docker deployment](https://github.com/getpaseo/paseo/blob/main/public-docs/docker.md),
[Paseo web UI](https://paseo.sh/docs/web-ui),
[Podman networking](https://docs.podman.io/en/stable/markdown/podman-network-create.1.html).
