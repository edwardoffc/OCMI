"""A historical clock order must not substitute another task's successful evidence."""
import argparse
import contextlib
import pathlib
import sqlite3
import tempfile

from check_lifecycle import binding, setup


def check(binary):
    with tempfile.TemporaryDirectory(prefix="ocmi-scope-") as temp:
        c = setup(binary, pathlib.Path(temp) / "selected")
        old = c.request("run", actor="worker-1")
        c.request("submit", actor="worker-1")
        c.request("review", dict(accept=True, reason="Accepted actual A evidence."))
        c.request("integrate", dict(candidate="A", reason="Verified fictional result."))
        c.request("resource.close", dict(disposition="keep", reason="Retain A history."))
        c.request("task.bind", binding(c.root, task="B", outcome="outcome-B", code=7))
        c.request("task.ready")
        assert c.request("run", actor="worker-1")["operationState"] == "failed"
        # Own synthetic ledger: emulate a retained/portable timestamp from a clock ahead.
        with contextlib.closing(sqlite3.connect(c.root / ".ocmi/local/state.sqlite")) as db:
            db.execute("UPDATE operation SET created_at='2099-01-01T00:00:00Z' WHERE id=?",
                       (old["operationId"],))
            db.commit()
        c.request("submit", actor="worker-1", ok=False)
        assert c.request("status")["taskId"] == "B"
        print("PASS: failed active B cannot borrow future-dated successful A evidence")


if __name__ == "__main__":
    parser = argparse.ArgumentParser()
    parser.add_argument("--binary", required=True)
    args = parser.parse_args()
    check(str(pathlib.Path(args.binary).resolve(strict=True)))
