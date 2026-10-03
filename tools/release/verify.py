"""Verify HostLens archives and their release manifests without extracting them."""

import hashlib
import json
import pathlib
import sys
import tarfile

REQUIRED = {
    "hostlens",
    "config.example.yaml",
    "INSTALL.md",
    "UPGRADING.md",
    "OPERATIONS.md",
    "VALIDATION.md",
    "tools.json",
    "release.json",
    "LICENSE",
    "NOTICE.txt",
    "profiles/host.yaml",
    "profiles/docker-readonly.yaml",
    "profiles/docker-evidence.yaml",
    "profiles/services-readonly.yaml",
    "profiles/service-logs.yaml",
    "profiles/repair-example.yaml",
    "systemd/hostlens-gateway.service",
    "systemd/hostlens-observer.service",
    "systemd/hostlens-repair.service",
}


def check(path, arch, version):
    files = {}
    with tarfile.open(path, "r:gz") as archive:
        for member in archive:
            if (
                not member.isfile()
                or member.name in files
                or member.name.startswith("/")
                or ".." in pathlib.PurePosixPath(member.name).parts
            ):
                raise ValueError("unsafe archive member: " + member.name)
            if member.size > 64 << 20:
                raise ValueError("oversized archive member: " + member.name)
            files[member.name] = archive.extractfile(member).read()
    missing = REQUIRED - files.keys()
    if missing:
        raise ValueError("missing archive members: " + ", ".join(sorted(missing)))
    manifest = json.loads(files["release.json"])
    if (
        manifest["version"] != version
        or manifest["architecture"] != arch
        or manifest["schema"] != 2
    ):
        raise ValueError("release identity mismatch")
    actual = {
        name: hashlib.sha256(data).hexdigest()
        for name, data in files.items()
        if name != "release.json"
    }
    if manifest["checksums"] != actual:
        raise ValueError("release member checksums differ")
    if files["hostlens"][:4] != b"\x7fELF":
        raise ValueError("binary is not ELF")
    return files["tools.json"]


if __name__ == "__main__":
    if len(sys.argv) != 3:
        raise SystemExit("usage: verify.py ARCHIVES VERSION")
    directory = pathlib.Path(sys.argv[1])
    version = sys.argv[2]
    catalogs = [
        check(directory / f"hostlens-{version}-linux-{arch}.tar.gz", arch, version)
        for arch in ("amd64", "arm64")
    ]
    if catalogs[0] != catalogs[1]:
        raise ValueError("MCP contracts differ across architectures")
    print("PASS: archive contents, manifests, and MCP contracts verified")
