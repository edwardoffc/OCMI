"""Independent Git/WIP checkpoint consumer; fictional owned fixtures only."""
import argparse
import hashlib
import json
import os
import pathlib
import shlex
import shutil
import subprocess
import sys
import tempfile
import uuid

from check_cli import decode
from check_checkpoint import directory_link, remove_directory_link


def sha(data):
    return hashlib.sha256(data).hexdigest()


def oid(data):
    return hashlib.sha1(b"blob " + str(len(data)).encode() + b"\0" + data).hexdigest()


def git(root, *args, extra=(), data=None):
    env = {k: v for k, v in os.environ.items() if not k.upper().startswith("GIT_")}
    env.update(GIT_CONFIG_NOSYSTEM="1", GIT_CONFIG_GLOBAL=os.devnull, GIT_ATTR_NOSYSTEM="1",
               GIT_TERMINAL_PROMPT="0", GIT_OPTIONAL_LOCKS="0", GIT_NO_LAZY_FETCH="1",
               GIT_NO_REPLACE_OBJECTS="1")
    command = ["git", "--no-pager", "-c", "core.hooksPath=", "-c", "core.fsmonitor=false",
               "-c", "maintenance.auto=false", "-c", "gc.auto=0", *extra, *args]
    p = subprocess.run(command, cwd=root, env=env, input=data, capture_output=True, timeout=30)
    assert p.returncode == 0, (args, p.returncode, p.stderr)
    return p.stdout


def files(root):
    return {p.relative_to(root).as_posix(): p.read_bytes() for p in root.rglob("*")
            if p.is_file() and ".git" not in p.relative_to(root).parts}


def state(root, refs):
    return dict(head=git(root, "rev-parse", "HEAD"), attached=git(root, "symbolic-ref", "HEAD"),
                refs={r: git(root, "rev-parse", r) for r in refs},
                index=git(root, "ls-files", "--stage", "-z"), files=files(root))


def invoke(binary, root, action, payload, ok=True, actor="main", revision=0):
    request = dict(schemaVersion=1, action=action, projectRoot=str(root), actor=actor,
                   requestId=str(uuid.uuid4()), expectedRevision=revision, payload=payload)
    env = dict(os.environ)
    foreign_index = root.parent / "foreign-env-index"
    env.update(GIT_DIR=str(root.parent / "foreign-git-dir"), GIT_WORK_TREE=str(root.parent / "foreign-tree"),
               GIT_INDEX_FILE=str(foreign_index), GIT_CONFIG_COUNT="1", GIT_CONFIG_KEY_0="core.fsmonitor",
               GIT_CONFIG_VALUE_0="forbidden-source-command")
    p = subprocess.run([binary, "request"], input=json.dumps(request), text=True, encoding="utf-8",
                       capture_output=True, timeout=210, env=env)
    assert not foreign_index.exists(), "inherited Git index escaped selected ownership"
    response = decode(p)
    assert response["ok"] is ok, (action, response, p.stderr)
    assert (p.returncode == 0) is ok, (action, p.returncode, response)
    return response


def fixture(base):
    source = base / "source project Ω"; source.mkdir()
    template = base / "empty-template"; template.mkdir()
    git(source, "init", "--object-format=sha1", "--initial-branch=main", "--template=" + str(template))
    git(source, "config", "user.name", "Synthetic fixture")
    git(source, "config", "user.email", "fixture@example.com")
    initial = {".gitignore": b"ignored.txt\n", ".gitattributes": b"*.filtered filter=ocmi\n",
               "change.filtered": b"original\n", "delete.txt": b"remove from worktree\n",
               "stage-delete.txt": b"remove from index\n", "empty.txt": b"",
               "nested/code Ω.txt": "original Ω\n".encode(), "binary.bin": b"\0\xff\x10"}
    for name, content in initial.items():
        p = source / name; p.parent.mkdir(parents=True, exist_ok=True); p.write_bytes(content)
    git(source, "add", "."); git(source, "commit", "--no-gpg-sign", "-m", "Fixture foundation")
    git(source, "branch", "kept")
    (source / "empty.txt").write_bytes(b"second commit\n")
    git(source, "add", "empty.txt"); git(source, "commit", "--no-gpg-sign", "-m", "Fixture continuation")
    marker = base / "filter-marker"
    script = base / "identity-filter.py"
    script.write_text("import pathlib,sys\npathlib.Path(sys.argv[1]).write_text('invoked')\n"
                      "sys.stdout.buffer.write(sys.stdin.buffer.read())\n", encoding="utf-8")
    command = " ".join(shlex.quote(p) for p in [sys.executable.replace("\\", "/"), script.as_posix(), marker.as_posix()])
    git(source, "config", "filter.ocmi.clean", command)
    git(source, "config", "filter.ocmi.smudge", command)
    git(source, "hash-object", "--path=change.filtered", "--stdin", data=b"control\n")
    assert marker.read_text() == "invoked", "filter control did not execute"
    marker.unlink()
    hooks = source / ".git" / "hooks"; hooks.mkdir(exist_ok=True)
    hook_marker = base / "hook-marker"
    hook = hooks / "pre-commit"
    hook.write_text("#!/bin/sh\nprintf invoked > " + shlex.quote(hook_marker.as_posix()) + "\n", encoding="utf-8")
    hook.chmod(0o700)
    git(source, "commit", "--allow-empty", "--no-gpg-sign", "-m", "Hook control",
        extra=("-c", "core.hooksPath=" + str(hooks)))
    assert hook_marker.read_text() == "invoked", "hook control did not execute"
    hook_marker.unlink()
    git(source, "config", "remote.origin.url", "https://fixture.invalid/private-repository")
    (source / "change.filtered").write_bytes(b"STAGED raw\r\n")
    (source / "new.txt").write_bytes(b"staged addition\n")
    git(source, "add", "change.filtered", "new.txt")
    assert marker.exists(), "applicable staged-path filter control missing"
    marker.unlink()
    (source / "change.filtered").write_bytes(b"UNSTAGED raw\x00\xff\r\n")
    (source / "new.txt").write_bytes(b"unstaged addition\n")
    (source / "delete.txt").unlink()
    git(source, "rm", "stage-delete.txt")
    (source / "empty.txt").write_bytes(b"")
    (source / "nested/code Ω.txt").write_bytes("changed Ω\r\n".encode())
    (source / "untracked.txt").write_bytes(b"selected untracked\n")
    (source / "ignored.txt").write_bytes(b"selected ignored\n")
    (source / "unselected-private.txt").write_bytes(b"not selected for transport\n")
    refs = ["refs/heads/main", "refs/heads/kept"]
    expected = state(source, refs)
    assert git(source, "cat-file", "blob", ":change.filtered") == b"STAGED raw\r\n"
    assert expected["files"]["change.filtered"] != b"STAGED raw\r\n"
    wrong = dict(expected, index=expected["index"].replace(oid(b"STAGED raw\r\n").encode(),
                                                        oid(expected["files"]["change.filtered"]).encode()))
    assert wrong != expected, "index-collapse oracle accepted changed staged bytes"
    assert set(initial) != set(expected["files"]), "HEAD-only oracle lacks observable WIP"
    return source, refs, expected, marker, hook_marker


def seal(package, manifest, raw=None):
    raw = raw if raw is not None else json.dumps(manifest, ensure_ascii=False, separators=(",", ":")).encode()
    (package / "manifest.json").write_bytes(raw)
    (package / "COMPLETE").write_text(json.dumps(dict(schemaVersion=2, manifestSha256=sha(raw))), encoding="utf-8")


def check(binary, fixture_only=False):
    base = pathlib.Path(tempfile.mkdtemp(prefix="ocmi-git-checkpoint-"))
    (base / "OWNERSHIP.json").write_text(json.dumps(dict(owner="OCMI independent acceptance",
        purpose="Fictional Git/WIP recovery consumer", retainUntil="Main component acceptance",
        disposition="Retain failed/successful fixture evidence; hosted runner lifetime ends externally")), encoding="utf-8")
    source, refs, expected_source, filter_marker, hook_marker = fixture(base)
    expected = dict(expected_source, files={k: v for k, v in expected_source["files"].items()
                                          if k != "unselected-private.txt"})
    print("PASS: actual Git fixture, applicable filter/hook controls and discriminating index/WIP oracle")
    print("Fixture retained:", base)
    if fixture_only:
        return
    package = base / "private recovery"
    payload = dict(packagePath=str(package), checkpointId="fictional-git", systemVersion="1.3.5",
        sourceIdentity="synthetic-git-source", owner="synthetic-orchestrator", purpose="Coherent Git/WIP recovery",
        retainUntil="Verified fresh-root recovery", nextAction="Confirm binding and missing data before continuing.",
        references=[dict(id="external-data", required=True, status="missing", nextAction="Provision separately.")],
        refs=refs, files=["untracked.txt", "ignored.txt"])
    original_index = (source / ".git/index").read_bytes()
    original_config = (source / ".git/config").read_bytes()
    original_git = {p.relative_to(source / ".git").as_posix(): sha(p.read_bytes())
                    for p in (source / ".git").rglob("*") if p.is_file()}
    for action, p, code in [("capture", payload, "CAPTURED"), ("inspect", dict(packagePath=str(package)), "INSPECTED")]:
        result = invoke(binary, source, "checkpoint.git." + action, p)
        assert result["code"] == "CHECKPOINT_GIT_" + code, result
        d = result["data"]
        assert d["state"] == "requires_rebind" and d["readyToResume"] is False and d["kind"] == "git-wip", d
        assert d["checkpointId"] == "fictional-git" and d["references"] == payload["references"], d
    assert (source / ".git/index").read_bytes() == original_index and (source / ".git/config").read_bytes() == original_config
    assert state(source, refs) == expected_source and not (source / ".ocmi").exists()
    assert {p.relative_to(source / ".git").as_posix(): sha(p.read_bytes())
            for p in (source / ".git").rglob("*") if p.is_file()} == original_git, "capture mutated source Git storage"
    assert not filter_marker.exists() and not hook_marker.exists(), "source code/config executed during capture"
    manifest = json.loads((package / "manifest.json").read_bytes())
    assert manifest["schemaVersion"] == 2 and manifest["kind"] == "git-wip"
    g = manifest["git"]
    assert g["headOid"] == expected["head"].decode().strip() and g["attachedRef"] == "refs/heads/main"
    assert {r["name"]: r["oid"] for r in g["refs"]} == {r: v.decode().strip() for r, v in expected["refs"].items()}
    assert {f["path"] for f in g["files"]} == set(expected["files"]) | {"delete.txt", "stage-delete.txt"}
    artifact_names = {"manifest.json", "COMPLETE", "history.bundle"} | {"data/blobs/" + b["sha256"] for b in g["blobs"]}
    assert {p.relative_to(package).as_posix() for p in package.rglob("*") if p.is_file()} == artifact_names
    for b in g["blobs"]:
        raw = (package / "data/blobs" / b["sha256"]).read_bytes()
        assert len(raw) == b["size"] and sha(raw) == b["sha256"] and oid(raw) == b["oid"]
    text = (package / "manifest.json").read_text(encoding="utf-8")
    assert str(source) not in text and "fixture.invalid" not in text
    invoke(binary, source, "checkpoint.git.capture", payload, ok=False)
    invoke(binary, source, "checkpoint.git.inspect", dict(packagePath=str(package)), actor="worker", ok=False)
    invoke(binary, source, "checkpoint.git.inspect", dict(packagePath=str(package)), revision=1, ok=False)
    hidden = base / "unavailable-source"; source.rename(hidden)
    destination = base / "restored different root Ω"; destination.mkdir()
    restored = invoke(binary, destination, "checkpoint.git.restore", dict(packagePath=str(package)))
    assert restored["code"] == "CHECKPOINT_GIT_RESTORED" and restored["data"]["readyToResume"] is False
    assert state(destination, refs) == expected, "HEAD/refs/index/worktree did not recover exactly"
    assert not filter_marker.exists() and not hook_marker.exists()
    config = (destination / ".git/config").read_text(encoding="utf-8")
    assert "fixture.invalid" not in config and "filter" not in config and "hooksPath" not in config
    assert not (destination / ".git/hooks/pre-commit").exists()
    occupied_before = state(destination, refs)
    invoke(binary, destination, "checkpoint.git.restore", dict(packagePath=str(package)), ok=False)
    assert state(destination, refs) == occupied_before
    print("PASS: closed Git/WIP package restores exact HEAD/refs/index/native files without source/config/task DB")
    for i, kind in enumerate(["missing-complete", "truncated-bundle", "header-only", "extra", "blob", "index-oid",
                              "traversal", "duplicate-path", "unknown-field", "duplicate-key", "version",
                              "resealed-non-bundle", "missing-owner", "unbound-head", "bundle-path",
                              "casefold-path", "orphan-blob", "resealed-truncated-pack"]):
        bad = base / ("corrupt-" + str(i)); shutil.copytree(package, bad)
        m = json.loads((bad / "manifest.json").read_bytes())
        if kind == "missing-complete": (bad / "COMPLETE").unlink()
        elif kind == "truncated-bundle": (bad / "history.bundle").write_bytes(b"truncated")
        elif kind == "header-only":
            raw = b"# v2 git bundle\n\n"; (bad / "history.bundle").write_bytes(raw)
            m["git"]["bundle"].update(size=len(raw), sha256=sha(raw)); seal(bad, m)
        elif kind == "extra": (bad / "surprise").write_bytes(b"not declared")
        elif kind == "blob": (bad / "data/blobs" / m["git"]["blobs"][0]["sha256"]).write_bytes(b"changed")
        elif kind == "index-oid": m["git"]["index"][0]["oid"] = "0" * 40; seal(bad, m)
        elif kind == "traversal": m["git"]["files"][0]["path"] = "../escaped"; seal(bad, m)
        elif kind == "duplicate-path": m["git"]["files"].append(dict(m["git"]["files"][0])); seal(bad, m)
        elif kind == "unknown-field": m["remote"] = "forbidden"; seal(bad, m)
        elif kind == "duplicate-key":
            raw = (bad / "manifest.json").read_bytes(); raw = b'{"kind":"git-wip",' + raw[1:]; seal(bad, m, raw)
        elif kind == "version": m["schemaVersion"] = 99; seal(bad, m)
        elif kind == "resealed-non-bundle":
            raw = b"This is not any Git bundle.\n"; (bad / "history.bundle").write_bytes(raw)
            m["git"]["bundle"].update(size=len(raw), sha256=sha(raw)); seal(bad, m)
        elif kind == "missing-owner": m["owner"] = ""; seal(bad, m)
        elif kind == "unbound-head": m["git"]["headOid"] = "0" * 40; m["git"]["attachedRef"] = "refs/heads/undeclared"; seal(bad, m)
        elif kind == "bundle-path": m["git"]["bundle"]["path"] = "not-history.bundle"; seal(bad, m)
        elif kind == "casefold-path": m["git"]["files"][0]["path"] = m["git"]["files"][1]["path"].upper(); seal(bad, m)
        elif kind == "orphan-blob":
            raw = b"unreferenced fictional blob\n"; digest = sha(raw)
            (bad / "data/blobs" / digest).write_bytes(raw)
            m["git"]["blobs"].append(dict(sha256=digest, oid=oid(raw), size=len(raw)))
            m["git"]["blobs"].sort(key=lambda x: x["sha256"]); seal(bad, m)
        elif kind == "resealed-truncated-pack":
            raw = (bad / "history.bundle").read_bytes()[:-24]
            (bad / "history.bundle").write_bytes(raw)
            m["git"]["bundle"].update(size=len(raw), sha256=sha(raw)); seal(bad, m)
        empty = base / ("rejected-destination-" + str(i)); empty.mkdir()
        invoke(binary, empty, "checkpoint.git.inspect", dict(packagePath=str(bad)), ok=False)
        invoke(binary, empty, "checkpoint.git.restore", dict(packagePath=str(bad)), ok=False)
        assert not list(empty.iterdir()) and not (base / "escaped").exists()
    print("PASS: eighteen corrupt/partial/resealed/ambiguous packages refuse without destination effects")
    link = base / "linked-package"; directory_link(link, package)
    plain = base / "plain-empty"; plain.mkdir()
    try:
        invoke(binary, plain, "checkpoint.git.restore", dict(packagePath=str(link)), ok=False)
        assert not list(plain.iterdir())
    finally:
        remove_directory_link(link)
    for i, operation in enumerate(["detached", "unmerged", "included-config", "linked-git", "source-link",
                                   "merge-state", "skip-worktree", "assume-valid", "alternates", "shallow"]):
        candidate = base / ("unsafe-source-" + str(i)); shutil.copytree(hidden, candidate)
        if operation == "detached": git(candidate, "update-ref", "--no-deref", "HEAD", expected["head"].decode().strip())
        elif operation == "unmerged":
            entry = b"100644 " + oid(b"STAGED raw\r\n").encode() + b" 1\tconflict.txt\0"
            git(candidate, "update-index", "--add", "-z", "--index-info", data=entry)
        elif operation == "included-config":
            with (candidate / ".git/config").open("a", encoding="utf-8") as f: f.write("\n[include]\n\tpath = ../foreign-config\n")
        elif operation == "linked-git": (candidate / ".git").rename(candidate / "gitdir"); (candidate / ".git").write_text("gitdir: gitdir\n")
        elif operation == "source-link":
            nested = candidate / "nested"; nested.rename(candidate / "external-nested"); directory_link(nested, candidate / "external-nested")
        elif operation == "merge-state": (candidate / ".git/MERGE_HEAD").write_bytes(expected["head"])
        elif operation == "skip-worktree": git(candidate, "update-index", "--skip-worktree", "change.filtered")
        elif operation == "assume-valid": git(candidate, "update-index", "--assume-unchanged", "change.filtered")
        elif operation == "alternates":
            alternative = base / "empty-alternate-objects"; alternative.mkdir()
            info = candidate / ".git/objects/info"; info.mkdir(exist_ok=True)
            (info / "alternates").write_text(alternative.as_posix() + "\n", encoding="utf-8")
        elif operation == "shallow": (candidate / ".git/shallow").write_bytes(expected["head"])
        before = (candidate / ".git/index").read_bytes() if (candidate / ".git").is_dir() else b""
        output = base / ("refused-output-" + str(i))
        try:
            invoke(binary, candidate, "checkpoint.git.capture", dict(payload, packagePath=str(output)), ok=False)
            assert not output.exists()
            if before: assert (candidate / ".git/index").read_bytes() == before
        finally:
            if operation == "source-link": remove_directory_link(candidate / "nested")
    print("PASS: ten unsupported source states refuse without source index mutation")
    print("PASS: Git checkpoint independent consumer; owned fixtures retained through main acceptance")


if __name__ == "__main__":
    parser = argparse.ArgumentParser(); parser.add_argument("--binary"); parser.add_argument("--fixture-only", action="store_true")
    args = parser.parse_args()
    assert args.fixture_only or args.binary, "--binary is required for the actual consumer"
    check(str(pathlib.Path(args.binary).resolve(strict=True)) if args.binary else "", args.fixture_only)
