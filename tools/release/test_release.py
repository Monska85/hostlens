"""Check release identity and archive trust boundaries."""

import importlib.util
import io
import json
import pathlib
import tarfile
import tempfile
import unittest

ROOT = pathlib.Path(__file__).parent


def load(name):
    spec = importlib.util.spec_from_file_location(name, ROOT / (name + ".py"))
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


prepare = load("prepare")
verify = load("verify")


class ReleaseTests(unittest.TestCase):
    def test_semver_rejects_ambiguous_versions(self):
        for value in ("0.1.0-dev", "1.2.3", "2.0.0-rc.1"):
            self.assertEqual(prepare.version(value), value)
        for value in ("v1.2.3", "01.2.3", "1.2.3-01", "../x", "1.2.3\n"):
            with self.subTest(value=value), self.assertRaises(ValueError):
                prepare.version(value)

    def test_archive_rejects_path_escape_and_duplicate_members(self):
        for names in (("../escape",), ("hostlens", "hostlens"), ("link",)):
            with self.subTest(names=names), tempfile.TemporaryDirectory() as directory:
                path = pathlib.Path(directory) / "candidate.tar.gz"
                with tarfile.open(path, "w:gz") as archive:
                    for name in names:
                        info = tarfile.TarInfo(name)
                        info.size = 1
                        if name == "link":
                            info.type = tarfile.SYMTYPE
                            info.linkname = "/etc/passwd"
                        archive.addfile(info, io.BytesIO(b"x") if info.isfile() else None)
                with self.assertRaises(ValueError):
                    verify.check(path, "amd64", "1.2.3")

    def test_manifest_must_cover_every_member(self):
        with tempfile.TemporaryDirectory() as directory:
            path = pathlib.Path(directory) / "candidate.tar.gz"
            files = {name: b"x" for name in verify.REQUIRED}
            files["release.json"] = json.dumps(
                {
                    "version": "1.2.3",
                    "architecture": "amd64",
                    "schema": 2,
                    "checksums": {},
                }
            ).encode()
            with tarfile.open(path, "w:gz") as archive:
                for name, data in files.items():
                    info = tarfile.TarInfo(name)
                    info.size = len(data)
                    archive.addfile(info, io.BytesIO(data))
            with self.assertRaisesRegex(ValueError, "checksums"):
                verify.check(path, "amd64", "1.2.3")


if __name__ == "__main__":
    unittest.main()
