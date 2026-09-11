"""Exercise optional compiler storage without requiring a nested Docker daemon."""

import os
import pathlib
import subprocess
import tempfile
import unittest

ROOT = pathlib.Path(__file__).resolve().parents[2]


class CompilerCacheTests(unittest.TestCase):
    def test_disabled_missing_and_image_isolation(self):
        with tempfile.TemporaryDirectory() as directory:
            root = pathlib.Path(directory)
            binary = root / "bin"
            binary.mkdir()
            docker = binary / "docker"
            docker.write_text('#!/bin/sh\nprintf "sha256:%s\\n" "${IMAGE_ID}"\n')
            docker.chmod(0o755)
            cache = root / "cache"
            cache.mkdir(mode=0o700)
            env = dict(os.environ, PATH=str(binary) + ":" + os.environ["PATH"])
            command = [
                "sh",
                "-ec",
                'image=fixture; . "$1"; printf "selected=%s\\n" "${compiler_cache}"',
                "cache-test",
                str(ROOT / "scripts/compiler-cache.sh"),
            ]
            for value in ("", str(root / "absent")):
                result = subprocess.run(
                    command,
                    env=dict(env, HOSTLENS_BUILD_CACHE=value),
                    text=True,
                    capture_output=True,
                    check=True,
                )
                self.assertIn("selected=\n", result.stdout)
            for identity in ("first", "second"):
                result = subprocess.run(
                    command,
                    env=dict(env, HOSTLENS_BUILD_CACHE=str(cache), IMAGE_ID=identity),
                    text=True,
                    capture_output=True,
                    check=True,
                )
                self.assertIn(f"selected={cache}/{identity}\n", result.stdout)
                self.assertTrue((cache / identity).is_dir())
            restored = cache / "first" / "bucket"
            restored.mkdir(mode=0o700)
            entry = restored / "compiler-output"
            entry.write_text("restored cache output")
            entry.chmod(0o600)
            subprocess.run(
                command,
                env=dict(env, HOSTLENS_BUILD_CACHE=str(cache), IMAGE_ID="first"),
                text=True,
                capture_output=True,
                check=True,
            )
            self.assertEqual(restored.stat().st_mode & 0o777, 0o777)
            self.assertEqual(entry.stat().st_mode & 0o777, 0o666)
            (cache / "linked").symlink_to(root)
            result = subprocess.run(
                command,
                env=dict(env, HOSTLENS_BUILD_CACHE=str(cache), IMAGE_ID="linked"),
                text=True,
                capture_output=True,
            )
            self.assertNotEqual(result.returncode, 0)
            self.assertIn("symlink", result.stderr)
            cache.chmod(0o755)
            result = subprocess.run(
                command,
                env=dict(env, HOSTLENS_BUILD_CACHE=str(cache), IMAGE_ID="unsafe"),
                text=True,
                capture_output=True,
                check=True,
            )
            self.assertIn("selected=\n", result.stdout)
            self.assertFalse((cache / "unsafe").exists())
