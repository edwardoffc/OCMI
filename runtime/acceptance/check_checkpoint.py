"""Independent selected-file checkpoint consumers; fictional local data only."""
import argparse
import hashlib
import json
import os
import pathlib
import shutil
import subprocess
import tempfile
import uuid

from check_cli import decode


def invoke(binary, root, action, payload, ok=True, actor="main"):
    request = dict(schemaVersion=1, action=action, projectRoot=str(root), actor=actor,
                   requestId=str(uuid.uuid4()), expectedRevision=0, payload=payload)
    process = subprocess.run([binary, "request"], input=json.dumps(request), text=True,
                             encoding="utf-8", capture_output=True, timeout=30)
    response = decode(process)
    assert response["ok"] is ok, (action, response, process.stderr)
    return response["data"]


def inventory(root):
    return {p.relative_to(root).as_posix(): p.read_bytes()
            for p in root.rglob("*") if p.is_file()}


def seal(package, manifest):
    raw = json.dumps(manifest, ensure_ascii=False, separators=(",", ":")).encode("utf-8")
    (package / "manifest.json").write_bytes(raw)
    digest = hashlib.sha256(raw).hexdigest()
    (package / "COMPLETE").write_text(json.dumps(dict(schemaVersion=1, manifestSha256=digest)), encoding="utf-8")


def directory_link(link, target):
    if os.name == "nt":
        result = subprocess.run(["cmd", "/c", "mklink", "/J", str(link), str(target)],
                                capture_output=True, timeout=10)
        assert result.returncode == 0, "junction control unavailable"
    else:
        link.symlink_to(target, target_is_directory=True)


def remove_directory_link(link):
    if os.name == "nt": os.rmdir(link)
    else: link.unlink()


def assert_metadata(value, files):
    assert value["checkpointId"] == "fictional-topic" and value["fileCount"] == len(files), value
    assert value["readyToResume"] is False and value["state"] == "requires_rebind", value
    assert len(value["manifestSha256"]) == 64 and value["nextAction"], value
    assert value["references"] == [dict(id="fixture-data", required=True, status="missing",
                                       nextAction="Provision separately from the protected source.")], value


def check(binary):
    with tempfile.TemporaryDirectory(prefix="ocmi-checkpoint-") as temp:
        base = pathlib.Path(temp)
        source = base / "selected topic Ω"; source.mkdir()
        selected = {"HANDOFF.md": b"Continue fictional topic.\n", "nested/notes.txt": "Тест Ω\n".encode(),
                    "binary.bin": b"\x00\xff\x10\r\n", "empty.txt": b""}
        for name, data in selected.items():
            path = source / name; path.parent.mkdir(parents=True, exist_ok=True); path.write_bytes(data)
        (source / "unselected.txt").write_bytes(b"Do not export another topic.")
        package = base / "private checkpoint"
        payload = dict(packagePath=str(package), checkpointId="fictional-topic", systemVersion="1.3.5",
                       sourceIdentity="fictional-source-sha256", owner="synthetic-orchestrator",
                       purpose="Selected-topic recovery", retainUntil="Verified recovery acceptance",
                       nextAction="Read HANDOFF after confirming the selected binding.", files=list(selected),
                       references=[dict(id="fixture-data", required=True, status="missing",
                                        nextAction="Provision separately from the protected source.")])
        before = inventory(source)
        captured = invoke(binary, source, "checkpoint.capture", payload)
        assert_metadata(captured, selected)
        assert inventory(source) == before and not (source / ".ocmi").exists(), "capture altered source"
        assert (package / "COMPLETE").is_file()
        manifest = json.loads((package / "manifest.json").read_text(encoding="utf-8"))
        assert manifest["schemaVersion"] == 1 and manifest["kind"] == "files", manifest
        assert {entry["path"] for entry in manifest["files"]} == set(selected)
        assert str(source) not in (package / "manifest.json").read_text(encoding="utf-8"), "native root leaked"
        assert {p.relative_to(package).as_posix() for p in package.rglob("*") if p.is_file()} == {
            "manifest.json", "COMPLETE", *["data/" + name for name in selected]}, "unexpected package payload"
        assert_metadata(invoke(binary, source, "checkpoint.inspect", dict(packagePath=str(package))), selected)
        intact = inventory(package)
        invoke(binary, source, "checkpoint.capture", payload, ok=False)
        assert inventory(package) == intact, "capture overwrote an existing checkpoint"
        renamed = base / "unavailable old source"; source.rename(renamed)
        destination = base / "fresh root"; destination.mkdir()
        restored = invoke(binary, destination, "checkpoint.restore", dict(packagePath=str(package)))
        assert_metadata(restored, selected)
        assert inventory(destination) == selected and not (destination / ".ocmi").exists(), "lost/extra restored files"
        invoke(binary, destination, "checkpoint.restore", dict(packagePath=str(package)), ok=False)
        assert inventory(destination) == selected, "replayed restore changed destination"
        print("PASS: selected file-only checkpoint/inspect/fresh-root restore without source or task DB")

        foreign = base / "foreign"; foreign.mkdir(); (foreign / "sentinel").write_bytes(b"preserve")
        invoke(binary, foreign, "checkpoint.restore", dict(packagePath=str(package)), ok=False)
        assert inventory(foreign) == {"sentinel": b"preserve"}
        invoke(binary, renamed, "checkpoint.capture", dict(payload, packagePath=str(base / "worker-denied")), ok=False, actor="worker-1")
        assert not (base / "worker-denied").exists()
        print("PASS: foreign destination, repeated output and worker-label guards preserve existing data")

        for label in ("changed-data", "missing-complete", "changed-manifest", "extra-data", "missing-data",
                      "traversal", "case-collision", "directory-collision", "unsupported-version", "unknown-kind",
                      "duplicate-manifest-key", "duplicate-complete-key", "trailing-json", "unknown-field"):
            broken = base / label; shutil.copytree(package, broken)
            edited = json.loads((broken / "manifest.json").read_text(encoding="utf-8"))
            if label == "changed-data": (broken / "data" / "HANDOFF.md").write_bytes(b"changed")
            elif label == "missing-complete": (broken / "COMPLETE").unlink()
            elif label == "changed-manifest": (broken / "manifest.json").write_bytes(b"{}")
            elif label == "extra-data": (broken / "data" / "unexpected.txt").write_bytes(b"extra")
            elif label == "missing-data": (broken / "data" / "HANDOFF.md").unlink()
            elif label == "traversal": edited["files"][0]["path"] = "../outside.txt"; seal(broken, edited)
            elif label == "case-collision": edited["files"].append(dict(edited["files"][0], path="handoff.md")); seal(broken, edited)
            elif label == "directory-collision": edited["files"].append(dict(edited["files"][0], path="HANDOFF.md/child")); seal(broken, edited)
            elif label == "unsupported-version": edited["schemaVersion"] = 999; seal(broken, edited)
            elif label == "unknown-kind": edited["kind"] = "git"; seal(broken, edited)
            elif label == "duplicate-manifest-key":
                raw = (broken / "manifest.json").read_text(encoding="utf-8")
                raw = '{"schemaVersion":999,' + raw.lstrip()[1:]
                (broken / "manifest.json").write_text(raw, encoding="utf-8")
                sha = hashlib.sha256((broken / "manifest.json").read_bytes()).hexdigest()
                (broken / "COMPLETE").write_text(json.dumps(dict(schemaVersion=1, manifestSha256=sha)), encoding="utf-8")
            elif label == "duplicate-complete-key":
                raw = (broken / "COMPLETE").read_text(encoding="utf-8")
                (broken / "COMPLETE").write_text('{"schemaVersion":999,' + raw.lstrip()[1:], encoding="utf-8")
            elif label == "trailing-json":
                raw = (broken / "COMPLETE").read_text(encoding="utf-8")
                (broken / "COMPLETE").write_text(raw + '{}', encoding="utf-8")
            elif label == "unknown-field": edited["automaticRun"] = True; seal(broken, edited)
            target = base / (label + "-target"); target.mkdir()
            invoke(binary, target, "checkpoint.inspect", dict(packagePath=str(broken)), ok=False)
            invoke(binary, target, "checkpoint.restore", dict(packagePath=str(broken)), ok=False)
            assert list(target.iterdir()) == [], (label, "invalid package changed destination")
        assert not (base / "outside.txt").exists()
        print("PASS: fourteen corrupt/partial/ambiguous/unsafe packages refused before destination effects")

        for i, names in enumerate((["../outside.txt"], ["HANDOFF.md", "HANDOFF.md"], ["missing.txt"],
                                  [".env"], ["state.sqlite"], ["CON.txt"], [".git/config"], [])):
            bad = base / ("selection-" + str(i))
            invoke(binary, renamed, "checkpoint.capture", dict(payload, packagePath=str(bad), files=names), ok=False)
            assert not bad.exists()
        for i, name in enumerate((".env", "state.sqlite", "private.pem", ".codex/config.toml")):
            sensitive = renamed / name; sensitive.parent.mkdir(parents=True, exist_ok=True)
            sensitive.write_bytes(b"fictional excluded configuration, not a real secret")
            bad = base / ("existing-sensitive-" + str(i))
            invoke(binary, renamed, "checkpoint.capture", dict(payload, packagePath=str(bad), files=[name]), ok=False)
            assert not bad.exists() and sensitive.read_bytes().startswith(b"fictional")
        too_many = ["HANDOFF.md"] * 513
        invoke(binary, renamed, "checkpoint.capture", dict(payload, packagePath=str(base / "too-many"), files=too_many), ok=False)
        (renamed / "large.bin").write_bytes(b"x" * ((8 << 20) + 1))
        invoke(binary, renamed, "checkpoint.capture", dict(payload, packagePath=str(base / "too-large"), files=["large.bin"]), ok=False)
        (renamed / ".git").mkdir()
        invoke(binary, renamed, "checkpoint.capture", dict(payload, packagePath=str(base / "git-root")), ok=False)
        assert not any(p.exists() for p in (base / "too-many", base / "too-large", base / "git-root"))
        (renamed / ".git").rmdir()
        print("PASS: explicit selection, sensitive/reserved paths, bounded payload and Git refusal")

        link = base / "linked-source"
        directory_link(link, renamed)
        try:
            invoke(binary, link, "checkpoint.capture", dict(payload, packagePath=str(base / "link-output")), ok=False)
            assert not (base / "link-output").exists()
        finally:
            remove_directory_link(link)
        linked_package = base / "linked-package"; directory_link(linked_package, package)
        linked_target = base / "linked-target"; plain_target = base / "plain-target"; plain_target.mkdir()
        directory_link(linked_target, plain_target)
        linked_parent = base / "linked-parent"; parent_target = base / "parent-target"; parent_target.mkdir()
        child = parent_target / "empty-child"; child.mkdir(); directory_link(linked_parent, parent_target)
        broken_link = base / "linked-data"; shutil.copytree(package, broken_link)
        moved_payload = base / "payload-origin"; (broken_link / "data" / "nested").rename(moved_payload)
        payload_link = broken_link / "data" / "nested"; directory_link(payload_link, moved_payload)
        try:
            invoke(binary, plain_target, "checkpoint.inspect", dict(packagePath=str(linked_package)), ok=False)
            invoke(binary, plain_target, "checkpoint.restore", dict(packagePath=str(linked_package)), ok=False)
            invoke(binary, linked_target, "checkpoint.restore", dict(packagePath=str(package)), ok=False)
            invoke(binary, linked_parent / "empty-child", "checkpoint.restore", dict(packagePath=str(package)), ok=False)
            invoke(binary, plain_target, "checkpoint.inspect", dict(packagePath=str(broken_link)), ok=False)
            invoke(binary, plain_target, "checkpoint.restore", dict(packagePath=str(broken_link)), ok=False)
            assert list(plain_target.iterdir()) == [] and list(child.iterdir()) == []
            assert (moved_payload / "notes.txt").read_bytes() == selected["nested/notes.txt"]
        finally:
            for link in (payload_link, linked_package, linked_target, linked_parent): remove_directory_link(link)
        print("PASS: actual native source/package/payload/destination/ancestor links refused without traversal")


if __name__ == "__main__":
    parser = argparse.ArgumentParser(); parser.add_argument("--binary", required=True)
    args = parser.parse_args(); check(str(pathlib.Path(args.binary).resolve(strict=True)))
