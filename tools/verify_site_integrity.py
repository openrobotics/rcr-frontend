#!/usr/bin/env python3
# Copyright 2026 Open Source Robotics Foundation, Inc.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.
"""Verify that an assembled registry site matches its own source.json hashes.

Bazel verifies every overlay/ and patches/ file it fetches from a registry
against the `integrity` declared in that module version's source.json. If the
deploy pipeline ever publishes a file whose bytes differ from the registry
commit (e.g. a tool rewrote a BUILD file in the checkout), consumers fail
with a checksum mismatch - and if the module is something like rosdistro,
*everything* fails. This script is the last line of defense: run it over the
assembled ./site before upload and fail the job on any mismatch.

Usage:
    verify_site_integrity.py <site-dir>/modules [--registry <registry-dir>]

Checks, for every <modules>/<name>/<version>/source.json:
  * each overlay/<path> and patches/<path> entry exists and its
    sha256 matches the declared "sha256-<base64>" integrity;
  * with --registry, that MODULE.bazel and source.json are byte-identical to
    the committed copies (git show HEAD:<path>), covering files that are
    served but not hashed by source.json.
Exit status is non-zero if anything mismatches.
"""

import argparse
import base64
import hashlib
import json
import os
import subprocess
import sys


def integrity(data: bytes) -> str:
    return "sha256-" + base64.b64encode(hashlib.sha256(data).digest()).decode()


def read(path: str):
    try:
        with open(path, "rb") as f:
            return f.read()
    except FileNotFoundError:
        return None


class CommittedTree:
    """Reads committed blobs from a git checkout via one `git cat-file --batch`."""

    def __init__(self, registry: str):
        self.proc = subprocess.Popen(
            ["git", "-C", registry, "cat-file", "--batch"],
            stdin=subprocess.PIPE,
            stdout=subprocess.PIPE,
        )

    def _get(self, spec: str):
        self.proc.stdin.write((spec + "\n").encode())
        self.proc.stdin.flush()
        header = self.proc.stdout.readline().decode().split()
        if len(header) != 3:  # "<sha> missing"
            return None
        size = int(header[2])
        data = self.proc.stdout.read(size)
        self.proc.stdout.read(1)  # trailing newline
        return data

    def get(self, rel: str):
        """Return committed bytes of <rel> from HEAD, trying modules/ then bcr_staging/."""
        for tree_rel in (rel, os.path.join("bcr_staging", rel)):
            data = self._get(f"HEAD:{tree_rel}")
            if data is not None:
                return data
        return None

    def close(self):
        self.proc.stdin.close()
        self.proc.wait()


def main() -> int:
    ap = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    ap.add_argument("modules", help="path to the assembled site's modules/ directory")
    ap.add_argument("--registry", help="path to the registry git checkout to compare unhashed files against")
    args = ap.parse_args()

    problems = []
    versions = 0
    hashed_files = 0
    committed = CommittedTree(args.registry) if args.registry else None

    for name in sorted(os.listdir(args.modules)):
        mod_dir = os.path.join(args.modules, name)
        if not os.path.isdir(mod_dir):
            continue
        for version in sorted(os.listdir(mod_dir)):
            ver_dir = os.path.join(mod_dir, version)
            src_path = os.path.join(ver_dir, "source.json")
            if not os.path.isfile(src_path):
                continue
            versions += 1
            try:
                src = json.loads(read(src_path))
            except json.JSONDecodeError as e:
                problems.append(f"{name}@{version}: source.json is not valid JSON: {e}")
                continue

            for kind in ("overlay", "patches"):
                for rel, want in (src.get(kind) or {}).items():
                    hashed_files += 1
                    data = read(os.path.join(ver_dir, kind, rel))
                    if data is None:
                        problems.append(f"{name}@{version}: {kind}/{rel} declared in source.json but missing from site")
                        continue
                    got = integrity(data)
                    if got != want:
                        problems.append(
                            f"{name}@{version}: {kind}/{rel} integrity mismatch\n"
                            f"    declared {want}\n"
                            f"    served   {got}"
                        )

            if committed:
                for fname in ("MODULE.bazel", "source.json"):
                    rel = f"modules/{name}/{version}/{fname}"
                    want = committed.get(rel)
                    served = read(os.path.join(ver_dir, fname))
                    if want is None:
                        problems.append(f"{name}@{version}: {fname} is served but not committed in the registry")
                    elif served != want:
                        problems.append(f"{name}@{version}: {fname} differs from the committed registry copy")

    if committed:
        committed.close()

    print(f"checked {versions} module versions, {hashed_files} hashed overlay/patch files")
    if problems:
        print(f"\n{len(problems)} problem(s):", file=sys.stderr)
        for p in problems:
            print("  " + p, file=sys.stderr)
        return 1
    print("OK: site content matches source.json integrity hashes")
    return 0


if __name__ == "__main__":
    sys.exit(main())
