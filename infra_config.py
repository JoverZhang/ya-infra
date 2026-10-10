"""Read deployment settings from the invocation directory's single infra.toml."""
from pathlib import Path
import tomllib


def section(*keys, root=None):
    path = (Path(root) if root is not None else Path.cwd()) / "infra.toml"
    try:
        with path.open("rb") as stream:
            value = tomllib.load(stream)
    except FileNotFoundError:
        raise ValueError(f"Missing {path}; copy infra.example.toml to ./infra.toml in the directory where you run commands") from None
    except tomllib.TOMLDecodeError:
        raise ValueError(f"Invalid TOML in {path}; fix the configuration syntax") from None
    for key in keys:
        if not isinstance(value, dict) or key not in value:
            raise ValueError(f"{path}: missing configuration section {'.'.join(keys)}")
        value = value[key]
    if not isinstance(value, dict):
        raise ValueError(f"{path}: {'.'.join(keys)} must be a table")
    return value
