"""Exercise the coverage launcher's failure and cleanup contract without Docker."""

import os
import pathlib
import shutil
import signal
import subprocess
import tempfile
import time
import unittest


class CoverageLauncher(unittest.TestCase):
    def test_reports_and_failures(self):
        for scenario in ("success", "failed", "incomplete", "missing-cache", "cancel"):
            with self.subTest(scenario=scenario), tempfile.TemporaryDirectory() as directory:
                root = pathlib.Path(directory)
                (root / "scripts").mkdir()
                script = root / "scripts/test-container.sh"
                shutil.copyfile(
                    pathlib.Path(__file__).resolve().parents[2] / script.relative_to(root), script
                )
                shutil.copyfile(
                    pathlib.Path(__file__).resolve().parents[2] / "scripts/compiler-cache.sh",
                    root / "scripts/compiler-cache.sh",
                )
                binary = root / "bin"
                binary.mkdir()
                docker = binary / "docker"
                docker.write_text(
                    "#!/usr/bin/env python3\n"
                    "import os, pathlib, signal, sys, time\n"
                    "root = pathlib.Path(os.environ['FIXTURE_ROOT'])\n"
                    "args = sys.argv[1:]\n"
                    "if args[0] == 'rm':\n"
                    " (root / 'cleaned').write_text(args[-1])\n"
                    " if os.environ['SCENARIO'] == 'cancel':\n"
                    "  try: os.kill(int((root / 'pid').read_text()), signal.SIGTERM)\n"
                    "  except ProcessLookupError: pass\n"
                    " sys.exit(0)\n"
                    "pathlib.Path(args[args.index('--cidfile')+1]).write_text('fixture-container')\n"
                    "(root / 'pid').write_text(str(os.getpid()))\n"
                    "mount = next(a for a in args if a.endswith('dst=/coverage'))\n"
                    "out = pathlib.Path(mount.split('src=',1)[1].split(',dst=',1)[0])\n"
                    "for name in ['coverage.out', 'index.html', 'functions.txt']:\n"
                    " if os.environ['SCENARIO'] != 'incomplete' or name != 'index.html':\n"
                    "  (out / name).write_text('fresh report')\n"
                    "if os.environ['SCENARIO'] == 'cancel': time.sleep(30)\n"
                    "sys.exit(17 if os.environ['SCENARIO'] == 'failed' else 0)\n"
                )
                docker.chmod(0o755)
                reports = root / "coverage"
                reports.mkdir()
                for name in ("coverage.out", "index.html", "functions.txt"):
                    (reports / name).write_text("stale report")
                cache = root / "modules"
                if scenario != "missing-cache":
                    (cache / "cache/download").mkdir(parents=True)
                temporary = root / "tmp"
                temporary.mkdir()
                env = dict(
                    os.environ,
                    PATH=f"{binary}:{os.environ['PATH']}",
                    HOSTLENS_MOD_CACHE=str(cache),
                    TMPDIR=str(temporary),
                    FIXTURE_ROOT=str(root),
                    SCENARIO=scenario,
                    HOSTLENS_BUILD_CACHE="",
                )
                process = subprocess.Popen(
                    ["sh", str(script), "coverage"],
                    env=env,
                    stdout=subprocess.PIPE,
                    stderr=subprocess.PIPE,
                    text=True,
                )
                try:
                    if scenario == "cancel":
                        deadline = time.monotonic() + 5
                        while not (root / "pid").exists() and process.poll() is None:
                            if time.monotonic() > deadline:
                                self.fail("fixture container did not start")
                            time.sleep(0.01)
                        process.send_signal(signal.SIGTERM)
                    stdout, stderr = process.communicate(timeout=10)
                finally:
                    if process.poll() is None:
                        process.kill()
                        process.communicate()
                if scenario == "success":
                    self.assertEqual(process.returncode, 0, stderr)
                    self.assertIn("Coverage reports:", stdout)
                    self.assertTrue(all(p.read_text() == "fresh report" for p in reports.iterdir()))
                    self.assertEqual(len(list(reports.iterdir())), 3)
                else:
                    self.assertNotEqual(process.returncode, 0)
                    self.assertEqual(list(reports.iterdir()), [])
                    self.assertNotIn("Coverage reports:", stdout)
                    if scenario == "failed":
                        self.assertEqual(process.returncode, 17)
                    if scenario == "cancel":
                        self.assertEqual(process.returncode, 143)
                self.assertEqual(list(temporary.iterdir()), [])
                if scenario != "missing-cache":
                    self.assertEqual((root / "cleaned").read_text(), "fixture-container")
