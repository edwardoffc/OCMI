"""Native recovery identity controls using only an owned synthetic database/process."""
import argparse
import contextlib
import hashlib
import os
import pathlib
import socket
import sqlite3
import subprocess
import sys
import tempfile
import uuid

from check_cli import Client
from check_lifecycle import binding
from check_recovery import eventually, request_process


def native_hash():
    if sys.platform == "win32":
        import winreg
        with winreg.OpenKey(winreg.HKEY_LOCAL_MACHINE, r"SOFTWARE\Microsoft\Cryptography", 0,
                            winreg.KEY_QUERY_VALUE | winreg.KEY_WOW64_64KEY) as key:
            machine = winreg.QueryValueEx(key, "MachineGuid")[0].strip()
        raw = "windows\0" + machine + "\0" + socket.gethostname()
    elif sys.platform == "linux":
        raw = "linux\0" + pathlib.Path("/proc/sys/kernel/random/boot_id").read_text().strip()
        raw += "\0" + os.readlink("/proc/self/ns/pid")
    elif sys.platform == "darwin":
        raw = "darwin\0" + subprocess.check_output(
            ["/usr/sbin/sysctl", "-n", "kern.bootsessionuuid"], text=True, timeout=5).strip()
    else:
        raise AssertionError("unqualified native platform")
    return hashlib.sha256(raw.encode()).hexdigest()


def check(binary):
    with tempfile.TemporaryDirectory(prefix="ocmi-recovery-boundary-") as temp:
        root = pathlib.Path(temp) / "selected"
        root.mkdir()
        (root / "contract.md").write_text("Fictional recovery identity boundary.")
        (root / "input.txt").write_text("fixed")
        c = Client(binary, root)
        c.request("init", dict(projectId="recovery-boundary", deviceId="native-device",
                                copyId="native-copy", owner="main"))
        p = binding(root)
        p["command"].update(args=[str(pathlib.Path(__file__).with_name("check_recovery.py").resolve()),
                                   "--command", str(root)], timeoutMs=10000)
        c.request("task.bind", p)
        c.request("task.ready")
        command = request_process(c, str(uuid.uuid4()))
        try:
            state = eventually(lambda: c.request("status") if (root / "started.txt").exists() else None,
                               "actual child did not start")
            op = state["operationId"]
            command.kill()
            command.wait(timeout=5)
            command.stdout.close()
            command.stderr.close()
            def absent():
                value = c.request("operation.inspect", dict(operationId=op))
                return value if value["ownership"]["canReconcile"] else None
            eventually(absent, "owned registered tree did not become absent")
            database = root / ".ocmi/local/state.sqlite"
            def sql(statement, args=()):
                with contextlib.closing(sqlite3.connect(database)) as db:
                    cursor = db.execute(statement, args)
                    rows = cursor.fetchall()
                    db.commit()
                    return rows
            recorded = sql("SELECT context_hash,parent_pid FROM operation_owner WHERE operation_id=?", (op,))[0]
            # Same qualified native-only SHA256 representation as the independently tested probe.
            assert recorded[0] == native_hash(), "stored context is not the qualified native context hash"
            reconcile = dict(operationId=op, disposition="abandon", reason="Owned boundary fixture.")
            def blocked():
                result = c.request("operation.inspect", dict(operationId=op))
                assert result["ownership"]["canReconcile"] is False, result
                c.request("operation.reconcile", reconcile, ok=False)
            sql("UPDATE operation_owner SET parent_pid=? WHERE operation_id=?", (os.getpid(), op))
            blocked()  # Actual live foreign/reused supervisor identity, not a fabricated PID.
            sql("UPDATE operation_owner SET parent_pid=? WHERE operation_id=?", (recorded[1], op))
            sql("UPDATE project SET copy_id='different-copy'")
            blocked()
            sql("UPDATE project SET copy_id='native-copy'")
            sql("UPDATE project SET device_id='different-device'")
            blocked()
            sql("UPDATE project SET device_id='native-device'")
            sql("UPDATE operation_owner SET context_hash='unqualified-context' WHERE operation_id=?", (op,))
            blocked()
            sql("UPDATE operation_owner SET context_hash=?,phase='launching' WHERE operation_id=?", (recorded[0], op))
            blocked()
            sql("UPDATE meta SET schema_version=999")
            c.request("operation.inspect", dict(operationId=op), ok=False)
            assert sql("SELECT schema_version FROM meta")[0][0] == 999
            sql("UPDATE meta SET schema_version=4")
            sql("DELETE FROM operation_owner WHERE operation_id=?", (op,))
            result = c.request("operation.inspect", dict(operationId=op))
            assert result["ownership"]["phase"] == "unregistered", result
            assert result["ownership"]["canReconcile"] is False, result
            c.request("operation.reconcile", reconcile, ok=False)
            print("PASS: qualified native hash, live supervisor, changed copy/context, launch gap and unregistered refusal")
        finally:
            if command.poll() is None:
                command.kill()
            command.wait(timeout=5)
            if not command.stdout.closed:
                command.stdout.close()
            if not command.stderr.closed:
                command.stderr.close()


if __name__ == "__main__":
    parser = argparse.ArgumentParser()
    parser.add_argument("--binary", required=True)
    args = parser.parse_args()
    check(str(pathlib.Path(args.binary).resolve(strict=True)))
