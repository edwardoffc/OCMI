"""Independent black-box checks for the first CLI slice; no runtime implementation imports."""
import argparse
import json
import pathlib
import subprocess
import sys
import tempfile
import uuid


def decode(process):
    value = json.loads(process.stdout)
    assert value.get("schemaVersion") == 1 and type(value.get("ok")) is bool, value
    assert isinstance(value.get("revision"), int) and value["revision"] >= 0, value
    assert isinstance(value.get("code"), str) and value["code"], value
    assert isinstance(value.get("data"), dict), value
    assert process.returncode == (0 if value["ok"] else 2), (process.returncode, value)
    return value


class Client:
    def __init__(self, binary, root):
        self.binary, self.root, self.revision = binary, root, 0

    def request(self, action, payload=None, actor="main", key=None, revision=None, ok=True):
        request = dict(schemaVersion=1, action=action, projectRoot=str(self.root), actor=actor,
                       requestId=key or str(uuid.uuid4()), expectedRevision=self.revision if revision is None else revision,
                       payload=payload or {})
        process = subprocess.run([self.binary, "request"], input=json.dumps(request),
                                 text=True, encoding="utf-8", capture_output=True, timeout=15)
        response = decode(process)
        assert response["ok"] is ok, (action, response, process.stderr)
        self.revision = max(self.revision, response["revision"])
        return response["data"]

    def bind(self, exit_code=0):
        (self.root / "contract.md").write_text("Synthetic task: record one verified result.", encoding="utf-8")
        (self.root / "input.txt").write_text("original", encoding="utf-8")
        self.request("task.bind", dict(taskId="task-1", outcomeId="outcome-1", taskRevision=1, completionScope="target",
                     worker="worker-1", reviewer="main", contract="contract.md", inputs=["input.txt"],
                     command=dict(id="check", executable=sys.executable,
                                  args=[str(pathlib.Path(__file__).resolve()), "--fixture", str(self.root), str(exit_code)],
                                  cwd=".", timeoutMs=5000, outputLimitBytes=8192)))


def scenario(binary, root, exit_code=0):
    root.mkdir()
    c = Client(binary, root)
    c.request("init", dict(projectId="synthetic-project", deviceId="device-1", copyId="copy-1", owner="main"))
    c.bind(exit_code)
    c.request("task.ready")
    return c


def check(binary):
    with tempfile.TemporaryDirectory(prefix="ocmi-cli-") as temp:
        base = pathlib.Path(temp)
        c = scenario(binary, base / "project Ω with spaces")
        c.request("submit", actor="worker-1", ok=False)
        c.request("review", dict(accept=True), actor="worker-1", ok=False)
        c.request("stop", dict(enabled=True))
        c.request("run", actor="worker-1", ok=False)
        assert not (c.root / "effect.txt").exists(), "STOP failed to prevent execution"
        c.request("stop", dict(enabled=False))
        stale = c.revision - 1
        c.request("run", actor="worker-1", revision=stale, ok=False)
        assert not (c.root / "effect.txt").exists(), "stale command executed"
        key = str(uuid.uuid4())
        result = c.request("run", actor="worker-1", key=key)
        assert result["operationId"] and result["exitCode"] == 0
        assert result["operationState"] == "succeeded"
        run_revision = c.revision
        assert c.request("status")["nextAction"] == "submit"
        assert c.revision == run_revision, "read-only status changed state"
        replay = c.request("run", actor="worker-1", key=key, revision=stale)
        assert replay["operationId"] == result["operationId"]
        assert (c.root / "effect.txt").read_text(encoding="utf-8") == "once\n"
        c.request("run", {"different": True}, actor="worker-1", key=key, ok=False)
        c.request("submit", actor="worker-1")
        c.request("review", {"accept": True}, actor="worker-1", ok=False)
        c.request("review", {"accept": True, "reason": "Reviewed synthetic actual command evidence."})
        state = c.request("status")
        assert state["taskState"] == "accepted" and state["cleanup"] == "pending", state
        c.request("integrate", {"candidate": "synthetic-source-v1", "reason": "Verified fixture integration."})
        c.request("resource.close", {"disposition": "keep", "reason": "Preserve synthetic evidence until review."})
        state = c.request("status")
        assert state["taskState"] == "integrated" and state["cleanup"] == "done", state
        assert "logs" not in state, "status unexpectedly dumps logs"
        print("PASS: full cycle, role separation, STOP, stale revision, replay, cleanup, fresh-process status")

        failed = scenario(binary, base / "failed command", exit_code=7)
        receipt = failed.request("run", actor="worker-1")
        assert receipt["exitCode"] == 7 and receipt["operationState"] == "failed", receipt
        failed_revision = failed.revision
        fresh = Client(binary, failed.root)
        failed_state = fresh.request("status")
        assert failed_state["nextAction"] == "diagnose_and_revise", failed_state
        assert fresh.revision == failed_revision and failed_state["failureCount"] == 1
        failed.request("stop", {"enabled": True})
        assert failed.request("status")["nextAction"] == "clear_stop_or_cancel"
        failed.request("stop", {"enabled": False})
        assert failed.request("status")["nextAction"] == "diagnose_and_revise"
        failed.request("submit", actor="worker-1", ok=False)
        print("PASS: real nonzero command remains failed and cannot be submitted")

        changed = scenario(binary, base / "changed inputs")
        changed.request("run", actor="worker-1")
        (changed.root / "input.txt").write_text("changed", encoding="utf-8")
        changed.request("submit", actor="worker-1", ok=False)
        print("PASS: changed required input invalidates successful evidence")

        unknown = Client(binary, base / "missing store")
        unknown.request("status", ok=False)
        assert not unknown.root.exists(), "read-only status created missing project"
        print("PASS: missing state cannot become a successful empty project")


def controls():
    # Validate the transport oracle using actual child exits and malformed/wrong-success controls.
    with tempfile.TemporaryDirectory(prefix="ocmi-oracle-") as temp:
        root = pathlib.Path(temp)
        for code in (0, 7):
            p = subprocess.run([sys.executable, __file__, "--fixture", temp, str(code)], capture_output=True)
            assert p.returncode == code
        assert (root / "effect.txt").read_text() == "once\nonce\n"
    valid = dict(schemaVersion=1, ok=True, revision=0, code="OK", data={})
    decode(subprocess.CompletedProcess([], 0, json.dumps(valid)))
    for output, code in (("not json", 0), (json.dumps(valid), 2), (json.dumps({**valid, "ok": "true"}), 0)):
        try:
            decode(subprocess.CompletedProcess([], code, output))
        except (AssertionError, ValueError):
            continue
        raise AssertionError("oracle accepted malformed or false-success response")
    print("PASS: actual fixture exits/effects and response-oracle positive/negative controls")


if __name__ == "__main__":
    if len(sys.argv) > 1 and sys.argv[1] == "--fixture":
        with open(pathlib.Path(sys.argv[2]) / "effect.txt", "a", encoding="utf-8") as stream:
            stream.write("once\n")
        print("synthetic stdout")
        print("synthetic stderr", file=sys.stderr)
        raise SystemExit(int(sys.argv[3]))
    parser = argparse.ArgumentParser()
    parser.add_argument("--binary")
    parser.add_argument("--controls", action="store_true")
    args = parser.parse_args()
    if args.controls:
        controls()
    elif args.binary:
        check(str(pathlib.Path(args.binary).resolve(strict=True)))
    else:
        parser.error("require --controls or --binary; missing runtime is not a passing integration check")
