"""Missing development tools must fail before any check or installation runs."""

import os
import pathlib
import shutil
import subprocess
import tempfile
import unittest


class DevelopmentCommands(unittest.TestCase):
    def test_missing_tools_fail_with_setup_guidance_without_execution(self):
        commands = {
            "fmt": (
                "gofmt",
                ".tools/python/bin/ruff",
                ".tools/bin/shfmt",
            ),
            "fmt-check": (
                "gofmt",
                ".tools/python/bin/ruff",
                ".tools/bin/shfmt",
            ),
            "lint": (".tools/python/bin/ruff", "shellcheck", ".tools/bin/actionlint"),
        }
        with tempfile.TemporaryDirectory() as directory:
            root = pathlib.Path(directory)
            binary = root / "bin"
            binary.mkdir()
            (binary / "dirname").symlink_to(shutil.which("dirname"))
            (root / "scripts").mkdir()
            original = pathlib.Path(__file__).resolve().parents[2] / "scripts/dev.sh"
            script = root / "scripts/dev.sh"
            shutil.copyfile(original, script)
            script.chmod(0o755)
            marker = root / "executed"
            fixture = '#!/bin/sh\nprintf executed >"${EXECUTION_MARKER}"\nexit 99\n'
            for name in {tool for tools in commands.values() for tool in tools}:
                path = (root if "/" in name else binary) / name
                path.parent.mkdir(parents=True, exist_ok=True)
                path.write_text(fixture)
                path.chmod(0o755)
            for command, tools in commands.items():
                for missing in tools:
                    with self.subTest(command=command, missing=missing):
                        path = (root if "/" in missing else binary) / missing
                        path.unlink()
                        result = subprocess.run(
                            [str(script), command],
                            env=dict(os.environ, PATH=str(binary), EXECUTION_MARKER=str(marker)),
                            text=True,
                            capture_output=True,
                        )
                        self.assertNotEqual(result.returncode, 0)
                        self.assertIn(missing, result.stderr)
                        self.assertIn("make deps or just deps", result.stderr)
                        self.assertFalse(marker.exists())
                        path.write_text(fixture)
                        path.chmod(0o755)
