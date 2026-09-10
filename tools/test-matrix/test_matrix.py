"""Check serial acceptance selection, failure reporting and cancellation."""

import contextlib
import importlib.util
import io
import json
import os
import pathlib
import signal
import subprocess
import sys
import tempfile
import time
import unittest
from unittest import mock

spec = importlib.util.spec_from_file_location(
    "matrix_runner", pathlib.Path(__file__).with_name("main.py")
)
runner = importlib.util.module_from_spec(spec)
spec.loader.exec_module(runner)


class MatrixTests(unittest.TestCase):
    def setUp(self):
        self.cases = json.loads((runner.ROOT / "packaging/tests/matrix.json").read_text())[
            "include"
        ]

    def test_selection(self):
        for arch in ("amd64", "arm64"):
            self.assertEqual(
                runner.select_cases(self.cases, "native", None, arch)[0]["target"], "debian-" + arch
            )
        self.assertEqual(len(runner.select_cases(self.cases, "", "systemd", None)), 2)
        for target, kind in (("missing", None), ("debian-amd64", "systemd"), ("native", None)):
            with self.subTest(target=target, kind=kind), self.assertRaises(ValueError):
                runner.select_cases(self.cases, target, kind, "unknown")

    def test_stalled_status_probe_respects_budget(self):
        source = (runner.ROOT / "scripts/systemd-acceptance.sh").read_text()
        function = "wait_for() {" + source.split("wait_for() {", 1)[1].split("\nready()", 1)[0]
        # Shorten only the configured budget, keeping the production polling code.
        function = function.replace("+ 30", "+ 1")
        result = subprocess.run(
            ["sh", "-c", function + "\nwait_for stalled sleep 75"],
            capture_output=True,
            text=True,
            timeout=3,
        )
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("Timed out waiting for stalled", result.stderr)

    def test_systemd_uses_selected_image(self):
        case = dict(target="systemd", kind="systemd", mode="restricted", image="selected")
        with mock.patch.dict(os.environ, clear=True):
            self.assertEqual(runner.command_for(case)[1:], ["restricted", "selected"])
        with mock.patch.dict(os.environ, HOSTLENS_SYSTEMD_IMAGE="override"):
            self.assertEqual(runner.command_for(case)[1:], ["restricted", "override"])

    def test_json_needs_no_external_tools(self):
        with (
            mock.patch.object(sys, "argv", ["matrix", "--json"]),
            mock.patch.object(
                runner.subprocess, "check_output", side_effect=AssertionError("external command")
            ),
            contextlib.redirect_stdout(io.StringIO()) as output,
        ):
            self.assertEqual(runner.main(), 0)
        self.assertEqual(json.loads(output.getvalue())["include"], self.cases)

    def test_failures_do_not_skip_later_cases(self):
        with tempfile.TemporaryDirectory() as directory:
            events = pathlib.Path(directory) / "events"
            cases = [{"target": str(code)} for code in (0, 7, 0)]

            def command(case):
                return [
                    sys.executable,
                    "-c",
                    "import pathlib,sys; pathlib.Path(sys.argv[1]).open('a').write(sys.argv[2]+'\\n'); sys.exit(int(sys.argv[2]))",
                    str(events),
                    case["target"],
                ]

            with (
                mock.patch.object(runner, "command_for", side_effect=command),
                contextlib.redirect_stdout(io.StringIO()) as output,
            ):
                self.assertEqual(runner.run_cases(cases, os.environ.copy()), 1)
            self.assertEqual(events.read_text().splitlines(), ["0", "7", "0"])
            self.assertIn("FAIL exit=7 7", output.getvalue())
            self.assertIn("2/3 cases passed", output.getvalue())

    def test_cancellation_runs_child_cleanup_and_skips_queued_case(self):
        with tempfile.TemporaryDirectory() as directory:
            root = pathlib.Path(directory)
            ready, cleaned, queued = (root / name for name in ("ready", "cleaned", "queued"))
            worker = root / "worker.py"
            worker.write_text(f"""import os,pathlib,signal,time
signal.signal(signal.SIGTERM, lambda *_: (pathlib.Path({str(cleaned)!r}).touch(), exit(0)))
pathlib.Path({str(ready)!r}).with_suffix(".tmp").write_text(str(os.getpid()))
pathlib.Path({str(ready)!r}).with_suffix(".tmp").rename({str(ready)!r})
time.sleep(30)
""")
            parent = root / "parent.py"
            parent.write_text(f"""import importlib.util,os,sys
s=importlib.util.spec_from_file_location('runner',{runner.__file__!r})
m=importlib.util.module_from_spec(s);s.loader.exec_module(m)
m.command_for=lambda c: [sys.executable,{str(worker)!r}] if c['target']=='active' else [sys.executable,'-c',"open({str(queued)!r},'w').close()"]
try: sys.exit(m.run_cases([{{'target':'active'}},{{'target':'queued'}}],os.environ.copy()))
except KeyboardInterrupt: sys.exit(130)
""")
            process = subprocess.Popen(
                [sys.executable, "-B", str(parent)],
                stdout=subprocess.DEVNULL,
                start_new_session=True,
            )
            try:
                deadline = time.monotonic() + 5
                while not ready.exists() and process.poll() is None and time.monotonic() < deadline:
                    time.sleep(0.01)
                self.assertTrue(ready.exists(), "fixture did not start")
                process.send_signal(signal.SIGINT)
                self.assertEqual(process.wait(timeout=5), 130)
                self.assertTrue(cleaned.exists(), "active child did not receive cleanup signal")
                self.assertFalse(queued.exists())
            finally:
                if process.poll() is None:
                    process.kill()
                    process.wait()
                if ready.exists():
                    with contextlib.suppress(ProcessLookupError):
                        os.kill(int(ready.read_text()), signal.SIGTERM)


if __name__ == "__main__":
    unittest.main()
