#!/usr/bin/env -S uv run --no-project python
"""Verify picker catalog behavior through the built CLI in a private environment."""

import argparse
import hashlib
import json
from pathlib import Path
import shutil
import subprocess
from tempfile import TemporaryDirectory


def require(condition, message):
    if not condition:
        raise RuntimeError(message)


def drive(repo, evidence, runtime):
    binary = repo / "bin/rex-sesh"
    fixture_bin = runtime / "bin"
    fixture_bin.mkdir()
    zoxide = shutil.which("zoxide")
    require(zoxide, "zoxide is required for the catalog journey")
    (fixture_bin / "zoxide").symlink_to(zoxide)
    rex = fixture_bin / "rex"
    rex.write_text("""#!/bin/sh
case "$*" in
  '--autostart=false hosts --json') echo '{"hosts":[{"host":{"label":"fixture-host"}}]}' ;;
  *' do - --args '*)
    case "$*" in
      *' -S fixture-host '*) echo '{"sessions":[{"session_id":"session:2","label":"remote"}]}' ;;
      *) echo '{"sessions":[{"session_id":"session:1","label":"api","current":true}]}' ;;
    esac ;;
  *) exit 64 ;;
esac
""")
    rex.chmod(0o700)
    config = runtime / "config.toml"
    config.write_text('version = 1\n[list]\ncache = false\n[[workspace]]\nname = "configured"\npath = '
                      + json.dumps(str(runtime)) + '\n')
    shutil.copyfile(config, evidence / "fixture.toml")
    env = {
        "PATH": f"{fixture_bin}:/usr/bin:/bin", "HOME": str(runtime),
        "REX_SESH_STATE_DIR": str(runtime / "state"), "REX_SESH_REX_BIN": str(rex),
        "_ZO_DATA_DIR": str(runtime / "zoxide"), "_ZO_EXCLUDE_DIRS": "",
        "REX_BLOCK": "block:fixture",
    }

    def run(label, *args):
        with (evidence / "commands.jsonl").open("a") as log:
            print(json.dumps([label, *map(str, args)]), file=log)
        with (evidence / f"{label}.out").open("wb") as out, (evidence / f"{label}.err").open("wb") as err:
            result = subprocess.run(args, env=env, cwd=runtime, stdout=out, stderr=err, timeout=30)
        (evidence / f"{label}.exit").write_text(f"{result.returncode}\n")
        result.check_returncode()
        return (evidence / f"{label}.out").read_text()

    run("revision", "git", "-C", repo, "rev-parse", "HEAD")
    (evidence / "binary.sha256").write_text(hashlib.sha256(binary.read_bytes()).hexdigest() + "\n")
    run("version", binary, "--version")
    run("config-valid", binary, "config", "validate", config)
    for i, path in enumerate(("a/docs", "b/docs", "elsewhere/api")):
        directory = runtime / path
        directory.mkdir(parents=True)
        run(f"seed-{i}", "zoxide", "add", directory)
    before = run("zoxide-before", "zoxide", "query", "-l")
    listed = json.loads(run("list", binary, "list", "--json", "--config", config))
    require(any(r["source"] == "config" and r["name"] == "configured" for r in listed), "list lost config row")
    rows = json.loads(run("picker", binary, "picker", "--json", "--config", config))
    require([(r["source"], r["name"], r.get("machine"), r.get("session_id")) for r in rows[:2]]
            == [("rex", "api", None, "session:1"), ("rex", "remote", "fixture-host", "session:2")],
            "local and host rows differ from fixture")
    require(rows[0].get("attached") is True, "current marker missing")
    require(len(rows) == 4 and all(r["source"] == "zoxide" for r in rows[2:]), "unexpected directory rows")
    require({r["name"]: r["path"] for r in rows[2:]} == {p: str(runtime / p) for p in ("a/docs", "b/docs")},
            "directory naming or deduplication failed")
    require(set(before.splitlines()) == set(run("zoxide-after", "zoxide", "query", "-l").splitlines()),
            "registered directories changed")
    require(config.read_bytes() == (evidence / "fixture.toml").read_bytes(), "config changed")


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("repo", type=Path)
    parser.add_argument("evidence", type=Path, help="new evidence directory")
    args = parser.parse_args()
    evidence = args.evidence.resolve()
    evidence.mkdir(mode=0o700, parents=True, exist_ok=False)
    with TemporaryDirectory(dir=evidence, prefix="runtime-") as scratch:
        drive(args.repo.resolve(), evidence, Path(scratch))
    (evidence / "passed").write_text("catalog passed; scratch removed\n")
    print(evidence)


if __name__ == "__main__":
    main()
