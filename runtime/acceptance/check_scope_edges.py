"""Independent continuation edges introduced by preparation and v2 compatibility."""
import argparse
import copy
import hashlib
import pathlib
import tempfile

from check_cli import Client
from check_episode import reassess, two_failures
from check_lifecycle import binding, setup
from check_preparation import accept_and_integrate, package_digest, preparation


def check(binary, v2_binary):
    with tempfile.TemporaryDirectory(prefix="ocmi-scope-edges-") as temp:
        base = pathlib.Path(temp)
        c = setup(binary, base / "next guidance", 7)
        two_failures(c); reassess(c); preparation(c, 3); accept_and_integrate(c)
        c.request("task.bind", binding(c.root, task="B")); c.request("task.ready")
        state = c.request("status")
        assert state["nextAction"] == "outcome.review", state
        c.request(state["nextAction"], dict(outcomeId="outcome-A", decision="continue",
                  findings="Bounded preparation is complete.", changedApproach="Now check target.",
                  nextEvidence="Target receipt."))
        print("PASS: post-preparation continuation suggests the actual available policy decision")

        c = setup(v2_binary, base / "legacy inspect", 7)
        run = c.request("run", actor="worker-1")
        store = c.root / ".ocmi/local/state.sqlite"
        before = hashlib.sha256(store.read_bytes()).hexdigest()
        # Inspect must route legacy stores to explicit upgrade, as status/history do.
        import json
        import subprocess
        request = dict(schemaVersion=1, action="operation.inspect", projectRoot=str(c.root),
                       actor="main", requestId="legacy-inspect", expectedRevision=c.revision,
                       payload=dict(operationId=run["operationId"]))
        process = subprocess.run([binary, "request"], input=json.dumps(request), encoding="utf-8",
                                 capture_output=True, timeout=15)
        response = json.loads(process.stdout)
        assert process.returncode == 2 and response["code"] == "UPGRADE_REQUIRED", response
        assert response["data"]["nextAction"] == "state.upgrade"
        for action in ("status", "task.history"):
            request["action"] = action; request["requestId"] = "legacy-" + action; request["payload"] = {}
            process = subprocess.run([binary, "request"], input=json.dumps(request), encoding="utf-8",
                                     capture_output=True, timeout=15)
            response = json.loads(process.stdout)
            assert process.returncode == 2 and response["code"] == "UPGRADE_REQUIRED", (action, response)
            assert response["data"]["nextAction"] == "state.upgrade"
        assert hashlib.sha256(store.read_bytes()).hexdigest() == before
        print("PASS: v2 inspection requires explicit upgrade without writing legacy state")

        c = setup(v2_binary, base / "legacy scope injection")
        package = c.request("handover.export", dict(transferId="old-format", files=["contract.md", "input.txt"]))["package"]
        assert package_digest(package) == package["digest"]
        forged = copy.deepcopy(package)
        forged["task"]["completionScope"] = "target"
        for row in forged["tasks"] + forged["revisions"]:
            row["completionScope"] = "target"
        forged["digest"] = package_digest(forged)
        target = base / "injected"; target.mkdir()
        Client(binary, target).request("handover.import", dict(package=forged, deviceId="x", copyId="x"), ok=False)
        assert list(target.iterdir()) == []
        print("PASS: legacy package cannot inject v3 target-completion meaning")


if __name__ == "__main__":
    parser = argparse.ArgumentParser()
    parser.add_argument("--binary", required=True)
    parser.add_argument("--v2-binary", required=True)
    args = parser.parse_args()
    check(str(pathlib.Path(args.binary).resolve(strict=True)), str(pathlib.Path(args.v2_binary).resolve(strict=True)))
