#!/usr/bin/env bash
set -euo pipefail

python3 - <<'PY'
import os
import pathlib
import shlex
import subprocess

CURRENT_PID = os.getpid()
SCRIPT_NAME = "standalone_soak_24h.sh"
IGNORED_EXE = {
    "git",
    "grep",
    "pgrep",
    "python",
    "python3",
    "rg",
}


def split_command(command: str) -> list[str]:
    try:
        return shlex.split(command)
    except ValueError:
        return command.split()


def basename(value: str) -> str:
    return pathlib.PurePosixPath(value).name


ps_result = subprocess.run(
    ["ps", "-axo", "pid=,command="],
    text=True,
    stdout=subprocess.PIPE,
    stderr=subprocess.DEVNULL,
    check=False,
)

for raw_line in ps_result.stdout.splitlines():
    stripped = raw_line.strip()
    if not stripped:
        continue
    pid_text, _, command = stripped.partition(" ")
    try:
        pid = int(pid_text)
    except ValueError:
        continue
    if pid == CURRENT_PID:
        continue

    args = split_command(command)
    if not args:
        continue
    exe = basename(args[0])
    if exe in IGNORED_EXE:
        continue

    token_match = any(basename(arg) == SCRIPT_NAME for arg in args)
    shell_match = exe in {"bash", "env", "sh", "zsh"} and SCRIPT_NAME in command
    if token_match or shell_match:
        print(stripped)
PY
