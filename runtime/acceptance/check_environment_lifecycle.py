"""Independent actual-probe ownership and source-linked recovery consumer."""
import argparse
import json
import os
import pathlib
import shutil
import subprocess
import sys
import tempfile
import uuid

from check_environment import invoke, snapshot


def walk(value):
    if isinstance(value, dict):
        yield value
        for child in value.values():
            yield from walk(child)
    elif isinstance(value, list):
        for child in value:
            yield from walk(child)


def check(binary):
    base = pathlib.Path(tempfile.mkdtemp(prefix="ocmi-environment-lifecycle-"))
    project, scratch = base / "project", base / "owned scratch"
    project.mkdir(); scratch.mkdir(); (project / "docs").mkdir()
    (project / "go.mod").write_text("module fixture.invalid/recovery\ngo 1.20.0\n", encoding="utf-8")
    (project / "docs/setup.md").write_text("# Setup\nRestore dependencies.\nRestore data separately.\nConfirm current task.\n", encoding="utf-8")
    payload = dict(sources=[dict(kind="go-mod", path="go.mod"), dict(kind="guide", path="docs/setup.md")],
                   tools=[dict(kind="go", executable=str(pathlib.Path(shutil.which("go")).absolute()))],
                   scratchRoot=str(scratch), provisions=[dict(kind="data", source="docs/setup.md", line=3)],
                   continuation=dict(source="docs/setup.md", line=4))
    before = snapshot(project)
    report = invoke(binary, project, payload)
    assert snapshot(project) == before, "environment operation changed selected source"
    records = []
    for p in scratch.rglob("*"):
        if p.is_file() and p.suffix == ".json":
            assert p.stat().st_size <= 65536, "unbounded diagnostic journal"
            records.extend(walk(json.loads(p.read_text(encoding="utf-8"))))
    states = {r.get("state", r.get("phase")) for r in records}
    assert {"beforelaunch", "registered", "stopped"} <= states, ("actual fixed probe lacks retained owned lifecycle", states)
    assert any(r.get("owner") and r.get("purpose") and (r.get("retention") or r.get("retainUntil")) for r in records), "journal lacks lifecycle owner/purpose/retention"
    registered = [r for r in records if r.get("state", r.get("phase")) == "registered"]
    assert registered and all(isinstance(r.get("pid"), int) and r["pid"] > 0 for r in registered), registered
    for step in report["recoverySteps"]:
        assert isinstance(step, dict) and step.get("source") in ("go.mod", "docs/setup.md"), step
    serialized = json.dumps(report)
    assert "docs/setup.md" in serialized and '"line": 4' in serialized, "selected continuation reference lost"
    # Invalid references must not cause any probe or artifact creation first.
    old_scratch = snapshot(scratch)
    bad = dict(payload, provisions=[dict(kind="secret", source="docs/setup.md", line=99)])
    invoke(binary, project, bad, ok=False)
    assert snapshot(scratch) == old_scratch, "invalid input had resource effects"
    (project / "broken.json").write_text('{"engines":', encoding="utf-8")
    bad = dict(payload, sources=payload["sources"] + [dict(kind="node-package", path="broken.json")])
    invoke(binary, project, bad, ok=False)
    assert snapshot(scratch) == old_scratch, "late malformed manifest caused a probe or artifact first"
    if os.name != "nt":
        fifo = base / "synthetic-tool.fifo"
        os.mkfifo(fifo, 0o600)
        request = dict(schemaVersion=1, action="environment.inspect", projectRoot=str(project), actor="main",
                       requestId=str(uuid.uuid4()), expectedRevision=0,
                       payload=dict(payload, tools=[dict(kind="go", executable=str(fifo))]))
        result = subprocess.run([binary, "request"], input=json.dumps(request), text=True,
                                encoding="utf-8", capture_output=True, timeout=3)
        assert result.returncode == 2 and json.loads(result.stdout)["ok"] is False, "special executable was opened/executed"
        assert snapshot(scratch) == old_scratch, "special binding had resource effects"
        print("ENVIRONMENT_NATIVE_FIFO_REFUSAL_OK")
    (project / "sub").mkdir()
    (project / "sub/go.mod").write_text("module fixture.invalid/sub\ngo 1.21\n", encoding="utf-8")
    old_json = {p for p in scratch.rglob("*.json")}
    multi = dict(payload, sources=payload["sources"] + [dict(kind="go-mod", path="sub/go.mod")])
    report = invoke(binary, project, multi)
    assert all(r["status"] == "match" for r in report["requirements"] if r["kind"] == "go"), report
    events = [r for p in scratch.rglob("*.json") if p not in old_json
              for r in walk(json.loads(p.read_text(encoding="utf-8")))
              if r.get("state", r.get("phase")) == "registered"]
    assert len(events) == 1, ("same tool probed more than once", events)
    (project / "pyproject.toml").write_text('[project]\nname="synthetic"\nrequires-python=">=3.10,<4"\n', encoding="utf-8")
    py = dict(payload, sources=[dict(kind="python-project", path="pyproject.toml"), payload["sources"][1]],
              tools=[dict(kind="python", executable=str(pathlib.Path(sys.executable).resolve()))])
    report = invoke(binary, project, py)
    assert any(r["kind"] == "python" and r["status"] == "match" for r in report["requirements"]), report
    node = shutil.which("node")
    if node:
        (project / "package.json").write_text('{"name":"synthetic","engines":{"node":">=1 <999"}}', encoding="utf-8")
        js = dict(payload, sources=[dict(kind="node-package", path="package.json"), payload["sources"][1]],
                  tools=[dict(kind="node", executable=str(pathlib.Path(node).resolve()))])
        report = invoke(binary, project, js)
        assert any(r["kind"] == "node" and r["status"] == "match" for r in report["requirements"]), report
        print("ENVIRONMENT_ACTUAL_NODE_OK")
    else:
        print("ENVIRONMENT_ACTUAL_NODE_NOT_AVAILABLE; parser coverage only on this host")
    print("ENVIRONMENT_ACTUAL_PYTHON_OK")
    print("ENVIRONMENT_LIFECYCLE_OK actual registered/stopped probes, once-per-kind, source steps, no-effect invalid input")
    print("ENVIRONMENT_LIFECYCLE_FIXTURE_RETAINED=" + str(base))


if __name__ == "__main__":
    parser = argparse.ArgumentParser()
    parser.add_argument("--binary", required=True)
    args = parser.parse_args()
    check(str(pathlib.Path(args.binary).resolve()))
