"""Prepare HostLens-only metadata; GoReleaser owns archives and checksums."""

import argparse
import hashlib
import json
import os
import pathlib
import re
import shutil
import subprocess


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


ROOT = pathlib.Path(__file__).resolve().parents[2]
OUT = ROOT / "dist"
ARCHITECTURES = ("amd64", "arm64")

# Shipped docs keep development-state version placeholders in the repository
# and must always name the archive version they ride in.
DOC_VERSION_PLACEHOLDERS = (
    (b"hostlens-0.1.0-dev-", "hostlens-{}-"),
    (b"hostlens-0.1.0-linux-", "hostlens-{}-linux-"),
)

# Links that resolve in the repository but not inside an archive are rewritten
# to the versioned GitHub tree; every shipped relative link must then resolve
# within the archive.
GITHUB_BASE = "https://github.com/Monska85/hostlens"
REPO_ONLY_LINKS = (
    ("](../../openspec/changes/archive/", GITHUB_BASE + "/blob/v{v}/openspec/changes/archive/"),
    ("](../../openspec/changes)", GITHUB_BASE + "/tree/v{v}/openspec/changes)"),
    ("](../RELEASING.md", GITHUB_BASE + "/blob/v{v}/docs/RELEASING.md"),
    ("](SPEC.md", GITHUB_BASE + "/blob/v{v}/docs/v1/SPEC.md"),
)


def doc_bytes(source, value):
    data = (ROOT / source).read_bytes()
    for placeholder, replacement in DOC_VERSION_PLACEHOLDERS:
        data = data.replace(placeholder, replacement.format(value).encode())
    for repo_link, github_link in REPO_ONLY_LINKS:
        data = data.replace(repo_link.encode(), github_link.format(v=value).encode())
    return data


def clean():
    for name in ("linux-" + arch for arch in ARCHITECTURES):
        path = OUT / name
        if path.is_symlink():
            path.unlink()
        elif path.exists():
            shutil.rmtree(path)


def prepare(value):
    value = version(value)
    records = subprocess.check_output(
        [
            "go",
            "list",
            "-deps",
            "-f",
            "{{with .Module}}{{if not .Main}}{{.Path}} {{.Version}} {{.Dir}}{{end}}{{end}}",
            "./cmd/...",
        ],
        cwd=ROOT,
        text=True,
    )
    modules = {}
    for line in records.splitlines():
        if not line.strip():
            continue
        fields = line.split(maxsplit=2)
        if (
            len(fields) != 3
            or not fields[1].startswith("v")
            or not pathlib.Path(fields[2]).is_absolute()
        ):
            raise ValueError("Malformed dependency module record: " + line)
        modules[fields[0]] = {"Path": fields[0], "Version": fields[1], "Dir": fields[2]}
    dependencies = [modules[key] for key in sorted(modules)]
    for arch in ARCHITECTURES:
        target = OUT / ("linux-" + arch)
        shutil.copytree(ROOT / "packaging/profiles", target / "profiles")
        for source, name in (
            ("packaging/config.yaml", "config.example.yaml"),
            ("docs/v1/OPERATIONS.md", "OPERATIONS.md"),
            ("docs/v1/INSTALL.md", "INSTALL.md"),
            ("docs/v1/VALIDATION.md", "VALIDATION.md"),
            ("LICENSE", "LICENSE"),
            ("NOTICE", "NOTICE.txt"),
        ):
            if name.endswith(".md"):
                (target / name).write_bytes(doc_bytes(source, value))
            else:
                shutil.copyfile(ROOT / source, target / name)
        notices = target / "licenses"
        notices.mkdir()
        for module in dependencies:
            files = [
                p
                for p in pathlib.Path(module["Dir"]).iterdir()
                if p.is_file() and p.name.upper().startswith(("LICENSE", "COPYING", "NOTICE"))
            ]
            if not any(p.name.upper().startswith(("LICENSE", "COPYING")) for p in files):
                raise ValueError("Upstream license missing: " + module["Path"])
            for path in files:
                name = module["Path"].replace("/", "_") + "@" + module["Version"] + "_" + path.name
                shutil.copyfile(path, notices / name)
        manifest = {
            "version": value,
            "schema": 1,
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
                path.chmod(
                    0o755
                    if path.name in ("hostlens", "hostlens-diagnostics", "hostlens-docker-observer")
                    else 0o644
                )


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
