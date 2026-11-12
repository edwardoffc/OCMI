"""Independent resource lifecycle checks through the real CLI, using fictional roots."""
import argparse
import base64
import hashlib
import pathlib
import shutil
import subprocess
import tempfile
import uuid

from check_cli import Client, decode, scenario


def declare(c, resource_id, locator, kind="directory", mode="create", **extra):
    return c.request("resource.intent", dict(resourceId=resource_id, provider="local", kind=kind,
        mode=mode, locator=str(locator), purpose="Fictional acceptance resource", lifetime="until task close",
        disposition="remove" if mode == "create" else "release", ownerRef="fixture-owner", **extra))


def record(c, resource_id):
    return c.request("resource.inspect", dict(resourceId=resource_id))


def finish(c):
    c.request("run", actor="worker-1"); c.request("submit", actor="worker-1")
    c.request("review", dict(accept=True, reason="Fictional checked target."))
    c.request("integrate", dict(candidate="resource-fixture", reason="Actual fixture checked."))


def close(c, ok=True):
    return c.request("resource.close", dict(disposition="keep", reason="Retain fixture evidence."), ok=ok)


def attempt(c, action, payload):
    import json
    request = dict(schemaVersion=1, action=action, projectRoot=str(c.root), actor="main",
                   requestId=str(uuid.uuid4()), expectedRevision=c.revision, payload=payload)
    result = decode(subprocess.run([c.binary, "request"], input=json.dumps(request), text=True,
                                  encoding="utf-8", capture_output=True, timeout=15))
    c.revision = max(c.revision, result["revision"])
    return result


def check(binary):
    with tempfile.TemporaryDirectory(prefix="ocmi-resources-") as temp:
        base = pathlib.Path(temp)
        c = scenario(binary, base / "owned project Ω")
        declare(c, "scratch", "scratch")
        assert not (c.root / "scratch").exists(), "declaration caused a filesystem effect"
        revision = c.revision
        listed = c.request("resource.list")
        assert c.revision == revision and any(r["resourceId"] == "scratch" for r in listed["resources"])
        c.request("resource.apply", dict(resourceId="scratch"), actor="worker-1", ok=False)
        c.request("resource.apply", dict(resourceId="scratch"), key="create-once")
        assert (c.root / "scratch").is_dir()
        before = record(c, "scratch")
        c.request("resource.apply", dict(resourceId="scratch"), key="create-once", revision=0)
        assert record(c, "scratch")["nativeIdentity"] == before["nativeIdentity"]
        text = b"fictional artifact\n"
        c.request("resource.write", dict(resourceId="scratch", path="proof.txt",
                    contentBase64=base64.b64encode(text).decode()), actor="worker-1", key="write-once")
        assert (c.root / "scratch/proof.txt").read_bytes() == text
        stored = (c.root / ".ocmi/local/state.sqlite").read_bytes()
        assert text not in stored and base64.b64encode(text) not in stored, "resource journal copied file contents"
        c.request("resource.use", dict(resourceId="scratch", access="write", reason="Bounded task output."), actor="worker-1")
        c.request("resource.dispose", dict(resourceId="scratch", disposition="remove", reason="Still leased."), ok=False)
        finish(c)
        close(c, ok=False)
        c.request("resource.release", dict(resourceId="scratch", reason="Owner finishes task use after verified operation stop."))
        c.request("resource.hold", dict(resourceId="scratch", enabled=True, reason="Preserve evidence.", until="review completes"))
        c.request("resource.dispose", dict(resourceId="scratch", disposition="remove", reason="Held."), ok=False)
        assert (c.root / "scratch/proof.txt").read_bytes() == text
        c.request("resource.hold", dict(resourceId="scratch", enabled=False, reason="Review complete.", until="now"))
        c.request("resource.dispose", dict(resourceId="scratch", disposition="remove", reason="Exact owned disposable output."))
        assert not (c.root / "scratch").exists()
        assert record(c, "scratch")["observedState"] == "absent"
        c.request("stop", dict(enabled=True))
        close(c)
        assert c.request("status")["cleanup"] == "done"
        print("PASS: pre-effect intent, owner/replay, grouped output, lease/hold guards and verified actual removal")

        shared = base / "foreign shared cache"; shared.mkdir()
        protected = shared / "foreign.txt"; protected.write_bytes(b"must remain\n")
        digest = hashlib.sha256(protected.read_bytes()).hexdigest()
        c = scenario(binary, base / "borrower")
        declare(c, "shared", shared, mode="use")
        c.request("resource.apply", dict(resourceId="shared"))
        c.request("resource.use", dict(resourceId="shared", access="read", reason="Borrow only."), actor="worker-1")
        assert record(c, "shared")["ownership"] == "external"
        c.request("resource.dispose", dict(resourceId="shared", disposition="remove", reason="Use is not ownership."), ok=False)
        c.request("resource.release", dict(resourceId="shared", reason="Own usage ended."), actor="worker-1")
        finish(c); close(c)
        assert shared.is_dir() and hashlib.sha256(protected.read_bytes()).hexdigest() == digest
        print("PASS: verified shared use/release preserves real external ownership and foreign data")

        c = scenario(binary, base / "exclusive files and transfer")
        foreign_file = c.root / "preexisting.txt"; foreign_file.write_bytes(b"foreign data")
        intended = attempt(c, "resource.intent", dict(resourceId="preexisting", provider="local", kind="file", mode="create",
                           locator="preexisting.txt", purpose="Exclusive create control", lifetime="task", disposition="remove", ownerRef="main"))
        if intended["ok"]:
            c.request("resource.apply", dict(resourceId="preexisting", contentBase64="bmV3"), ok=False)
            c.request("resource.dispose", dict(resourceId="preexisting", disposition="remove", reason="A failed create did not acquire ownership."), ok=False)
            c.request("resource.cancel", dict(resourceId="preexisting", reason="Exclusive create produced no effect."))
        assert foreign_file.read_bytes() == b"foreign data"
        declare(c, "handoff-file", "handoff.txt", kind="file")
        c.request("resource.apply", dict(resourceId="handoff-file", contentBase64="c2FtcGxlCg=="))
        assert (c.root / "handoff.txt").read_bytes() == b"sample\n"
        finish(c)
        c.request("resource.dispose", dict(resourceId="handoff-file", disposition="transfer", transferId="resource-transfer",
                    reason="Selected destination checkpoint.", until="matching acknowledgment"))
        assert record(c, "handoff-file")["held"] is True
        close(c, ok=False)
        package = c.request("handover.export", dict(transferId="resource-transfer", files=["contract.md", "input.txt", "handoff.txt"]))["package"]
        c.request("handover.ack", dict(transferId="wrong", generation=package["generation"]), ok=False)
        assert record(c, "handoff-file")["held"] is True
        c.request("handover.ack", dict(transferId="resource-transfer", generation=package["generation"]))
        transferred = record(c, "handoff-file")
        assert transferred["disposition"] == "transferred" and transferred["held"] is False, transferred
        assert (c.root / "handoff.txt").read_bytes() == b"sample\n", "metadata acknowledgment deleted source data"
        print("PASS: exclusive file creation preserves preexisting data; transfer hold clears only on the exact acknowledgment")

        c = scenario(binary, base / "changed inventory")
        declare(c, "scratch", "scratch"); c.request("resource.apply", dict(resourceId="scratch"))
        unknown = c.root / "scratch/unknown.txt"; unknown.write_bytes(b"unclassified must survive")
        c.request("resource.dispose", dict(resourceId="scratch", disposition="remove", reason="Unknown data."), ok=False)
        assert unknown.read_bytes() == b"unclassified must survive"
        print("PASS: unclassified content cannot be recursively deleted by a resource label")

        c = scenario(binary, base / "replacement identity")
        declare(c, "scratch", "scratch"); c.request("resource.apply", dict(resourceId="scratch"))
        moved = c.root / "original"; (c.root / "scratch").rename(moved)
        shutil.copytree(moved, c.root / "scratch")
        c.request("resource.dispose", dict(resourceId="scratch", disposition="remove", reason="Copied marker is insufficient."), ok=False)
        assert moved.is_dir() and (c.root / "scratch").is_dir()
        print("PASS: copied marker/path cannot impersonate a different native directory")

        c = scenario(binary, base / "gates and unsupported tool")
        c.request("stop", dict(enabled=True))
        declare_payload = dict(resourceId="blocked", provider="local", kind="directory", mode="create",
            locator="blocked", purpose="Blocked fixture", lifetime="task", disposition="remove", ownerRef="main")
        c.request("resource.intent", declare_payload, ok=False)
        c.request("stop", dict(enabled=False))
        for resource_id, path in (("escape", "../outside"), ("state", ".ocmi/local"), ("git", ".git")):
            p = {**declare_payload, "resourceId": resource_id, "locator": path}
            c.request("resource.intent", p, ok=False)
        c.request("resource.intent", dict(resourceId="future", provider="fictional-provider", kind="vm",
            mode="create", locator="fictional-machine", purpose="Unsupported provider fixture",
            lifetime="task", disposition="remove", ownerRef="main"))
        c.request("resource.apply", dict(resourceId="future"), ok=False)
        c.request("run", actor="worker-1", ok=False)
        assert not (c.root / "effect.txt").exists()
        c.request("resource.cancel", dict(resourceId="future", reason="No supported effect was issued."))
        # Refusing the impossible intent early is as safe as refusing before effect.
        dependency = attempt(c, "resource.intent", dict(resourceId="missing-dependency",
            provider="local", kind="directory", mode="create", locator="dependent",
            purpose="Missing dependency fixture", lifetime="task", disposition="remove",
            ownerRef="main", dependsOn=["not-declared"]))
        if dependency["ok"]:
            c.request("resource.apply", dict(resourceId="missing-dependency"), ok=False)
            c.request("resource.cancel", dict(resourceId="missing-dependency", reason="Unissued intent cancelled."))
        assert not (c.root / "dependent").exists()
        finish(c); close(c)
        print("PASS: STOP, protected placement, missing dependencies and unqualified provider block effects and false completion")

        c = scenario(binary, base / "automatic process records")
        receipt = c.request("run", actor="worker-1")
        rows = c.request("resource.list", dict(limit=100))["resources"]
        process = [r for r in rows if r.get("operationId") == receipt["operationId"] and r["kind"] == "process"]
        assert len(process) == 1 and process[0]["observedState"] == "stopped", process
        assert any(r.get("automatic") and r["kind"] == "file" and r["ownership"] == "external" for r in rows), rows
        c.request("submit", actor="worker-1"); c.request("review", dict(accept=True, reason="Actual command checked."))
        c.request("integrate", dict(candidate="auto", reason="Actual bounded fixture.")); close(c)
        print("PASS: runner automatically accounts for its real process and borrowed tools, with observed stop and released usage")

        c = scenario(binary, base / "exclusive usage")
        declare(c, "lease", "lease"); c.request("resource.apply", dict(resourceId="lease"))
        c.request("resource.use", dict(resourceId="lease", access="read", reason="Owner reading."))
        c.request("resource.use", dict(resourceId="lease", access="write", reason="Writer cannot overlap reader."), actor="worker-1", ok=False)
        c.request("resource.release", dict(resourceId="lease", reason="Owner finished reading."))
        c.request("resource.use", dict(resourceId="lease", access="write", reason="Exclusive bounded writer."), actor="worker-1")
        c.request("resource.use", dict(resourceId="lease", access="read", reason="Reader cannot overlap writer."), ok=False)
        c.request("resource.release", dict(resourceId="lease", reason="Writer stopped."), actor="worker-1")
        c.request("resource.use", dict(resourceId="lease", access="read", reason="Same actor borrows again."), actor="worker-1")
        c.request("resource.release", dict(resourceId="lease", reason="Borrowing finished."), actor="worker-1")
        c.request("resource.dispose", dict(resourceId="lease", disposition="remove", reason="Verified no users."))
        finish(c); close(c)
        print("PASS: exclusive write conflicts in both directions; released use can be acquired again")

        c = scenario(binary, base / "close observes retained identity")
        declare(c, "kept", "kept"); c.request("resource.apply", dict(resourceId="kept"))
        c.request("resource.dispose", dict(resourceId="kept", disposition="keep", reason="Retained evidence.", until="owner review"))
        finish(c)
        (c.root / "kept").rename(c.root / "original-kept")
        (c.root / "kept").mkdir(); (c.root / "kept/foreign.txt").write_bytes(b"preserve replacement")
        assert record(c, "kept")["pending"] is True
        close(c, ok=False)
        assert (c.root / "kept/foreign.txt").read_bytes() == b"preserve replacement"
        assert (c.root / "original-kept").is_dir()
        print("PASS: completion rechecks retained native identity and preserves replacement data")

        from check_resource_process import project
        code = "import pathlib,sys; (pathlib.Path(sys.argv[1])/'managed/generated.txt').write_text('managed output',encoding='utf-8')"
        c = project(binary, base / "declared write group", code)
        declare(c, "managed", "managed"); c.request("resource.apply", dict(resourceId="managed"))
        c.request("resource.use", dict(resourceId="managed", access="write", reason="Declared exclusive command output."), actor="worker-1")
        c.request("run", actor="worker-1")
        assert (c.root / "managed/generated.txt").read_text(encoding="utf-8") == "managed output"
        c.request("resource.release", dict(resourceId="managed", reason="Verified stopped writer."), actor="worker-1")
        c.request("resource.dispose", dict(resourceId="managed", disposition="remove", reason="Captured declared command output."))
        assert not (c.root / "managed").exists()
        c.request("submit", actor="worker-1"); c.request("review", dict(accept=True, reason="Actual group result checked."))
        c.request("integrate", dict(candidate="managed-fixture", reason="Verified declared output.")); close(c)

        c = project(binary, base / "changed write group", "import pathlib,sys; (pathlib.Path(sys.argv[1])/'effect.txt').write_text('must not run')")
        declare(c, "managed", "managed"); c.request("resource.apply", dict(resourceId="managed"))
        c.request("resource.use", dict(resourceId="managed", access="write", reason="Exclusive declared output."), actor="worker-1")
        (c.root / "managed/foreign.txt").write_bytes(b"unclassified input")
        c.request("run", actor="worker-1", ok=False)
        assert not (c.root / "effect.txt").exists()
        assert (c.root / "managed/foreign.txt").read_bytes() == b"unclassified input"
        print("PASS: declared write groups are prevalidated and captured; unknown contents prevent execution/adoption")


if __name__ == "__main__":
    parser = argparse.ArgumentParser(); parser.add_argument("--binary", required=True)
    args = parser.parse_args(); check(str(pathlib.Path(args.binary).resolve(strict=True)))
