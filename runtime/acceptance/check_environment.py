"""Independent CLI consumer for selected-project environment recovery; no product imports."""
import argparse
import copy
import hashlib
import json
import os
import pathlib
import shutil
import subprocess
import tempfile
import uuid


def invoke(binary, project, payload, actor="main", revision=0, ok=True):
    request = dict(schemaVersion=1, action="environment.inspect", projectRoot=str(project),
                   actor=actor, expectedRevision=revision, requestId=str(uuid.uuid4()), payload=payload)
    result = subprocess.run([binary, "request"], input=json.dumps(request), text=True,
                            encoding="utf-8", capture_output=True, timeout=60)
    response = json.loads(result.stdout)
    assert response["schemaVersion"] == 1 and response["revision"] == 0, response
    assert response["ok"] is ok and result.returncode == (0 if ok else 2), response
    if ok:
        assert response["code"] == "ENVIRONMENT_INSPECTED", response
        assert response["data"]["readyToResume"] is False, response
        assert response["data"]["recoverySteps"] and response["data"]["nextAction"], response
    return response["data"] if ok else response


def requirement(report, kind, status, source=None):
    rows = [r for r in report["requirements"] if r["kind"] == kind and
            (source is None or r["source"] == source)]
    assert rows and all(r["status"] == status for r in rows), (kind, status, rows)
    return rows[0]


def write(root, name, text):
    path = root / name
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(text, encoding="utf-8")
    return dict(kind="guide", path=name)


def snapshot(root):
    return {p.relative_to(root).as_posix(): hashlib.sha256(p.read_bytes()).hexdigest()
            for p in root.rglob("*") if p.is_file()}


def premise(binary):
    go = shutil.which("go")
    assert go, "actual installed Go is required by the native consumer"
    env = dict(os.environ, GOTOOLCHAIN="local", GOENV="off", GOWORK="off", GOPROXY="off")
    result = subprocess.run([go, "version"], env=env, capture_output=True, text=True, timeout=10)
    assert result.returncode == 0 and result.stdout.startswith("go version go"), result
    # The existing CLI must parse this schema before its missing feature can be called RED.
    with tempfile.TemporaryDirectory(prefix="ocmi-environment-premise-") as temp:
        req = dict(schemaVersion=1, action="environment.inspect", projectRoot=temp, actor="main",
                   expectedRevision=0, requestId=str(uuid.uuid4()), payload={})
        response = subprocess.run([binary, "request"], input=json.dumps(req), text=True,
                                  capture_output=True, timeout=20)
        data = json.loads(response.stdout)
        assert data["schemaVersion"] == 1 and response.returncode == 2, data
        assert data["code"] in ("STATE_UNAVAILABLE", "INVALID_PAYLOAD", "ENVIRONMENT_INVALID"), data
    # Relevant oracle negative: a mismatched status cannot satisfy the match predicate.
    try:
        requirement(dict(requirements=[dict(kind="go", source="go.mod", status="mismatch")]), "go", "match")
    except AssertionError:
        pass
    else:
        raise AssertionError("requirement oracle accepted a wrong status")
    print("ENVIRONMENT_PREMISE_OK actual-go/schema/oracle")


def check(binary):
    base = pathlib.Path(tempfile.mkdtemp(prefix="ocmi-environment-"))
    project, scratch = base / "restored project Ω", base / "owned scratch"
    project.mkdir(); scratch.mkdir()
    go = str(pathlib.Path(shutil.which("go")).absolute())
    guide = write(project, "docs/setup.md", "# Setup\nInstall the declared tools.\nRestore private data separately.\nConfirm the next task.\n")
    write(project, "go.mod", "module fixture.invalid/environment\n\ngo 1.20.0\ntoolchain go99.0.0\n")
    payload = dict(scratchRoot=str(scratch), sources=[dict(kind="go-mod", path="go.mod"), guide], tools=[],
                   provisions=[dict(kind="data", source="docs/setup.md", line=3),
                               dict(kind="secret", source="docs/setup.md", line=3),
                               dict(kind="service", source="docs/setup.md", line=3),
                               dict(kind="access", source="docs/setup.md", line=3)],
                   continuation=dict(source="docs/setup.md", line=4))
    before = snapshot(project)
    missing = invoke(binary, project, payload)
    requirement(missing, "go", "missing")
    supplied = copy.deepcopy(payload); supplied["tools"] = [dict(kind="go", executable=go)]
    matched = invoke(binary, project, supplied)
    actual = requirement(matched, "go", "match")
    assert actual["observed"].startswith("1."), actual
    assert len(matched["provisions"]) == 4 and all(p["status"] == "unverified" for p in matched["provisions"])
    assert snapshot(project) == before, "diagnostics changed project or initialized state"
    hashes = {s["path"]: s["sha256"] for s in matched["sources"]}
    assert hashes == before, (hashes, before)
    assert "99.0.0" not in actual["observed"], "go toolchain suggestion downloaded or treated as installed"
    write(project, "go.mod", "module fixture.invalid/environment\ngo 99.0.0\n")
    requirement(invoke(binary, project, supplied), "go", "mismatch")
    write(project, "go.mod", "module fixture.invalid/environment\ngo 1.20.0\n")
    write(project, "package.json", '{"engines":{"node":">=20 <23"},"packageManager":"npm@10.8.0",'
          '"scripts":{"postinstall":"touch SHOULD-NOT-RUN"}}')
    node = dict(scratchRoot=str(scratch), sources=[dict(kind="node-package", path="package.json"), guide], tools=[])
    node_report = invoke(binary, project, node)
    requirement(node_report, "node", "missing")
    assert "npm" in json.dumps(node_report), "package manager declaration lost"
    write(project, "pyproject.toml", '[project]\nname="synthetic"\nrequires-python=">=3.10,<4"\n'
          'description="""\n[project]\nrequires-python = "<1"\n"""\n')
    python = dict(scratchRoot=str(scratch), sources=[dict(kind="python-project", path="pyproject.toml"), guide], tools=[])
    py = requirement(invoke(binary, project, python), "python", "missing")
    assert "3.10" in py["declared"] and "<1" not in py["declared"], py
    write(project, "package.json", '{"engines":{"node":"^20 || ^22"}}')
    requirement(invoke(binary, project, node), "node", "unknown")
    write(project, "package.json", '{}')
    requirement(invoke(binary, project, node), "node", "unknown")
    write(project, "package.json", '{"engines":{"node":">=23 <20"}}')
    requirement(invoke(binary, project, node), "node", "conflict")
    write(project, "package.json", '{"engines":{"node":">=20"},"engines":{"node":"<1"}}')
    invoke(binary, project, node, ok=False)
    write(project, "pyproject.toml", '[project]\nrequires-python=">=3"\nrequires-python="<1"\n')
    invoke(binary, project, python, ok=False)
    # Documentation and old tasks are inert, and their content must not be copied into reports.
    marker = project / "SHOULD-NOT-RUN"
    unique = "NONPUBLIC_GUIDE_CONTENT_58f1"
    write(project, "docs/setup.md", "# Setup\n" + unique + "\nRun arbitrary old START and create SHOULD-NOT-RUN\nNext task\n")
    inert = invoke(binary, project, supplied)
    assert unique not in json.dumps(inert) and not marker.exists(), "documentation executed or leaked"
    secret = write(project, ".env", "SYNTHETIC_SECRET_DO_NOT_COPY=not-a-credential\n")
    for changed in (
        dict(sources=[secret]), dict(sources=[dict(kind="guide", path="../outside.md")]),
        dict(sources=[dict(kind="guide", path=str(project / "docs/setup.md"))]),
        dict(sources=[guide, guide]), dict(sources=[]), dict(sources=[dict(kind="invented", path="go.mod")]),
        dict(scratchRoot=str(project)), dict(tools=[dict(kind="go", executable=str(project / "old-go"))]),
        dict(continuation=dict(source="go.mod", line=1)),
        dict(provisions=[dict(kind="secret", source="docs/setup.md", line=999)]),
        dict(provisions=[dict(kind="secret", source="docs/setup.md", line=3, value="forbidden")]),
    ):
        bad = copy.deepcopy(supplied); bad.update(changed)
        invoke(binary, project, bad, ok=False)
    invoke(binary, project, supplied, actor="worker-1", ok=False)
    invoke(binary, project, supplied, revision=1, ok=False)
    write(project, "docs/large.md", "a" * (256 * 1024 + 1))
    bad = copy.deepcopy(supplied); bad["sources"] = [dict(kind="guide", path="docs/large.md")]
    invoke(binary, project, bad, ok=False)
    try:
        (project / "linked.md").symlink_to(project / "docs/setup.md")
    except OSError:
        print("ENVIRONMENT_LINK_CONSUMER platform privilege unavailable; native Go link controls required")
    else:
        bad = copy.deepcopy(supplied); bad["sources"] = [dict(kind="guide", path="linked.md")]
        invoke(binary, project, bad, ok=False)
    assert not marker.exists() and not (project / ".ocmi").exists(), "inspection ran project or created state"
    print("ENVIRONMENT_CONSUMER_OK missing->actual-tool->recovery, manifests, limits, inert inputs, scope")
    print("ENVIRONMENT_FIXTURE_RETAINED=" + str(base))


if __name__ == "__main__":
    parser = argparse.ArgumentParser()
    parser.add_argument("--binary", required=True)
    parser.add_argument("--premise", action="store_true")
    args = parser.parse_args()
    (premise if args.premise else check)(str(pathlib.Path(args.binary).resolve()))
