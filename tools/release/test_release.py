"""Check metadata preparation; archive safety belongs to the shared Go reader."""

import hashlib
import importlib.util
import json
import os
import pathlib
import subprocess
import tempfile
import textwrap
import unittest
from unittest import mock

spec = importlib.util.spec_from_file_location(
    "prepare", pathlib.Path(__file__).with_name("prepare.py")
)
prepare = importlib.util.module_from_spec(spec)
spec.loader.exec_module(prepare)


class PreparationTests(unittest.TestCase):
    def test_versions(self):
        for value in ("0.1.0-dev", "1.2.3", "2.0.0-rc.1", "1.0.0-alpha-1"):
            self.assertEqual(prepare.version(value), value)
        for value in (
            "v1.2.3",
            "01.2.3",
            "1.2",
            "1.2.3-01",
            "1.2.3+build",
            "../x",
            "1.2.3\n",
            "",
            "--help",
        ):
            with self.subTest(value=value), self.assertRaises(ValueError):
                prepare.version(value)

    def setUp(self):
        temporary = tempfile.TemporaryDirectory(prefix="release fixture ")
        self.addCleanup(temporary.cleanup)
        self.root = pathlib.Path(temporary.name)
        for name in (
            "packaging/profiles/nginx.yaml",
            "packaging/profiles/docker-readonly.yaml",
            "packaging/config.yaml",
            "docs/v1/OPERATIONS.md",
            "docs/v1/INSTALL.md",
            "docs/v1/VALIDATION.md",
            "LICENSE",
            "NOTICE",
            "upstream/LICENSE",
            "upstream/NOTICE",
        ):
            path = self.root / name
            path.parent.mkdir(parents=True, exist_ok=True)
            path.write_text(name)
        for name, value in (("ROOT", self.root), ("OUT", self.root / "dist")):
            patch = mock.patch.object(prepare, name, value)
            patch.start()
            self.addCleanup(patch.stop)
        patch = mock.patch.object(
            prepare.subprocess,
            "check_output",
            return_value=f"example.org/module v1.0.0 {self.root / 'upstream'}\n" * 2,
        )
        self.modules = patch.start()
        self.addCleanup(patch.stop)
        self.reset()

    def reset(self):
        prepare.clean()
        for arch in prepare.ARCHITECTURES:
            stage = prepare.OUT / ("linux-" + arch)
            stage.mkdir(parents=True)
            for name in ("hostlens", "hostlens-diagnostics", "hostlens-docker-observer"):
                (stage / name).write_bytes(b"binary")

    def test_fresh_manifest_and_dependency_notices(self):
        prepare.prepare("1.2.3")
        (prepare.OUT / "linux-amd64/secret").write_text("stale")
        (self.root / "packaging/profiles/nginx.yaml").unlink()
        self.reset()
        prepare.prepare("1.2.3")
        for arch in prepare.ARCHITECTURES:
            stage = prepare.OUT / ("linux-" + arch)
            manifest = json.loads((stage / "release.json").read_text())
            self.assertEqual(
                (manifest["schema"], manifest["version"], manifest["architecture"]),
                (1, "1.2.3", arch),
            )
            self.assertEqual(
                manifest["checksums"],
                {
                    str(p.relative_to(stage)): hashlib.sha256(p.read_bytes()).hexdigest()
                    for p in stage.rglob("*")
                    if p.is_file() and p.name != "release.json"
                },
            )
            self.assertNotIn("secret", manifest["checksums"])
            self.assertNotIn("profiles/nginx.yaml", manifest["checksums"])
            self.assertEqual(len(list((stage / "licenses").iterdir())), 2)
            self.assertEqual(
                (stage / "licenses/example.org_module@v1.0.0_NOTICE").read_text(), "upstream/NOTICE"
            )
            for name, mode in (
                ("hostlens", 0o755),
                ("hostlens-diagnostics", 0o755),
                ("release.json", 0o644),
            ):
                self.assertEqual((stage / name).stat().st_mode & 0o777, mode)

    def test_clean_does_not_follow_staging_symlink(self):
        prepare.clean()
        (prepare.OUT / "linux-amd64").symlink_to(self.root / "upstream", target_is_directory=True)
        (prepare.OUT / "unrelated").write_text("keep")
        prepare.clean()
        self.assertFalse((prepare.OUT / "linux-amd64").is_symlink())
        self.assertTrue((self.root / "upstream/LICENSE").is_file())
        self.assertEqual((prepare.OUT / "unrelated").read_text(), "keep")

    def test_shipped_docs_carry_the_candidate_version(self):
        (self.root / "docs/v1/OPERATIONS.md").write_text(
            "tar -xzf hostlens-0.1.0-dev-linux-amd64.tar.gz\n"
            "After the v0.1.0 release, history stays historical.\n"
            "Tool labels are the fixed [diagnostic tool names](SPEC.md#tool-contract).\n"
        )
        (self.root / "docs/v1/INSTALL.md").write_text(
            "tar -xzf hostlens-0.1.0-linux-amd64.tar.gz -C /opt/hostlens-release\n"
        )
        (self.root / "docs/v1/VALIDATION.md").write_text(
            "The [audit records](../../openspec/changes) retain findings.\n"
            "The [delivery review](../../openspec/changes/archive/2026-09-11-simplify-delivery-toolchain/review.md) records scope.\n"
            "Use the [coverage command](../RELEASING.md#coverage).\n"
        )
        prepare.prepare("2.3.4")
        for arch in prepare.ARCHITECTURES:
            stage = prepare.OUT / ("linux-" + arch)
            for name in ("OPERATIONS.md", "INSTALL.md"):
                shipped = (stage / name).read_text()
                self.assertIn("hostlens-2.3.4-linux-amd64.tar.gz", shipped)
                self.assertNotIn("hostlens-0.1.0", shipped)
            self.assertIn(
                "After the v0.1.0 release",
                (stage / "OPERATIONS.md").read_text(),
                "historical version facts must not be templated",
            )
            shipped = (stage / "VALIDATION.md").read_text()
            for link in (
                "https://github.com/Monska85/hostlens/tree/v2.3.4/openspec/changes)",
                "https://github.com/Monska85/hostlens/blob/v2.3.4/openspec/changes/archive/2026-09-11-simplify-delivery-toolchain/review.md)",
                "https://github.com/Monska85/hostlens/blob/v2.3.4/docs/RELEASING.md#coverage)",
            ):
                self.assertIn(link, shipped)
            self.assertIn(
                "https://github.com/Monska85/hostlens/blob/v2.3.4/docs/v1/SPEC.md#tool-contract",
                (stage / "OPERATIONS.md").read_text(),
            )
            self.assertNotIn("../../openspec", shipped)

    def test_invalid_dependencies_fail_before_manifest(self):
        for record in ("malformed", "module v1.0.0 relative", "module invalid /tmp"):
            with self.subTest(record=record), self.assertRaisesRegex(ValueError, "Malformed"):
                self.modules.return_value = record
                prepare.prepare("1.2.3")
        self.modules.return_value = f"example.org/module v1.0.0 {self.root / 'upstream'}\n"
        (self.root / "upstream/LICENSE").unlink()
        with self.assertRaisesRegex(ValueError, "Upstream license missing"):
            prepare.prepare("1.2.3")
        self.assertFalse(any(prepare.OUT.rglob("release.json")))


class ChecksumTests(unittest.TestCase):
    def test_complete_candidate_required(self):
        root = pathlib.Path(__file__).resolve().parents[2]
        script = (root / "scripts/container-acceptance.sh").read_text()
        archive_check = "\n".join(
            line.strip()
            for line in script.splitlines()
            if line.strip().startswith(("sha256sum ", "sort checksums.txt"))
        )
        publish = textwrap.dedent(
            (root / ".github/workflows/release.yml").read_text().rsplit("run: |", 1)[1]
        )
        with tempfile.TemporaryDirectory() as directory:
            work = pathlib.Path(directory)
            archives = work / "dist" / "archives"
            archives.mkdir(parents=True)
            names = [f"hostlens-1.2.3-linux-{arch}.tar.gz" for arch in ("amd64", "arm64")]
            for name in names:
                (archives / name).write_bytes(b"candidate")
            valid = subprocess.check_output(["sha256sum", *names], cwd=archives, text=True)
            gh = work / "gh"
            gh.write_text(
                "#!/bin/sh\n"
                "# A release never exists on first view; create succeeds and marks it.\n"
                'if [ "${1}" = release ] && [ "${2}" = view ]; then\n'
                "  exit 1\n"
                "fi\n"
                'if [ "${1}" = release ] && [ "${2}" = create ]; then\n'
                "  touch uploaded\n"
                "  exit 0\n"
                "fi\n"
                "exit 1\n"
            )
            gh.chmod(0o755)
            # Publish extracts release notes from the changelog section for
            # the version; provide one so the checksum checks stay the focus.
            (work / "CHANGELOG.md").write_text(
                "## 1.2.3 - 2026-01-01\n\n### Added\n\n- Fixture release notes.\n"
            )
            env = dict(
                os.environ,
                HOSTLENS_VERSION="1.2.3",
                RELEASE_TAG="v1.2.3",
                GITHUB_WORKSPACE=work,
                PATH=directory + ":" + os.environ["PATH"],
            )
            for content in (
                valid,
                valid.splitlines()[0] + "\n",
                valid + "0" * 64 + "  " + names[0] + "\n",
            ):
                (archives / "checksums.txt").write_text(content)
                for command, cwd in ((archive_check, archives), (publish, work)):
                    (archives / "uploaded").unlink(missing_ok=True)
                    result = subprocess.run(
                        ["bash", "-eo", "pipefail", "-c", command],
                        cwd=cwd,
                        env=env,
                        capture_output=True,
                    )
                    self.assertEqual(result.returncode == 0, content == valid)
                    if content != valid:
                        self.assertFalse((archives / "uploaded").exists())


if __name__ == "__main__":
    unittest.main()
