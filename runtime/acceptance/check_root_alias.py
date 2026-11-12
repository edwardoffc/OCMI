"""Independent real-CLI selected-root aliases; all data is fictional and local."""
import argparse
import os
import pathlib
import tempfile

from check_cli import Client, scenario


def check(binary):
    with tempfile.TemporaryDirectory(prefix="ocmi-root-alias-") as temp:
        base = pathlib.Path(temp)
        for label in ("native", "trailing", "forward", "forward-trailing", "dot"):
            root = base / (label + " spaces Ω")
            c = scenario(binary, root)
            # Use the exact supplied string rather than letting pathlib normalize it.
            alias = str(root)
            if label.startswith("forward"):
                alias = root.as_posix()
            if label in ("trailing", "forward-trailing"):
                alias += "/" if label.startswith("forward") else os.sep
            if label == "dot":
                alias += os.sep + "."
            c.root = alias
            before = c.request("status")
            package = c.request("handover.export", dict(
                transferId="alias-" + label, files=["contract.md", "input.txt"]))["package"]
            assert package["source"]["projectId"] == before["projectId"], package
            assert set(package["files"]) == {"contract.md", "input.txt"}, package
            destination = base / (label + " restored")
            destination.mkdir()
            d = Client(binary, destination)
            d.request("handover.import", dict(package=package, deviceId="new-device", copyId=label))
            for name in ("contract.md", "input.txt"):
                assert (destination / name).read_bytes() == (root / name).read_bytes()
            state = d.request("status")
            assert state["stopped"] is True and state["taskState"] == "draft", state
            assert not (root / "effect.txt").exists(), "export unexpectedly ran task"
            print("PASS: actual export/import for selected-root", label)


if __name__ == "__main__":
    parser = argparse.ArgumentParser()
    parser.add_argument("--binary", required=True)
    args = parser.parse_args()
    check(str(pathlib.Path(args.binary).resolve(strict=True)))
