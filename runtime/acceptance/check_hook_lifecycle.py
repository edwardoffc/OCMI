"""New hook actions must preserve access to core-validated diagnosis and recovery."""
import argparse
import json
import pathlib
import subprocess
import tempfile

from check_cli import scenario


def check(binary):
    with tempfile.TemporaryDirectory(prefix="ocmi-hook-lifecycle-") as temp:
        c = scenario(binary, pathlib.Path(temp) / "selected")
        def hook(action, payload=None):
            event = dict(hook_event_name="PreToolUse", tool_name="mcp__ocmi__ocmi_request",
                         tool_input=dict(action=action, payload=payload or {}))
            result = subprocess.run([binary, "hook", "--project-root", str(c.root), "--actor", "main"],
                                    input=json.dumps(event), text=True, encoding="utf-8",
                                    capture_output=True, timeout=10)
            assert result.returncode == 0, result.stderr
            return json.loads(result.stdout).get("hookSpecificOutput", {}).get("permissionDecision") == "deny"
        assert not hook("task.revise"), "new valid action treated as unknown"
        c.request("stop", dict(enabled=True))
        for action in ("status", "task.history", "operation.inspect", "operation.reconcile", "state.upgrade"):
            assert not hook(action), (action, "preservation cannot reach core under STOP")
        for action in ("run", "task.revise", "outcome.reassess", "outcome.review", "outcome.decide"):
            assert hook(action), (action, "consequential call bypasses STOP hook")
        assert hook("unlisted-action"), "unknown action bypasses hook"
        assert c.request("status")["stopped"] is True, "hook changed core state"
        print("PASS: new actions recognized; preservation reaches core under STOP; grants/mutations denied; read-only hook")


if __name__ == "__main__":
    parser = argparse.ArgumentParser()
    parser.add_argument("--binary", required=True)
    args = parser.parse_args()
    check(str(pathlib.Path(args.binary).resolve(strict=True)))
