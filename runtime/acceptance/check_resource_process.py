"""Real bounded child trees, foreign control, STOP and project-local temporary accounting."""
import argparse
import json
import os
import pathlib
import subprocess
import sys
import tempfile
import time
import uuid

from check_cli import Client, decode
from check_lifecycle import binding


def project(binary, root, code, timeout=5000):
    root.mkdir()
    (root / "contract.md").write_text("Fictional resource/process check.", encoding="utf-8")
    (root / "input.txt").write_text("fixed", encoding="utf-8")
    c = Client(binary, root)
    c.request("init", dict(projectId="process-fixture", deviceId="desktop", copyId="copy", owner="main"))
    p = binding(root)
    p["command"] = dict(id="controlled", executable=sys.executable, args=["-c", code, str(root)],
                        cwd=".", timeoutMs=timeout, outputLimitBytes=8192)
    c.request("task.bind", p); c.request("task.ready")
    return c


def wait_for(test, limit=5):
    end = time.monotonic() + limit
    while time.monotonic() < end:
        if test():
            return
        time.sleep(.025)
    raise AssertionError("bounded observation did not reach its named condition")


def check(binary):
    with tempfile.TemporaryDirectory(prefix="ocmi-resource-process-") as temp:
        base = pathlib.Path(temp)
        foreign_path = base / "foreign heartbeat"
        foreign_code = "import pathlib,sys,time; p=pathlib.Path(sys.argv[1]); end=time.monotonic()+30\nwhile time.monotonic()<end:\n with p.open('a') as f: f.write('.')\n time.sleep(.025)\n"
        foreign = subprocess.Popen([sys.executable, "-c", foreign_code, str(foreign_path)],
                                   stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
        try:
            wait_for(lambda: foreign_path.exists() and foreign_path.stat().st_size > 0)
            code = """import os,pathlib,sys,tempfile
root=pathlib.Path(sys.argv[1]); scratch=pathlib.Path(tempfile.gettempdir())
(root/'scratch-location.txt').write_text(str(scratch),encoding='utf-8')
(scratch/'generated.txt').write_text('fictional temporary output',encoding='utf-8')
"""
            c = project(binary, base / "temporary output", code)
            receipt = c.request("run", actor="worker-1")
            scratch = pathlib.Path((c.root / "scratch-location.txt").read_text(encoding="utf-8"))
            assert scratch.resolve().is_relative_to((c.root / ".ocmi/local/runs").resolve()), scratch
            assert not scratch.exists(), "successful disposable temporary group remains"
            rows = c.request("resource.list", dict(limit=100))["resources"]
            assert any(r["kind"] == "process" and r.get("operationId") == receipt["operationId"]
                       and r["observedState"] == "stopped" for r in rows)
            assert any(r["kind"] == "directory" and r.get("automatic") and r["observedState"] == "absent" for r in rows)
            print("PASS: actual command TMP is owned/project-local, output is accounted and successful scratch is actually removed")

            child = "import pathlib,sys,time; p=pathlib.Path(sys.argv[1]); end=time.monotonic()+15\nwhile time.monotonic()<end:\n with p.open('a') as f: f.write('.')\n time.sleep(.02)\n"
            code = """import os,pathlib,subprocess,sys,time
root=pathlib.Path(sys.argv[1]); pulse=root/'child.txt'
subprocess.Popen([sys.executable,'-c',%r,str(pulse)],stdout=subprocess.DEVNULL,stderr=subprocess.DEVNULL)
end=time.monotonic()+3
while not pulse.exists() and time.monotonic()<end: time.sleep(.01)
if not pulse.exists(): raise RuntimeError('controlled child failed to start')
""" % child
            c = project(binary, base / "ordinary descendant", code)
            before = foreign_path.stat().st_size
            receipt = c.request("run", actor="worker-1")
            assert receipt["operationState"] in ("succeeded", "incomplete"), receipt
            pulse = c.root / "child.txt"; size = pulse.stat().st_size
            time.sleep(.15)
            assert pulse.stat().st_size == size, "owned descendant survived completed command"
            assert foreign.poll() is None and foreign_path.stat().st_size > before, "foreign process was stopped"
            rows = c.request("resource.list", dict(limit=100))["resources"]
            assert any(r["kind"] == "process" and r.get("operationId") == receipt["operationId"]
                       and r["observedState"] == "stopped" for r in rows)
            print("PASS: ordinary descendant with closed output is stopped before completion while foreign heartbeat continues")

            code = """import pathlib,sys,time
p=pathlib.Path(sys.argv[1])/'active.txt'; end=time.monotonic()+15
while time.monotonic()<end:
 with p.open('a') as f: f.write('.')
 time.sleep(.025)
"""
            c = project(binary, base / "real stop", code, timeout=10000)
            request = dict(schemaVersion=1, action="run", projectRoot=str(c.root), actor="worker-1",
                           requestId=str(uuid.uuid4()), expectedRevision=c.revision, payload={})
            run = subprocess.Popen([binary, "request"], stdin=subprocess.PIPE, stdout=subprocess.PIPE,
                                   stderr=subprocess.PIPE, text=True, encoding="utf-8")
            try:
                run.stdin.write(json.dumps(request)); run.stdin.close(); run.stdin = None
                pulse = c.root / "active.txt"
                wait_for(lambda: pulse.exists() and pulse.stat().st_size > 0)
                owner = Client(binary, c.root); owner.request("status")
                before = foreign_path.stat().st_size
                started = time.monotonic(); owner.request("stop", dict(enabled=True))
                stdout, stderr = run.communicate(timeout=9)
                result = decode(subprocess.CompletedProcess([], run.returncode, stdout, stderr))
                assert time.monotonic() - started < 9 and result["ok"], result
                size = pulse.stat().st_size; time.sleep(.15)
                assert pulse.stat().st_size == size and foreign_path.stat().st_size > before
                owner.request("status")
                assert owner.request("status")["stopped"] is True
                rows = owner.request("resource.list", dict(limit=100))["resources"]
                assert any(r["kind"] == "process" and r["observedState"] == "stopped" for r in rows)
                assert not any(r["kind"] == "process" and r["observedState"] == "running" for r in rows)
                print("PASS: actual concurrent STOP cancels owned work within its independent teardown budget, preserving foreign work")
            finally:
                if run.poll() is None:
                    # Only the exact fixture supervisor; its owned-child behavior is the subject
                    # of the existing recovery checks, never a name/global cleanup sweep.
                    run.kill(); run.communicate(timeout=10)
            code = """import pathlib,sys,tempfile
root=pathlib.Path(sys.argv[1]); scratch=pathlib.Path(tempfile.gettempdir())
(root/'scratch-location.txt').write_text(str(scratch),encoding='utf-8')
(scratch/'diagnostic.txt').write_text('fictional failure evidence',encoding='utf-8')
sys.exit(7)
"""
            c = project(binary, base / "retained failure evidence", code)
            receipt = c.request("run", actor="worker-1")
            assert receipt["operationState"] == "failed", receipt
            scratch = pathlib.Path((c.root / "scratch-location.txt").read_text(encoding="utf-8"))
            assert (scratch / "diagnostic.txt").read_text(encoding="utf-8") == "fictional failure evidence"
            rows = c.request("resource.list", dict(limit=100))["resources"]
            group = next(r for r in rows if r["kind"] == "directory" and r["ownership"] == "project"
                         and r.get("operationId") == receipt["operationId"])
            held = c.request("resource.inspect", dict(resourceId=group["resourceId"]))
            assert held["disposition"] == "keep" and held["logical"]["holdUntil"], held
            assert any(r["kind"] == "process" and r.get("operationId") == receipt["operationId"]
                       and r["observedState"] == "stopped" for r in rows)
            c.request("resource.dispose", dict(resourceId=group["resourceId"], disposition="remove",
                reason="Owner finished reviewing the retained fictional diagnostic."))
            assert not scratch.exists(), "retained automatic group has no qualified later cleanup route"
            print("PASS: failed operation retains actual diagnostic files with owner/condition and stopped execution")
            code = """import pathlib,sys,tempfile,time
root=pathlib.Path(sys.argv[1]); scratch=pathlib.Path(tempfile.gettempdir())
(scratch/'held.txt').write_text('fictional held evidence',encoding='utf-8')
(root/'scratch-location.txt').write_text(str(scratch),encoding='utf-8')
time.sleep(2)
"""
            c = project(binary, base / "retention during execution", code)
            request = dict(schemaVersion=1, action="run", projectRoot=str(c.root), actor="worker-1",
                           requestId=str(uuid.uuid4()), expectedRevision=c.revision, payload={})
            run = subprocess.Popen([binary, "request"], stdin=subprocess.PIPE, stdout=subprocess.PIPE,
                                   stderr=subprocess.PIPE, text=True, encoding="utf-8")
            try:
                run.stdin.write(json.dumps(request)); run.stdin.close(); run.stdin = None
                location = c.root / "scratch-location.txt"
                wait_for(lambda: location.exists() and location.stat().st_size > 0)
                owner = Client(binary, c.root); owner.request("status")
                rows = owner.request("resource.list", dict(limit=100))["resources"]
                group = next(r for r in rows if r["kind"] == "directory" and r["ownership"] == "project")
                tool = next(r for r in rows if r["kind"] == "file" and r["ownership"] == "external"
                            and r["resourceId"].startswith("ocmi-tool-"))
                owner.request("resource.release", dict(resourceId=tool["resourceId"],
                    reason="Owner cannot release a live command dependency."), ok=False)
                owner.request("resource.hold", dict(resourceId=group["resourceId"], enabled=True,
                    reason="Owner requested diagnostic retention.", until="owner releases evidence"))
                stdout, stderr = run.communicate(timeout=9)
                receipt = decode(subprocess.CompletedProcess([], run.returncode, stdout, stderr))
                assert receipt["ok"] and receipt["data"]["operationState"] == "succeeded", receipt
                scratch = pathlib.Path(location.read_text(encoding="utf-8"))
                assert (scratch / "held.txt").read_text(encoding="utf-8") == "fictional held evidence"
                held = owner.request("resource.inspect", dict(resourceId=group["resourceId"]))
                assert held["held"] and held["disposition"] == "keep", held
                owner.request("status")
                owner.request("resource.dispose", dict(resourceId=group["resourceId"], disposition="remove", reason="Still held."), ok=False)
                owner.request("resource.hold", dict(resourceId=group["resourceId"], enabled=False,
                    reason="Owner review is complete.", until="now"))
                owner.request("resource.dispose", dict(resourceId=group["resourceId"], disposition="remove",
                    reason="Owner explicitly finishes retained fixture lifecycle."))
                assert not scratch.exists()
                print("PASS: retention requested during execution preserves evidence after verified process stop")
            finally:
                if run.poll() is None:
                    run.kill(); run.communicate(timeout=10)
        finally:
            foreign.terminate()
            foreign.wait(timeout=10)


if __name__ == "__main__":
    parser = argparse.ArgumentParser(); parser.add_argument("--binary", required=True)
    args = parser.parse_args(); check(str(pathlib.Path(args.binary).resolve(strict=True)))
