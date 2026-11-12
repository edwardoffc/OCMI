"""Independent real-CLI controls for failure episode boundaries and corrupt history."""
import argparse
import contextlib
import hashlib
import pathlib
import sqlite3
import tempfile

from check_lifecycle import binding, revise, setup


def two_failures(c):
    c.request("run", actor="worker-1")
    revise(c, 2, 7)
    c.request("task.ready")
    c.request("run", actor="worker-1")


def reassess(c):
    c.request("outcome.reassess", dict(outcomeId="outcome-A", reason="Two actual failed cycles.",
              changedApproach="One changed diagnostic.", nextEvidence="A fresh command receipt."))


def check(binary):
    with tempfile.TemporaryDirectory(prefix="ocmi-episode-") as temp:
        base = pathlib.Path(temp)
        c = setup(binary, base / "accepted episode", 7)
        two_failures(c)
        reassess(c)
        revise(c, 3, 0)
        c.request("task.ready")
        c.request("run", actor="worker-1")
        c.request("submit", actor="worker-1")
        c.request("review", dict(accept=True, reason="Episode genuinely accepted."))
        c.request("integrate", dict(candidate="accepted-A", reason="Verified fictional result."))
        c.request("resource.close", dict(disposition="keep", reason="History retained."))
        c.request("task.bind", binding(c.root, task="B", outcome="outcome-A", code=7))
        c.request("task.ready")
        c.request("run", actor="worker-1")
        p = binding(c.root, task="B", outcome="outcome-A", revision=2, code=7)
        p["reason"] = "Later same-outcome episode."
        c.request("task.revise", p)
        c.request("task.ready")
        c.request("run", actor="worker-1")
        reassess(c)
        history = c.request("task.history")
        assert len(history["operations"]) == 5, history
        assert history["outcomes"][0]["failureCount"] == 2, history
        print("PASS: acceptance ends admission episode while same-outcome failure history survives")

        c = setup(binary, base / "process stop", 7)
        two_failures(c)
        reassess(c)
        revise(c, 3, 7)
        c.request("task.ready")
        c.request("run", actor="worker-1")
        c.request("outcome.review", dict(outcomeId="outcome-A", decision="stop",
                  findings="Stop this approach.", changedApproach="Preserve WIP.",
                  nextEvidence="New owner decision."))
        c.request("run", actor="worker-1", ok=False)
        history = c.request("task.history")
        assert any(d["decision"] == "stop" for d in history["decisions"]), history
        proof = c.root / "new-diagnostic.txt"
        proof.write_text("Independent fictional diagnostic after stop.", encoding="utf-8")
        c.request("outcome.decide", dict(outcomeId="outcome-A", decision="continue",
                  basis="A new checked diagnostic.", changedApproach="One bounded fresh attempt.",
                  nextEvidence="New command receipt.", evidence=[dict(path=proof.name,
                  sha256=hashlib.sha256(proof.read_bytes()).hexdigest())]))
        assert c.request("task.history")["outcomes"][0]["failureCount"] == 3
        print("PASS: process-review stop persists without grant; accountable project-owner recovery remains possible")

        c = setup(binary, base / "corrupt history")
        with contextlib.closing(sqlite3.connect(c.root / ".ocmi/local/state.sqlite")) as db:
            db.execute("UPDATE task_revision SET task_revision='not-an-integer'")
            db.commit()
        c.request("task.history", ok=False)
        print("PASS: row decode failure cannot become a successful fabricated history")


if __name__ == "__main__":
    parser = argparse.ArgumentParser()
    parser.add_argument("--binary", required=True)
    args = parser.parse_args()
    check(str(pathlib.Path(args.binary).resolve(strict=True)))
