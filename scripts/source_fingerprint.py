#!/usr/bin/env python3
import argparse
import datetime as dt
import hashlib
import json
import pathlib
import sys


INCLUDE_ROOTS = [
    ".github",
    "cmd",
    "deploy",
    "internal",
    "scripts",
    "web",
]

INCLUDE_FILES = [
    ".dockerignore",
    "Dockerfile",
    "go.mod",
    "go.sum",
]

IGNORE_PARTS = {
    ".git",
    ".DS_Store",
    "node_modules",
}

IGNORE_PREFIXES = [
    pathlib.PurePosixPath("docs/evidence"),
    pathlib.PurePosixPath("output"),
    pathlib.PurePosixPath("storage"),
    pathlib.PurePosixPath("tmp"),
]


def rel_posix(path: pathlib.Path, root: pathlib.Path) -> str:
    return path.relative_to(root).as_posix()


def should_ignore(rel: pathlib.PurePosixPath) -> bool:
    if any(part in IGNORE_PARTS for part in rel.parts):
        return True
    if any(part == ".env" or (part.startswith(".env.") and part != ".env.example") for part in rel.parts):
        return True
    return any(rel == prefix or rel.is_relative_to(prefix) for prefix in IGNORE_PREFIXES)


def iter_files(root: pathlib.Path):
    seen = set()

    for file_name in INCLUDE_FILES:
        path = root / file_name
        if path.is_file():
            rel = pathlib.PurePosixPath(file_name)
            if not should_ignore(rel):
                seen.add(file_name)
                yield path

    for root_name in INCLUDE_ROOTS:
        base = root / root_name
        if not base.exists():
            continue
        if base.is_file():
            rel = pathlib.PurePosixPath(root_name)
            if not should_ignore(rel) and root_name not in seen:
                seen.add(root_name)
                yield base
            continue
        for path in sorted(base.rglob("*")):
            if not path.is_file():
                continue
            rel_name = rel_posix(path, root)
            rel = pathlib.PurePosixPath(rel_name)
            if should_ignore(rel) or rel_name in seen:
                continue
            seen.add(rel_name)
            yield path


def build_fingerprint(root: pathlib.Path):
    files = []
    aggregate = hashlib.sha256()
    for path in iter_files(root):
        rel_name = rel_posix(path, root)
        data = path.read_bytes()
        digest = hashlib.sha256(data).hexdigest()
        files.append(
            {
                "path": rel_name,
                "size": len(data),
                "sha256": digest,
            }
        )

    files.sort(key=lambda item: item["path"])
    for item in files:
        aggregate.update(item["path"].encode("utf-8"))
        aggregate.update(b"\0")
        aggregate.update(str(item["size"]).encode("ascii"))
        aggregate.update(b"\0")
        aggregate.update(item["sha256"].encode("ascii"))
        aggregate.update(b"\n")

    return {
        "schema": 1,
        "generated_at": dt.datetime.now().astimezone().strftime("%Y-%m-%d %H:%M:%S %Z"),
        "algorithm": "sha256",
        "fingerprint": aggregate.hexdigest(),
        "file_count": len(files),
        "include_roots": INCLUDE_ROOTS,
        "include_files": INCLUDE_FILES,
        "ignore_prefixes": [prefix.as_posix() for prefix in IGNORE_PREFIXES],
        "ignore_parts": sorted(IGNORE_PARTS),
        "files": files,
    }


def main() -> int:
    parser = argparse.ArgumentParser(description="Generate MoChat Go source and validation fingerprint.")
    parser.add_argument("--root", default=".", help="Repository root. Defaults to current directory.")
    parser.add_argument("--out", help="Write fingerprint JSON to this path instead of stdout.")
    parser.add_argument("--check", help="Compare current fingerprint with an existing fingerprint JSON.")
    args = parser.parse_args()

    root = pathlib.Path(args.root).resolve()
    payload = build_fingerprint(root)
    text = json.dumps(payload, ensure_ascii=False, indent=2, sort_keys=True) + "\n"

    if args.out:
        out_path = pathlib.Path(args.out)
        out_path.parent.mkdir(parents=True, exist_ok=True)
        out_path.write_text(text, encoding="utf-8")
    else:
        sys.stdout.write(text)

    if args.check:
        expected_path = pathlib.Path(args.check)
        expected = json.loads(expected_path.read_text(encoding="utf-8"))
        if expected.get("fingerprint") != payload["fingerprint"]:
            print(
                f"source fingerprint mismatch: expected {expected.get('fingerprint')} current {payload['fingerprint']}",
                file=sys.stderr,
            )
            return 1
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
