"""Independent registered-recovery checks against real CLI and owned subprocesses."""
import argparse
import json
import pathlib
import shutil
import subprocess
import sys
import tempfile
import time
import uuid

from check_cli import Client
from check_lifecycle import binding


def request_process(c, key):
    request = dict(schemaVersion=1, action="run", projectRoot=str(c.root), actor="worker-1",
                   requestId=key, expectedRevision=c.revision, payload={})
    p = subprocess.Popen([c.binary, "request"], stdin=subprocess.PIPE, stdout=subprocess.PIPE,
                         stderr=subprocess.PIPE, text=True, encoding="utf-8")
    p.stdin.write(json.dumps(request))
    p.stdin.close()
    return p


def eventually(fn, description, timeout=10):
    deadline = time.monotonic() + timeout
    while time.monotonic() < deadline:
        value = fn()
        if value:
            return value
        time.sleep(0.04)
    raise AssertionError(description)


def check(binary):
    with tempfile.TemporaryDirectory(prefix="ocmi-recovery-") as temp:
        base = pathlib.Path(temp)
        foreign_marker = base / "foreign-heartbeat.txt"
        foreign = subprocess.Popen([sys.executable, __file__, "--heartbeat", str(foreign_marker)],
                                   stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
        owned = None
        try:
            eventually(lambda: foreign_marker.exists(), "foreign control did not start")
            root = base / "selected project Ω"
            root.mkdir()
            (root / "contract.md").write_text("Fictional interrupted operation.", encoding="utf-8")
            (root / "input.txt").write_text("fixed", encoding="utf-8")
            c = Client(binary, root)
            c.request("init", dict(projectId="recovery-fixture", deviceId="native-device",
                                    copyId="native-copy", owner="main"))
            payload = binding(root)
            payload["command"].update(args=[str(pathlib.Path(__file__).resolve()), "--command", str(root)],
                                       timeoutMs=10000)
            c.request("task.bind", payload)
            c.request("task.ready")
            key = str(uuid.uuid4())
            owned = request_process(c, key)
            state = eventually(lambda: c.request("status") if (root / "started.txt").exists() else None,
                               "actual command never started")
            op = state["operationId"]
            assert state["nextAction"] == "operation.inspect", state
            proof = c.request("operation.inspect", dict(operationId=op))
            assert proof["ownership"]["phase"] == "registered", proof
            assert proof["ownership"]["observation"] == "live", proof
            assert proof["ownership"]["canReconcile"] is False, proof
            reconcile = dict(operationId=op, disposition="abandon", reason="Actual interrupted fixture.")
            c.request("operation.reconcile", reconcile, ok=False)
            c.request("task.revise", {**binding(root, revision=2), "reason": "Cannot replace live work."}, ok=False)
            # Restore original command fingerprint inputs: failed revision must not change binding.
            c.request("outcome.reassess", dict(outcomeId="outcome-A", reason="Live work remains.",
                       changedApproach="Do not overlap.", nextEvidence="Inspection."), ok=False)
            owned.kill()
            owned.wait(timeout=5)
            owned.stdout.close()
            owned.stderr.close()
            owned = None
            def absent():
                data = c.request("operation.inspect", dict(operationId=op))
                return data if data["ownership"]["canReconcile"] else None
            proof = eventually(absent, "registered process tree absence was not confirmed")
            assert proof["ownership"]["observation"] == "absent", proof
            c.request("operation.reconcile", reconcile, revision=max(0, c.revision - 1), ok=False)
            # A raw copied local database cannot authorize recovery on another selected root.
            copied = base / "raw copied local store"
            shutil.copytree(root, copied)
            other = Client(binary, copied)
            other.request("status")
            copied_proof = other.request("operation.inspect", dict(operationId=op))
            assert copied_proof["ownership"]["canReconcile"] is False, copied_proof
            assert copied_proof["ownership"]["observation"] == "unknown", copied_proof
            other.request("operation.reconcile", reconcile, ok=False)
            c.request("operation.reconcile", reconcile, actor="worker-1", ok=False)
            c.request("stop", dict(enabled=True))
            decision_key = str(uuid.uuid4())
            receipt = c.request("operation.reconcile", reconcile, key=decision_key)
            assert receipt["operationId"] == op and receipt["operationState"] == "incomplete", receipt
            assert receipt.get("exitCode") is None, receipt
            terminal = c.request("operation.inspect", dict(operationId=op))
            assert terminal["ownership"]["canReconcile"] is False, terminal
            original = c.request("run", actor="worker-1", key=key, revision=0)
            assert original["operationId"] == op and original["operationState"] == "incomplete", original
            history = c.request("task.history")
            selected = [x for x in history["operations"] if x["operationId"] == op]
            assert len(selected) == 1 and selected[0]["state"] == "incomplete", selected
            assert history["outcomes"][0]["failureCount"] == 1, history
            c.request("operation.reconcile", reconcile, key=decision_key, revision=0)
            c.request("operation.reconcile", reconcile, ok=False)
            assert c.request("task.history")["outcomes"][0]["failureCount"] == 1
            c.request("submit", actor="worker-1", ok=False)
            c.request("run", actor="worker-1", ok=False)
            c.request("stop", dict(enabled=False))
            assert c.request("status")["nextAction"] == "diagnose_and_revise"
            fresh = binding(root, revision=2)
            fresh["reason"] = "Explicit fresh attempt after confirmed incomplete execution."
            c.request("task.revise", fresh)
            c.request("task.ready")
            c.request("run", actor="worker-1")
            c.request("submit", actor="worker-1")
            c.request("review", dict(accept=True, reason="New evidence, never recovered as success."))
            before = foreign_marker.read_text()
            eventually(lambda: foreign_marker.read_text() != before, "foreign process stopped progressing")
            assert foreign.poll() is None, "recovery interfered with foreign control"
            print("PASS: actual killed CLI, live refusal, native absence, copied-root refusal, owner/STOP, "
                  "incomplete once, replay, fresh explicit revision, foreign process unaffected")
        finally:
            if owned is not None:
                if owned.poll() is None:
                    owned.kill()
                owned.wait(timeout=5)
                owned.stdout.close()
                owned.stderr.close()
            foreign.terminate()
            foreign.wait(timeout=5)


if __name__ == "__main__":
    if len(sys.argv) > 1 and sys.argv[1] == "--command":
        root = pathlib.Path(sys.argv[2])
        (root / "started.txt").write_text("actual child started", encoding="utf-8")
        time.sleep(2.0)
        (root / "finished.txt").write_text("child exited naturally", encoding="utf-8")
        raise SystemExit(0)
    if len(sys.argv) > 1 and sys.argv[1] == "--heartbeat":
        marker = pathlib.Path(sys.argv[2])
        for i in range(600):
            marker.write_text(str(i), encoding="utf-8")
            time.sleep(0.05)
        raise SystemExit(0)
    parser = argparse.ArgumentParser()
    parser.add_argument("--binary", required=True)
    args = parser.parse_args()
    check(str(pathlib.Path(args.binary).resolve(strict=True)))
