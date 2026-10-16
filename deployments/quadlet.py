"""Install native user container units; application definitions stay in modules."""
import argparse
import json
import os
from pathlib import Path
import re
import subprocess
import sys

from infra_config import section
import podman


def value(value):
    value = str(value)
    if any(character in value for character in "\n\r\0"):
        raise ValueError("Unit values must be single-line strings")
    return value.replace("%", "%%")


def quote(item):
    return json.dumps(value(item), ensure_ascii=False)


def path(value):
    return Path(value).expanduser().resolve()


def unit(service, config, options, checks):
    lines = ["# Managed by ya-infra; edit ./config.toml and run deploy.",
             "[Unit]", f"Description=Infra {service}", "StartLimitIntervalSec=60", "StartLimitBurst=3",
             "", "[Container]", "Image=" + value(config["image_ref"]),
             "ContainerName=" + value(config["container_name"]), "Pull=never", "ImageVolume=ignore"]
    labels = {"infra.managed": "true", "infra.service": service, "infra.owner": str(os.getuid())}
    lines += ["Label=" + key + "=" + item for key, item in labels.items()]
    lines += options + ["", "[Service]", "Restart=on-failure", "RestartSec=5", "TimeoutStartSec=90"]
    lines += ["ExecStartPre=" + " ".join(quote(arg) for arg in command) for command in checks]
    return "\n".join(lines + ["", "[Install]", "WantedBy=default.target", ""])


def main(service, definition, verify):
    parser = argparse.ArgumentParser(prog=f"just svc {service}")
    parser.add_argument("action", nargs="?", choices=["render", "deploy", "status", "verify", "logs", "stop", "restart", "exec"])
    parser.add_argument("args", nargs=argparse.REMAINDER)
    args = parser.parse_args()
    if args.action is None:
        parser.print_help()
        return
    if args.args and args.action != "exec":
        parser.error("Only exec accepts additional arguments")
    config = section("services", service)
    name = config["container_name"]
    if not re.fullmatch(r"[a-zA-Z0-9][a-zA-Z0-9_.-]*", name):
        raise ValueError("Invalid container_name")
    systemd = ["systemctl", "--user"]
    service_unit = f"infra-{service}.service"
    if args.action in ("status", "logs"):
        command = (systemd + ["is-active", service_unit] if args.action == "status" else
                   ["journalctl", "--user", "-u", service_unit, "--no-pager", "-n", "60"])
        subprocess.run(command, check=True)
        return
    if args.action in ("render", "deploy"):
        if not re.fullmatch(r"(?:sha256:|[^\s]+@sha256:)[a-f0-9]{64}", config["image_ref"]):
            raise ValueError("Pin image_ref to an immutable local image ID or registry digest")
        if podman.inspect(podman.engine, "image", config["image_ref"]) is None:
            raise ValueError("Pinned image is missing; build or load it before deployment")
        content = definition(config)
        if args.action == "render":
            print(content, end="")
            return
    existing = podman.inspect(podman.engine, "container", name)
    if existing:
        podman.require_owned(existing.get("Config", {}).get("Labels") or {},
                             {"infra.managed": "true", "infra.service": service, "infra.owner": str(os.getuid())},
                             "Existing container is not managed here; stop and rename it before migration")
    if args.action == "deploy":
        directory = Path(os.environ.get("XDG_CONFIG_HOME", str(Path.home() / ".config"))) / "containers/systemd"
        directory.mkdir(parents=True, exist_ok=True)
        target = directory / f"infra-{service}.container"
        previous = target.read_text() if target.exists() else None
        if previous is not None and not previous.startswith("# Managed by ya-infra;"):
            raise ValueError("Refusing to overwrite an unmanaged Quadlet file")
        active = subprocess.run(systemd + ["is-active", "--quiet", service_unit]).returncode == 0
        if previous != content or not active:
            def write(value):
                temporary = target.with_suffix(".tmp")
                temporary.write_text(value)
                temporary.chmod(0o600)
                temporary.replace(target)
                subprocess.run(systemd + ["daemon-reload"], check=True)
            try:
                write(content)
                subprocess.run(systemd + ["restart", service_unit], check=True)
                verify(config)
            except Exception:
                if previous is not None:
                    write(previous)
                    subprocess.run(systemd + ["restart", service_unit], check=False)
                else:
                    subprocess.run(systemd + ["stop", service_unit], check=False)
                    target.unlink()
                    subprocess.run(systemd + ["daemon-reload"], check=False)
                raise
    elif args.action == "exec":
        if not args.args:
            parser.error("exec requires a command")
        podman.engine("exec", name, *args.args, capture=False)
        return
    elif args.action in ("stop", "restart"):
        subprocess.run(systemd + [args.action, service_unit], check=True)
        if args.action == "stop":
            return
    else:
        subprocess.run(systemd + ["is-active", "--quiet", service_unit], check=True)
    verify(config)
    print(f"PASS {service}: native user unit and local application probe")


def report(main):
    try:
        main()
    except (KeyError, ValueError, OSError, RuntimeError, subprocess.CalledProcessError) as error:
        print(f"error: {error}", file=sys.stderr)
        sys.exit(1)
