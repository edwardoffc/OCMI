"""Actual v3 upgrade and portable resource obligations, without native ownership transfer."""
import argparse
import contextlib
import copy
import hashlib
import json
import pathlib
import shutil
import sqlite3
import tempfile

from check_cli import Client, scenario
from check_lifecycle import binding, setup
from check_resources import attempt, close, declare, finish, record
from check_preparation import package_digest


def check(binary, v3_binary):
    with tempfile.TemporaryDirectory(prefix="ocmi-resource-portable-") as temp:
        base = pathlib.Path(temp)
        old = setup(v3_binary, base / "actual v3")
        old.request("run", actor="worker-1"); old.request("submit", actor="worker-1")
        old.request("review", dict(accept=True, reason="Real prior result."))
        old.request("integrate", dict(candidate="v3-result", reason="Prior checked fixture."))
        old.request("resource.close", dict(disposition="keep", reason="Prior evidence."))
        history = old.request("task.history")
        store = old.root / ".ocmi/local/state.sqlite"
        digest = hashlib.sha256(store.read_bytes()).hexdigest()
        new = Client(binary, old.root)
        new.request("status", ok=False)
        assert hashlib.sha256(store.read_bytes()).hexdigest() == digest
        upgraded = new.request("state.upgrade", key="v3-upgrade", revision=old.revision)
        assert new.request("state.upgrade", key="v3-upgrade", revision=0) == upgraded
        assert new.request("task.history") == history
        assert new.request("status")["resourceSummary"]["legacyCoverageUnknown"] is True
        backup = pathlib.Path(upgraded["backupPath"]).resolve(strict=True)
        with contextlib.closing(sqlite3.connect(backup)) as db:
            assert db.execute("SELECT schema_version FROM meta").fetchone() == (3,)
            assert db.execute("PRAGMA quick_check").fetchone() == ("ok",)
        restored = base / "backup restored"; (restored / ".ocmi/local").mkdir(parents=True)
        shutil.copyfile(backup, restored / ".ocmi/local/state.sqlite")
        assert Client(v3_binary, restored).request("status")["taskState"] == "integrated"
        old.request("status", ok=False)
        print("PASS: actual v3 owner upgrade preserves history, old-readable backup and explicit unknown legacy coverage")

        c = scenario(binary, base / "source")
        declare(c, "kept", "kept")
        c.request("resource.apply", dict(resourceId="kept"))
        c.request("resource.write", dict(resourceId="kept", path="proof.txt", contentBase64="ZmljdGlvbmFsCg=="), actor="worker-1")
        native = record(c, "kept")["nativeIdentity"]
        c.request("resource.dispose", dict(resourceId="kept", disposition="keep", reason="Required recovery material.", until="verified destination recovery"))
        logical_query = "SELECT purpose,lifetime,intended_disposition,owner_ref,depends_json,held,hold_reason,hold_until,task_id,task_revision FROM resource_ledger WHERE id='kept'"
        with contextlib.closing(sqlite3.connect(c.root / ".ocmi/local/state.sqlite")) as db:
            logical_before = db.execute(logical_query).fetchone()
        assert "verified destination recovery" in logical_before, "retention condition was not recorded"
        finish(c); close(c)
        package = c.request("handover.export", dict(transferId="resource-portable", files=["contract.md", "input.txt", "kept/proof.txt"]))["package"]
        assert package["schemaVersion"] == 4
        # Existing quarantined command history is not a new native resource binding.
        resource_fields = {key: package.get(key) for key in ("resourceLedger", "resourceUses", "resourceEvents")}
        encoded = json.dumps(resource_fields)
        def strings(value):
            if isinstance(value, str):
                yield value.replace("\\", "/").casefold()
            elif isinstance(value, dict):
                for item in value.values():
                    yield from strings(item)
            elif isinstance(value, list):
                for item in value:
                    yield from strings(item)
        roots = (str(c.root).replace("\\", "/").casefold(), str(c.root.resolve()).replace("\\", "/").casefold())
        assert all(not any(prefix in value for prefix in roots) for value in strings(resource_fields))
        assert native not in encoded, "native identity escaped into portable resource record"
        assert any(r["resourceId"] == "kept" for r in package["resourceLedger"])
        assert package_digest(package) == package["digest"], "resource checksum premise differs"
        for case in ("duplicate-resource", "missing-dependency", "unknown-use", "unknown-event"):
            bad = copy.deepcopy(package)
            if case == "duplicate-resource":
                bad["resourceLedger"].append(copy.deepcopy(bad["resourceLedger"][0]))
            elif case == "missing-dependency":
                next(r for r in bad["resourceLedger"] if r["resourceId"] == "kept")["dependsOn"] = ["missing-resource"]
            elif case == "unknown-use":
                bad["resourceUses"][0]["resourceId"] = "missing-resource"
            else:
                bad["resourceEvents"][0]["resourceId"] = "missing-resource"
            bad["digest"] = package_digest(bad)
            invalid = base / case; invalid.mkdir()
            refused = attempt(Client(binary, invalid), "handover.import", dict(package=bad, deviceId="laptop", copyId="bad"))
            assert refused["ok"] is False and refused["code"] == "INVALID_PACKAGE", (case, refused)
            assert not (invalid / ".ocmi").exists(), "malformed resource transfer partially wrote destination"
        dest = base / "other device"; dest.mkdir()
        d = Client(binary, dest)
        d.request("handover.import", dict(package=package, deviceId="laptop", copyId="new-copy"))
        assert (dest / "kept/proof.txt").read_bytes() == b"fictional\n"
        assert d.request("status")["stopped"] is True
        with contextlib.closing(sqlite3.connect(dest / ".ocmi/local/state.sqlite")) as db:
            assert db.execute(logical_query).fetchone() == logical_before, "resource obligations/task association changed in transit"
        imported = record(d, "kept")
        assert imported["observedState"] == "needs_rebind" and not imported.get("nativeIdentity")
        d.request("resource.dispose", dict(resourceId="kept", disposition="remove", reason="Copied record is not ownership."), ok=False)
        d.request("resource.rebind", dict(resourceId="kept", locator="kept", ownerRef="restored-data-owner", reason="Observed destination copy, use only."))
        assert record(d, "kept")["ownership"] == "external"
        d.request("resource.dispose", dict(resourceId="kept", disposition="remove", reason="Use remains external."), ok=False)
        assert (dest / "kept/proof.txt").read_bytes() == b"fictional\n"
        d.request("handover.rebind", dict(command=binding(dest)["command"]))
        d.request("stop", dict(enabled=False)); d.request("task.ready")
        d.request("run", actor="worker-1")
        print("PASS: resource purpose/obligations transfer without local paths, native identity or destination deletion authority")

        old = setup(v3_binary, base / "actual v3 package")
        old.request("run", actor="worker-1")
        package = old.request("handover.export", dict(transferId="v3-old", files=["contract.md", "input.txt"]))["package"]
        assert package["schemaVersion"] == 3
        dest = base / "old package destination"; dest.mkdir()
        d = Client(binary, dest)
        d.request("handover.import", dict(package=package, deviceId="new", copyId="new"))
        assert d.request("status")["completionScope"] == "target"
        assert d.request("status")["resourceSummary"]["legacyCoverageUnknown"] is True
        print("PASS: actual v3 digest/scope compatibility preserves unqualified old resource coverage honestly")


if __name__ == "__main__":
    parser = argparse.ArgumentParser(); parser.add_argument("--binary", required=True); parser.add_argument("--v3-binary", required=True)
    args = parser.parse_args(); check(str(pathlib.Path(args.binary).resolve(strict=True)), str(pathlib.Path(args.v3_binary).resolve(strict=True)))
