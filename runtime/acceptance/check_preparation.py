"""Independent CLI controls: preparation never closes a target failure episode."""
import argparse
import copy
import hashlib
import pathlib
import tempfile

from check_cli import Client
from check_episode import reassess, two_failures
from check_lifecycle import binding, revise, setup


def policy(c):
    return next(o for o in c.request("task.history")["outcomes"] if o["outcomeId"] == "outcome-A")


def package_digest(package):
    value = copy.deepcopy(package)
    value["digest"] = ""
    import json
    encoded = json.dumps(value, ensure_ascii=False, separators=(",", ":"))
    for text, escaped in (("&", "\\u0026"), ("<", "\\u003c"), (">", "\\u003e"),
                          ("\u2028", "\\u2028"), ("\u2029", "\\u2029")):
        encoded = encoded.replace(text, escaped)
    return hashlib.sha256(encoded.encode("utf-8")).hexdigest()


def preparation(c, revision):
    p = binding(c.root, revision=revision, scope="preparation")
    p["reason"] = "Accepted helper is not target completion."
    c.request("task.revise", p)
    c.request("task.ready")
    c.request("run", actor="worker-1")
    c.request("submit", actor="worker-1")


def accept_and_integrate(c):
    before = policy(c)
    reply = c.request("review", dict(accept=True, reason="Preparation independently accepted."), key="prep-review")
    assert c.request("review", dict(accept=True, reason="Preparation independently accepted."),
                     key="prep-review", revision=0) == reply
    assert policy(c) == before, "preparation acceptance changed target policy"
    c.request("integrate", dict(candidate="preparation-only", reason="Helper integrated; target still open."))
    c.request("resource.close", dict(disposition="keep", reason="Retain fictional evidence."))


def check(binary, v2_binary):
    with tempfile.TemporaryDirectory(prefix="ocmi-preparation-") as temp:
        base = pathlib.Path(temp)
        c = setup(binary, base / "gated target", 7)
        assert c.request("status")["completionScope"] == "target"
        two_failures(c)
        reassess(c)
        preparation(c, 3)
        before = policy(c)
        assert before["failureCount"] == 2, before
        accept_and_integrate(c)
        history = c.request("task.history")
        assert history["reviews"][-1]["completionScope"] == "preparation", history
        assert history["revisions"][-1]["completionScope"] == "preparation", history
        c.request("task.bind", binding(c.root, task="B", scope="target"))
        c.request("task.ready")
        c.request("run", actor="worker-1", ok=False)
        assert policy(c) == before, "new task ID reset target failure policy"
        assert (c.root / "effect.txt").read_text() == "once\nonce\nonce\n"
        c.request("outcome.review", dict(outcomeId="outcome-A", decision="continue",
                  findings="Preparation done, target still needs its bounded attempt.",
                  changedApproach="One actual target check.", nextEvidence="Target command receipt."))
        c.request("run", actor="worker-1")
        c.request("submit", actor="worker-1")
        c.request("review", dict(accept=True, reason="Explicit target actually completed."))
        assert policy(c)["failureCount"] == 0
        assert len(c.request("task.history")["operations"]) == 4
        print("PASS: accepted preparation retains retry gate; only explicit target acceptance closes episode")

        c = setup(binary, base / "invalid scopes")
        for value in (None, "unknown", "legacy_unknown", 1):
            p = binding(c.root, revision=2)
            p["reason"] = "Must reject uncertain completion intent."
            p["completionScope"] = value
            revision = c.revision
            c.request("task.revise", p, ok=False)
            assert c.revision == revision
        p.pop("completionScope")
        c.request("task.revise", p, ok=False)
        empty = base / "unbound"; empty.mkdir()
        (empty / "contract.md").write_text("fictional", encoding="utf-8")
        (empty / "input.txt").write_text("fictional", encoding="utf-8")
        d = Client(binary, empty)
        d.request("init", dict(projectId="scope", deviceId="d", copyId="c", owner="main"))
        for value in (None, "unknown", "legacy_unknown"):
            p = binding(empty); p["completionScope"] = value
            d.request("task.bind", p, ok=False)
        p.pop("completionScope"); d.request("task.bind", p, ok=False)
        print("PASS: omitted/null/unknown/reserved completion intent refused before mutation")

        c = setup(binary, base / "failed preparation")
        p = binding(c.root, revision=2, code=7, scope="preparation"); p["reason"] = "Bounded helper."
        c.request("task.revise", p); c.request("task.ready")
        c.request("run", actor="worker-1")
        assert policy(c)["failureCount"] == 1
        print("PASS: preparation failure counts against the same target")

        c = setup(binary, base / "portable scope", 7)
        c.request("run", actor="worker-1")
        preparation(c, 2); accept_and_integrate(c)
        before = c.request("task.history")
        package = c.request("handover.export", dict(transferId="scope-transport", files=["contract.md", "input.txt"]))["package"]
        assert package["schemaVersion"] == 4
        dest = base / "scope restored"; dest.mkdir()
        d = Client(binary, dest)
        d.request("handover.import", dict(package=package, deviceId="laptop", copyId="new-copy"))
        after = d.request("task.history")
        for name in ("revisions", "reviews", "outcomes"):
            assert before[name] == after[name], (name, before[name], after[name])
        assert d.request("status")["stopped"] is True
        malformed = copy.deepcopy(package)
        # Verify the serializer against an actual valid package before using it
        # to distinguish scope validation from checksum rejection.
        assert package_digest(package) == package["digest"]
        malformed["task"]["completionScope"] = "invented"
        malformed["digest"] = package_digest(malformed)
        refused = base / "refused"; refused.mkdir()
        Client(binary, refused).request("handover.import", dict(package=malformed, deviceId="x", copyId="x"), ok=False)
        assert list(refused.iterdir()) == []
        print("PASS: v3 transfer retains scope/history/policy and import quarantine")

        c = setup(v2_binary, base / "actual previous v2", 7)
        two_failures(c); reassess(c)
        revise(c, 3, 0); c.request("task.ready")
        c.request("run", actor="worker-1"); c.request("submit", actor="worker-1")
        before = policy(c)
        store = c.root / ".ocmi/local/state.sqlite"
        old_hash = hashlib.sha256(store.read_bytes()).hexdigest()
        d = Client(binary, c.root)
        d.request("status", ok=False)
        assert hashlib.sha256(store.read_bytes()).hexdigest() == old_hash
        d.request("state.upgrade", actor="worker-1", revision=c.revision, ok=False)
        upgraded = d.request("state.upgrade", key="v2-explicit-upgrade", revision=c.revision)
        assert d.request("state.upgrade", key="v2-explicit-upgrade", revision=0) == upgraded
        assert pathlib.Path(upgraded["backupPath"]).is_file()
        assert d.request("status")["completionScope"] == "legacy_unknown"
        assert policy(d) == before
        d.request("review", dict(accept=True, reason="Legacy work accepted without inferred target completion."))
        assert policy(d) == before
        history = d.request("task.history")
        assert all(r["completionScope"] == "legacy_unknown" for r in history["revisions"])
        assert history["reviews"][-1]["completionScope"] == "legacy_unknown"
        c.request("status", ok=False)
        print("PASS: actual v2 explicit backup/upgrade preserves unknown scope and failure gate; old reader refuses v3")


if __name__ == "__main__":
    parser = argparse.ArgumentParser()
    parser.add_argument("--binary", required=True)
    parser.add_argument("--v2-binary", required=True)
    args = parser.parse_args()
    check(str(pathlib.Path(args.binary).resolve(strict=True)), str(pathlib.Path(args.v2_binary).resolve(strict=True)))
