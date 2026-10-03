"""Prepare HostLens release metadata; GoReleaser creates the archives."""

import argparse
import hashlib
import json
import os
import pathlib
import platform
import re
import shutil
import subprocess

ROOT = pathlib.Path(__file__).resolve().parents[2]
OUT = ROOT / "dist"
ARCHITECTURES = ("amd64", "arm64")


def version(value):
    number = r"(0|[1-9][0-9]*)"
    if not re.fullmatch(
        number + r"\." + number + r"\." + number + r"(?:-[0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*)?", value
    ):
        raise ValueError("expected MAJOR.MINOR.PATCH with an optional SemVer prerelease")
    if "-" in value:
        for part in value.split("-", 1)[1].split("."):
            if part.isdigit() and len(part) > 1 and part[0] == "0":
                raise ValueError("numeric prerelease identifiers cannot have leading zeros")
    return value


def clean():
    for arch in ARCHITECTURES:
        path = OUT / ("linux-" + arch)
        if path.is_symlink():
            path.unlink()
        elif path.exists():
            shutil.rmtree(path)


def dependencies():
    records = subprocess.check_output(
        [
            "go",
            "list",
            "-deps",
            "-f",
            "{{with .Module}}{{if not .Main}}{{.Path}} {{.Version}} {{.Dir}}{{end}}{{end}}",
            "./cmd/hostlens",
        ],
        cwd=ROOT,
        text=True,
    )
    modules = {}
    for line in records.splitlines():
        if not line.strip():
            continue
        fields = line.split(maxsplit=2)
        if len(fields) != 3 or not fields[1].startswith("v"):
            raise ValueError("malformed module record: " + line)
        modules[fields[0]] = (fields[1], pathlib.Path(fields[2]))
    return modules


def prepare(value):
    value = version(value)
    modules = dependencies()
    native = {"x86_64": "amd64", "aarch64": "arm64"}.get(platform.machine().lower())
    for arch in ARCHITECTURES:
        target = OUT / ("linux-" + arch)
        if arch == native:
            reported = subprocess.check_output([target / "hostlens", "version"], text=True).strip()
            if reported != value:
                raise ValueError("release binary version differs from manifest version")
        (target / "profiles").mkdir(parents=True, exist_ok=True)
        shutil.copytree(ROOT / "packaging/profiles", target / "profiles", dirs_exist_ok=True)
        shutil.copytree(ROOT / "internal/uninstall/systemd", target / "systemd")
        for source, name in (
            ("packaging/config.yaml", "config.example.yaml"),
            ("docs/current/INSTALL.md", "INSTALL.md"),
            ("docs/current/OPERATIONS.md", "OPERATIONS.md"),
            ("docs/current/VALIDATION.md", "VALIDATION.md"),
            ("docs/current/tools.json", "tools.json"),
            ("UPGRADING.md", "UPGRADING.md"),
            ("LICENSE", "LICENSE"),
            ("NOTICE", "NOTICE.txt"),
        ):
            data = (ROOT / source).read_bytes().replace(b"0.1.0-dev", value.encode())
            if name == "INSTALL.md":
                data = data.replace(b"../../UPGRADING.md", b"UPGRADING.md")
            if name == "UPGRADING.md":
                data = data.replace(b"docs/current/OPERATIONS.md", b"OPERATIONS.md")
                data = data.replace(b"docs/current/INSTALL.md", b"INSTALL.md")
            (target / name).write_bytes(data)
        notices = target / "licenses"
        notices.mkdir()
        for module, (module_version, source) in sorted(modules.items()):
            files = [
                p
                for p in source.iterdir()
                if p.is_file() and p.name.upper().startswith(("LICENSE", "COPYING", "NOTICE"))
            ]
            if not any(p.name.upper().startswith(("LICENSE", "COPYING")) for p in files):
                raise ValueError("upstream license missing: " + module)
            for item in files:
                name = module.replace("/", "_") + "@" + module_version + "_" + item.name
                shutil.copyfile(item, notices / name)
        manifest = {
            "version": value,
            "schema": 2,
            "architecture": arch,
            "checksums": {
                str(p.relative_to(target)): hashlib.sha256(p.read_bytes()).hexdigest()
                for p in sorted(target.rglob("*"))
                if p.is_file()
            },
        }
        (target / "release.json").write_text(json.dumps(manifest, indent=2, sort_keys=True) + "\n")
        for path in target.rglob("*"):
            if path.is_file():
                path.chmod(0o755 if path.name == "hostlens" else 0o644)
    tools = [OUT / ("linux-" + arch) / "tools.json" for arch in ARCHITECTURES]
    if len({hashlib.sha256(path.read_bytes()).hexdigest() for path in tools}) != 1:
        raise ValueError("architecture-specific tool snapshots differ")


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("action", choices=("version", "clean", "manifest"))
    parser.add_argument("value", nargs="?", default=os.environ.get("HOSTLENS_VERSION", "0.1.0-dev"))
    args = parser.parse_args()
    if args.action == "clean":
        clean()
    elif args.action == "version":
        print(version(args.value))
    else:
        prepare(args.value)
