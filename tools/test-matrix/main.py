"""Run named archive acceptance cases serially; CI owns parallel scheduling."""

import argparse
import contextlib
import json
import os
import pathlib
import signal
import subprocess
import sys
import tempfile

ROOT = pathlib.Path(__file__).resolve().parents[2]


def select_cases(cases, target, kind, engine):
    if target == "native":
        target = "debian-" + engine
    selected = [
        c
        for c in cases
        if (not target or c["target"] == target) and (not kind or c["kind"] == kind)
    ]
    if not selected:
        raise ValueError(
            "unknown or incompatible target; choose: " + ", ".join(c["target"] for c in cases)
        )
    return selected


def command_for(case):
    if case["kind"] == "systemd":
        return [
            str(ROOT / "scripts/test-systemd-container.sh"),
            case["mode"],
            os.environ.get("HOSTLENS_SYSTEMD_IMAGE", case["image"]),
        ]
    return [str(ROOT / "scripts/test-platforms.sh"), case["image"], case["arch"]]


def run_cases(cases, env):
    failures = 0
    for case in cases:
        print("START " + case["target"], flush=True)
        with subprocess.Popen(command_for(case), env=env, start_new_session=True) as process:
            try:
                code = process.wait()
            except KeyboardInterrupt:
                with contextlib.suppress(ProcessLookupError):
                    os.killpg(process.pid, signal.SIGTERM)
                process.wait()
                raise
        failures += code != 0
        print(("PASS " if code == 0 else f"FAIL exit={code} ") + case["target"], flush=True)
    print(f"{len(cases) - failures}/{len(cases)} cases passed", flush=True)
    return int(bool(failures))


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("target", nargs="?", default="")
    modes = parser.add_mutually_exclusive_group()
    modes.add_argument("--list", action="store_true")
    modes.add_argument("--json", action="store_true")
    parser.add_argument("--kind", choices=("platform", "systemd"))
    parser.add_argument(
        "--archives",
        type=pathlib.Path,
        default=os.environ.get("HOSTLENS_ARCHIVES", ROOT / "dist/archives"),
    )
    args = parser.parse_args()
    cases = json.loads((ROOT / "packaging/tests/matrix.json").read_text())["include"]
    engine = None
    if args.target == "native" or not (args.list or args.json):
        engine = subprocess.check_output(
            [str(ROOT / "scripts/container-arch.sh")], text=True
        ).strip()
    selected = select_cases(cases, args.target, args.kind, engine)
    if args.json:
        print(json.dumps({"include": selected}, separators=(",", ":")))
        return 0
    if args.list:
        print("\n".join(c["target"] for c in selected))
        print("native (Debian for the Docker engine architecture)")
        return 0
    version = subprocess.check_output(
        [sys.executable, "-B", str(ROOT / "tools/release/prepare.py"), "version"], text=True
    ).strip()
    archives = args.archives.resolve()
    arches = {engine if c["arch"] == "native" else c["arch"] for c in selected}
    for arch in arches:
        if not (archives / f"hostlens-{version}-linux-{arch}.tar.gz").is_file():
            raise ValueError(
                f"archive missing for {version} linux/{arch}; select existing archives or run make package"
            )
    env = dict(os.environ, HOSTLENS_VERSION=version, HOSTLENS_ARCHIVES=str(archives))
    with tempfile.TemporaryDirectory(prefix="hostlens-helpers-") as directory:
        helpers = pathlib.Path(directory)
        helpers.chmod(0o755)
        env["HOSTLENS_HELPERS"] = directory
        cache = (
            os.environ.get("HOSTLENS_MOD_CACHE")
            or subprocess.check_output(["go", "env", "GOMODCACHE"], text=True).strip()
        )
        print(
            f"Archive acceptance: {version}, directory={archives}, Docker engine={engine}",
            flush=True,
        )
        subprocess.run(
            [
                "go",
                "build",
                "-trimpath",
                "-o",
                directory + "/",
                "./tools/archive",
                "./tools/smoke",
                "./tools/containment",
            ],
            cwd=ROOT,
            env=dict(
                env,
                CGO_ENABLED="0",
                GOOS="linux",
                GOARCH=engine,
                GOTOOLCHAIN="local",
                GOPROXY="off",
                GOMODCACHE=cache,
            ),
            check=True,
        )
        return run_cases(selected, env)


if __name__ == "__main__":

    def interrupt(_signum, _frame):
        raise KeyboardInterrupt

    signal.signal(signal.SIGTERM, interrupt)
    try:
        sys.exit(main())
    except KeyboardInterrupt:
        sys.exit(130)
    except (ValueError, OSError, subprocess.CalledProcessError) as error:
        sys.exit(str(error))
