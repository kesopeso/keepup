"""Behavior checks for index freshness; run inside the Graphify container."""
from concurrent.futures import ThreadPoolExecutor
import json
import os
from pathlib import Path
import subprocess
import sys
import tempfile
import unittest

HELPER = Path(__file__).with_name("index.py").resolve()
FAKE = '''#!/usr/bin/env python
import json, os, sys, time
from pathlib import Path
out=Path(os.environ["GRAPHIFY_OUT"])
count=out / "calls"
count.write_text(str(int(count.read_text())+1 if count.exists() else 1))
mode=os.environ.get("KEEPUP_GRAPHIFY_TEST_MODE", "")
if mode=="fail": sys.exit(7)
if mode=="race": Path("apps/main.ts").write_text("changed during extraction")
if mode=="slow": time.sleep(.2)
if mode=="invalid":
    (out / "graph.json").write_text("invalid")
else:
    (out / "graph.json").write_text(json.dumps({"nodes":[{"id":"apps_main", "label":"main.ts", "source_file":"apps/main.ts", "source_location":"L1"}], "links":[]}))
'''


class FreshnessTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory(prefix="keepup-index-test-")
        self.root = Path(self.temp.name)
        subprocess.run(["git", "init", "-q", str(self.root)], check=True)
        (self.root / "apps").mkdir()
        self.source = self.root / "apps/main.ts"
        self.source.write_text("export const value = 1;\n")
        (self.root / ".gitignore").write_text("graphify-out/\napps/ignored/\n")
        (self.root / ".graphifyignore").write_text("/*\n!/apps/\n")
        (self.root / "fake-bin").mkdir()
        executable = self.root / "fake-bin/graphify"
        executable.write_text(FAKE)
        executable.chmod(0o755)
        self.env = {**os.environ, "PATH": str(executable.parent) + os.pathsep + os.environ["PATH"]}
        self.stamp = self.root / "graphify-out/.keepup-index.json"

    def tearDown(self):
        self.temp.cleanup()

    def run_helper(self, command, mode=""):
        return subprocess.run([sys.executable, str(HELPER), command], cwd=self.root,
                              env={**self.env, "KEEPUP_GRAPHIFY_TEST_MODE": mode}, capture_output=True, text=True)

    def ensure_ok(self):
        result = self.run_helper("ensure")
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)

    def test_missing_then_fresh_and_no_unnecessary_extraction(self):
        self.assertEqual(self.run_helper("status").returncode, 3)
        self.assertFalse((self.root / "graphify-out").exists())
        self.ensure_ok()
        self.ensure_ok()
        self.assertEqual(self.run_helper("status").returncode, 0)
        self.assertEqual((self.root / "graphify-out/calls").read_text(), "1")
        self.assertIn("git_revision", json.loads(self.stamp.read_text()))

    def test_uncommitted_edit_with_unchanged_size_and_mtime(self):
        self.ensure_ok()
        old = self.source.stat()
        self.source.write_text("export const value = 2;\n")
        os.utime(self.source, ns=(old.st_atime_ns, old.st_mtime_ns))
        self.assertEqual(self.run_helper("status").returncode, 3)
        self.ensure_ok()
        self.assertEqual(self.run_helper("status").returncode, 0)

    def test_tracked_delete_rename_and_untracked_addition(self):
        subprocess.run(["git", "add", "apps/main.ts"], cwd=self.root, check=True)
        self.ensure_ok()
        self.source.rename(self.root / "apps/renamed.ts")
        self.assertEqual(self.run_helper("status").returncode, 3)
        self.ensure_ok()
        (self.root / "apps/new.ts").write_text("export const fresh = 1;")
        self.assertEqual(self.run_helper("status").returncode, 3)
        self.ensure_ok()
        (self.root / "apps/renamed.ts").unlink()
        self.assertEqual(self.run_helper("status").returncode, 3)

    def test_docs_and_ignored_output_do_not_invalidate(self):
        self.ensure_ok()
        (self.root / "docs").mkdir()
        (self.root / "docs/note.md").write_text("documentation")
        (self.root / "apps/ignored").mkdir()
        (self.root / "apps/ignored/generated.ts").write_text("ignored output")
        self.assertEqual(self.run_helper("status").returncode, 0)

    def test_docs_only_commit_does_not_invalidate(self):
        subprocess.run(["git", "add", "."], cwd=self.root, check=True)
        commit = ["git", "-c", "user.name=Index Test", "-c", "user.email=index@example.invalid", "commit", "-qm"]
        subprocess.run([*commit, "initial"], cwd=self.root, check=True)
        self.ensure_ok()
        (self.root / "note.md").write_text("docs change")
        subprocess.run(["git", "add", "note.md"], cwd=self.root, check=True)
        subprocess.run([*commit, "docs"], cwd=self.root, check=True)
        self.assertEqual(self.run_helper("status").returncode, 0)

    def test_scope_config_change_invalidates(self):
        self.ensure_ok()
        (self.root / ".graphifyignore").write_text("/*\n!/apps/\napps/mobile/\n")
        self.assertEqual(self.run_helper("status").returncode, 3)

    def test_external_graph_change_and_missing_graph_invalidate(self):
        self.ensure_ok()
        graph = self.root / "graphify-out/graph.json"
        graph.write_text("{}")
        self.assertEqual(self.run_helper("status").returncode, 3)
        graph.unlink()
        self.assertEqual(self.run_helper("status").returncode, 3)

    def test_failed_or_racing_extraction_never_stamps_fresh(self):
        for mode in ("fail", "invalid", "race"):
            with self.subTest(mode=mode):
                self.ensure_ok()
                result = self.run_helper("refresh", mode)
                self.assertEqual(result.returncode, 1, result.stdout + result.stderr)
                self.assertFalse(self.stamp.exists())
                self.assertEqual(self.run_helper("status").returncode, 3)

    def test_concurrent_ensure_extracts_once(self):
        with ThreadPoolExecutor(max_workers=2) as workers:
            results = list(workers.map(lambda _: self.run_helper("ensure", "slow"), range(2)))
        for result in results:
            self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        self.assertEqual((self.root / "graphify-out/calls").read_text(), "1")


if __name__ == "__main__":
    unittest.main()
