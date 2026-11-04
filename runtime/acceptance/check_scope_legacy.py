"""Actual v2 transfer/backup and v3 scope validation, through separate CLI processes."""
import argparse
import contextlib
import copy
import pathlib
import shutil
import sqlite3
import tempfile

from check_cli import Client
from check_episode import two_failures
from check_lifecycle import binding, setup
from check_preparation import package_digest


def policy_row(root):
    with contextlib.closing(sqlite3.connect(root / ".ocmi/local/state.sqlite")) as db:
        return db.execute("SELECT failure_count,reassess_used,review_used,allowance,gate FROM outcome_policy WHERE outcome_id='outcome-A'").fetchone()


def check(binary, v2_binary):
    with tempfile.TemporaryDirectory(prefix="ocmi-scope-legacy-") as temp:
        base = pathlib.Path(temp)
        c = setup(v2_binary, base / "previous transfer", 7)
        two_failures(c)
        before = policy_row(c.root)
        package = c.request("handover.export", dict(transferId="actual-v2", files=["contract.md", "input.txt"]))["package"]
        assert package["schemaVersion"] == 2
        dest = base / "legacy import"; dest.mkdir()
        d = Client(binary, dest)
        d.request("handover.import", dict(package=package, deviceId="laptop", copyId="restored"))
        assert policy_row(dest) == before
        state = d.request("status")
        assert state["completionScope"] == "legacy_unknown" and state["stopped"] is True
        h = d.request("task.history")
        assert all(r["completionScope"] == "legacy_unknown" for r in h["tasks"] + h["revisions"])
        d.request("stop", dict(enabled=False))
        d.request("handover.rebind", dict(command=binding(dest)["command"]))
        d.request("task.ready"); d.request("run", actor="worker-1", ok=False)
        assert not (dest / "effect.txt").exists()
        print("PASS: actual v2 package imports unknown scope, intact complete policy and enforced retry gate")

        c = setup(v2_binary, base / "previous upgrade", 7)
        two_failures(c)
        before = policy_row(c.root)
        d = Client(binary, c.root)
        receipt = d.request("state.upgrade", revision=c.revision)
        assert policy_row(c.root) == before
        backup = pathlib.Path(receipt["backupPath"]).resolve(strict=True)
        with contextlib.closing(sqlite3.connect(backup)) as db:
            assert db.execute("PRAGMA quick_check").fetchone() == ("ok",)
            assert db.execute("SELECT schema_version FROM meta").fetchone() == (2,)
        restored = base / "backup restored"; (restored / ".ocmi/local").mkdir(parents=True)
        shutil.copyfile(backup, restored / ".ocmi/local/state.sqlite")
        old = Client(v2_binary, restored)
        assert old.request("status")["failureCount"] == 2
        assert policy_row(restored) == before
        print("PASS: v2 backup actually restores in previous executable with unchanged whole policy")

        c = setup(binary, base / "current package")
        package = c.request("handover.export", dict(transferId="scope-validation", files=["contract.md", "input.txt"]))["package"]
        assert package_digest(package) == package["digest"], "independent rechecksumming premise failed"
        for case in ("missing-task", "unknown-task", "missing-revision", "mismatch-revision", "mismatch-task"):
            bad = copy.deepcopy(package)
            if case == "missing-task":
                bad["task"].pop("completionScope")
            elif case == "unknown-task":
                bad["task"]["completionScope"] = "invented"
            elif case == "missing-revision":
                bad["revisions"][0].pop("completionScope")
            elif case == "mismatch-revision":
                bad["revisions"][0]["completionScope"] = "preparation"
            else:
                bad["tasks"][0]["completionScope"] = "preparation"
            bad["digest"] = package_digest(bad)
            target = base / case; target.mkdir()
            Client(binary, target).request("handover.import", dict(package=bad, deviceId="x", copyId=case), ok=False)
            assert list(target.iterdir()) == [], case
        print("PASS: rechecksummed missing/unknown/inconsistent v3 scopes rejected without destination writes")


if __name__ == "__main__":
    parser = argparse.ArgumentParser()
    parser.add_argument("--binary", required=True)
    parser.add_argument("--v2-binary", required=True)
    args = parser.parse_args()
    check(str(pathlib.Path(args.binary).resolve(strict=True)), str(pathlib.Path(args.v2_binary).resolve(strict=True)))
