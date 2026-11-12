"""Independent real-CLI bounds: raw 32 MiB, >8 MiB history, visible overflow."""
import argparse
import hashlib
import json
import os
import pathlib
import random
import tempfile

from check_git_checkpoint import git, invoke


def check(binary):
    base = pathlib.Path(tempfile.mkdtemp(prefix="ocmi-git-limits-")).resolve()
    (base / "OWNERSHIP.json").write_text(json.dumps(dict(owner="OCMI independent acceptance",
        purpose="Git raw-blob/history bounds and overflow refusal", retainUntil="component acceptance",
        disposition="retain local proof; hosted runner lifetime is managed by CI")), encoding="utf-8")
    print("Owned bounds fixture retained through acceptance:", base, flush=True)
    source = base / "source"; source.mkdir()
    template = base / "template"; template.mkdir()
    fixed = ("-c", "core.autocrlf=false", "-c", "core.attributesFile=" + os.devnull)
    git(source, "init", "--initial-branch=main", "--object-format=sha1", "--template=" + str(template), extra=fixed)
    history = random.Random(521).randbytes(9 << 20)
    (source / "history.bin").write_bytes(history)
    git(source, "add", "history.bin", extra=fixed)
    git(source, "commit", "--no-gpg-sign", "-m", "Fixture", extra=(*fixed, "-c", "user.name=fixture", "-c", "user.email=fixture@example.com"))
    (source / "history.bin").unlink()
    raw = (b"\x00\xfffixture\n" * ((32 << 20) // 10 + 1))[:32 << 20]
    assert len(raw) == 32 << 20
    expected = hashlib.sha256(raw).hexdigest()
    (source / "large.bin").write_bytes(raw)
    git(source, "add", "large.bin", extra=fixed)
    index = git(source, "ls-files", "--stage", "-z", extra=fixed)
    index_bytes = (source / ".git/index").read_bytes()
    package = base / "checkpoint"
    payload = dict(packagePath=str(package), checkpointId="bounds", systemVersion="1.3.5",
                   sourceIdentity="synthetic-bounds", owner="main", purpose="independent bounds",
                   retainUntil="qualification", nextAction="confirm binding", references=[],
                   refs=["refs/heads/main"], files=[])
    invoke(binary, source, "checkpoint.git.capture", payload)
    assert (package / "history.bundle").stat().st_size > 8 << 20, "fixture must discriminate the old 8 MiB reader"
    assert (source / ".git/index").read_bytes() == index_bytes

    # A selected extra is larger than the contract's per-blob cap. The source
    # index and prior package remain usable; no truncated replacement is published.
    (source / "oversize.bin").write_bytes(b"x" * ((32 << 20) + 1))
    overflow = base / "overflow"
    invoke(binary, source, "checkpoint.git.capture", dict(payload, packagePath=str(overflow), files=["oversize.bin"]), ok=False)
    assert not overflow.exists()
    assert (source / ".git/index").read_bytes() == index_bytes
    source.rename(base / "unavailable-source")
    context = base / "context"; context.mkdir()
    invoke(binary, context, "checkpoint.git.inspect", dict(packagePath=str(package)))
    restored = base / "restored"; restored.mkdir()
    result = invoke(binary, restored, "checkpoint.git.restore", dict(packagePath=str(package)))
    assert result["data"]["readyToResume"] is False
    assert git(restored, "ls-files", "--stage", "-z", extra=fixed) == index
    assert not (restored / "history.bin").exists()
    actual = restored / "large.bin"
    assert actual.stat().st_size == 32 << 20 and hashlib.sha256(actual.read_bytes()).hexdigest() == expected
    assert {p.name for p in restored.iterdir()} == {".git", "large.bin"}
    print("PASS: real 32 MiB indexed/worktree blob and >8 MiB history restore without source; 32 MiB+1 refuses, source index preserved", flush=True)


if __name__ == "__main__":
    parser = argparse.ArgumentParser()
    parser.add_argument("--binary", required=True)
    args = parser.parse_args()
    check(str(pathlib.Path(args.binary).resolve(strict=True)))
