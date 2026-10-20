"""Maintain KeepUp's application graph. Execute inside the Graphify container."""
from __future__ import annotations

import argparse
import fcntl
import hashlib
import importlib.metadata
import json
import os
from pathlib import Path
import subprocess
import sys
import tempfile

SCHEMA = 1


def git(root: Path, *args: str, optional: bool = False) -> bytes:
    result = subprocess.run(["git", "-C", str(root), *args], capture_output=True)
    if result.returncode and not optional:
        raise RuntimeError(result.stderr.decode(errors="replace").strip())
    return result.stdout if result.returncode == 0 else b""


def digest(path: Path) -> str:
    h = hashlib.sha256()
    with path.open("rb") as stream:
        for chunk in iter(lambda: stream.read(1024 * 1024), b""):
            h.update(chunk)
    return h.hexdigest()


def fingerprint(root: Path) -> str:
    """Hash source contents, not only HEAD, mtimes, or the list of dirty paths."""
    paths = set(git(root, "ls-files", "-z", "--cached", "--others", "--exclude-standard", "--", "apps").split(b"\0"))
    h = hashlib.sha256()
    for raw in sorted(paths - {b""}):
        path = root / os.fsdecode(raw)
        if not path.is_file() and not path.is_symlink():
            continue  # Tracked deletions must change the snapshot too.
        h.update(raw + b"\0")
        if path.is_symlink():
            h.update(b"symlink:" + os.fsencode(os.readlink(path)))
        else:
            h.update(digest(path).encode())
        h.update(b"\0")
    config = [root / name for name in (".gitignore", ".graphifyignore", ".graphifyrc", "Dockerfile.graphify")]
    config.append(Path(__file__).resolve())
    # Git's local excludes can affect untracked discovery and Graphify detection.
    excludes = git(root, "rev-parse", "--git-path", "info/exclude").decode().strip()
    config.append(Path(excludes) if Path(excludes).is_absolute() else root / excludes)
    for path in config:
        h.update(path.name.encode() + b"\0")
        h.update(digest(path).encode() if path.is_file() else b"missing")
    h.update(importlib.metadata.version("graphifyy").encode())
    h.update(str(SCHEMA).encode())
    return h.hexdigest()


def freshness(root: Path, out: Path) -> tuple[bool, str]:
    graph = out / "graph.json"
    if not graph.is_file():
        return False, "graph missing"
    try:
        stamp = json.loads((out / ".keepup-index.json").read_text())
        if not isinstance(stamp, dict) or stamp.get("schema") != SCHEMA:
            return False, "freshness stamp missing or incompatible"
        if stamp.get("fingerprint") != fingerprint(root):
            return False, "application sources or indexing configuration changed"
        if stamp.get("graph_hash") != digest(graph):
            return False, "graph changed outside the freshness helper"
    except (OSError, ValueError):
        return False, "freshness stamp missing or unreadable"
    return True, "application index fresh"


def validate_graph(graph: Path) -> None:
    from graphify.build import graph_has_legacy_ids
    data = json.loads(graph.read_text())
    nodes = data.get("nodes", [])
    edges = data.get("links", data.get("edges", []))
    if not nodes or not isinstance(nodes, list) or not isinstance(edges, list):
        raise RuntimeError("extraction did not produce a nonempty graph")
    ids = {n["id"] for n in nodes}
    if len(ids) != len(nodes) or any(e["source"] not in ids or e["target"] not in ids for e in edges):
        raise RuntimeError("extraction produced duplicate IDs or dangling edges")
    if graph_has_legacy_ids(nodes):
        raise RuntimeError("extraction produced inconsistent node-ID roots")


def update(root: Path, out: Path, force: bool = False) -> bool:
    out.mkdir(parents=True, exist_ok=True)
    with (out / ".update.lock").open("a") as lock:
        fcntl.flock(lock, fcntl.LOCK_EX)
        fresh, reason = freshness(root, out)
        if fresh and not force:
            print("Graphify: application index fresh.")
            return False
        stamp = out / ".keepup-index.json"
        stamp.unlink(missing_ok=True)
        before = fingerprint(root)
        print(f"Graphify: refreshing ({reason if not force else 'explicit refresh'}).", flush=True)
        command = ["graphify", "extract", ".", "--code-only"]
        if force:
            command.append("--force")
        result = subprocess.run(
            command,
            cwd=root,
            env={**os.environ, "GRAPHIFY_OUT": str(out.resolve()), "GRAPHIFY_BACKEND": "none"},
        )
        if result.returncode:
            raise RuntimeError(f"extraction exited {result.returncode}; index may be stale")
        validate_graph(out / "graph.json")
        after = fingerprint(root)
        if before != after:
            raise RuntimeError("sources changed during extraction; retry when edits settle")
        value = {"schema": SCHEMA, "fingerprint": after, "graph_hash": digest(out / "graph.json"),
                 "git_revision": git(root, "rev-parse", "HEAD", optional=True).decode().strip() or None}
        fd, name = tempfile.mkstemp(prefix=".keepup-index-", dir=out)
        try:
            with os.fdopen(fd, "w") as stream:
                json.dump(value, stream)
                stream.write("\n")
            os.replace(name, stamp)
        finally:
            Path(name).unlink(missing_ok=True)
        print("Graphify: application index refreshed and verified.")
        return True


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("command", choices=("status", "ensure", "refresh"))
    args = parser.parse_args()
    root = Path.cwd().resolve()
    out = root / "graphify-out"
    try:
        if args.command == "status":
            fresh, reason = freshness(root, out)
            print(f"Graphify: {reason}.")
            return 0 if fresh else 3
        update(root, out, force=args.command == "refresh")
        return 0
    except (OSError, ValueError, KeyError, TypeError, RuntimeError) as exc:
        print(f"Graphify: {exc}", file=sys.stderr)
        return 1


if __name__ == "__main__":
    raise SystemExit(main())
