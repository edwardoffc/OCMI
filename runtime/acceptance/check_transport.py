"""Independent CLI framing and real hook-process controls."""
import argparse
import contextlib
import json
import pathlib
import sqlite3
import subprocess
import tempfile

from check_cli import decode, scenario


def check(binary):
    with tempfile.TemporaryDirectory(prefix='ocmi-transport-') as temp:
        c = scenario(binary, pathlib.Path(temp)/'selected')
        request = dict(schemaVersion=1, action='status', projectRoot=str(c.root), actor='main',
                       requestId='bounded-status', expectedRevision=0, payload={})
        for value, expected in ((json.dumps(request), True),
                                (json.dumps(request)+' '*(8*1024*1024), False),
                                ('{"schemaVersion":', False)):
            process = subprocess.run([binary, 'request'], input=value, text=True, encoding='utf-8',
                                     capture_output=True, timeout=15)
            assert decode(process)['ok'] is expected
        print('PASS: valid CLI request, malformed JSON and oversized valid-prefix input')

        def hook(action, payload):
            event = dict(hook_event_name='PreToolUse', tool_name='mcp__ocmi__ocmi_request',
                         tool_input=dict(action=action, payload=payload))
            p = subprocess.run([binary, 'hook', '--project-root', str(c.root), '--actor', 'main'],
                input=json.dumps(event), text=True, encoding='utf-8', capture_output=True, timeout=15)
            assert p.returncode == 0, p.stderr
            return json.loads(p.stdout)

        def denied(result):
            return result.get('hookSpecificOutput', {}).get('permissionDecision') == 'deny'

        assert not denied(hook('handover.rebind', {})), 'known rebind treated as unsupported'
        assert not denied(hook('stop', {'enabled': True})), 'valid STOP enable must pass to core'
        c.request('stop', {'enabled': True})
        c.request('stop', {}, ok=False)
        c.request('stop', {'enabled': None}, ok=False)
        assert denied(hook('run', {}))
        assert denied(hook('stop', {})), 'missing enabled treated as explicit clear'
        assert not denied(hook('stop', {'enabled': False}))
        c.request('stop', {'enabled': False})
        with contextlib.closing(sqlite3.connect(c.root/'.ocmi/local/state.sqlite')) as db:
            db.execute("INSERT INTO operation(id,task_id,state,output,truncated,before_fingerprint,after_fingerprint,created_at) VALUES('lost','task-1','prepared','',0,'x','','2099-01-01')")
            db.commit()
        assert denied(hook('run', {})), 'unknown execution allowed by hook'
        assert not denied(hook('stop', {'enabled': True})), 'unknown execution must not block STOP enable'
        assert not denied(hook('status', {}))
        print('PASS: real hook STOP/explicit clear/rebind/unknown/status controls')


if __name__ == '__main__':
    parser = argparse.ArgumentParser()
    parser.add_argument('--binary', required=True)
    args = parser.parse_args()
    check(str(pathlib.Path(args.binary).resolve(strict=True)))
