"""Black-box selected-root lifecycle and native hook checks; fictional data only."""
import argparse
from contextlib import closing
import json
import os
import pathlib
import sqlite3
import subprocess
import sys
import tempfile
import uuid

from check_cli import decode


PATHS = dict(system='system', bin='system/bin', sync='OCMI_Sync',
             coordinator='OCMI_Sync/coordinator', personal='OCMI_Sync/personal',
             recovery='OCMI_Sync/recovery', local='local', state='local/state',
             store='local/state/state.sqlite', bindings='local/bindings',
             cache='local/cache', runs='local/runs')


def declaration():
    return dict(schemaVersion=1, layoutVersion=1, kind='installation', paths=PATHS)


class Client:
    def __init__(self, binary, root, kind):
        self.binary, self.root, self.kind, self.revision = binary, root, kind, 0

    def request(self, action, payload=None, actor='main', key=None, revision=None, ok=True):
        request = dict(schemaVersion=1, projectRoot=str(self.root), rootKind=self.kind,
                       action=action, actor=actor, requestId=key or str(uuid.uuid4()),
                       expectedRevision=self.revision if revision is None else revision,
                       payload=payload or {})
        process = subprocess.run([self.binary, 'request'], input=json.dumps(request),
                                 text=True, encoding='utf-8', capture_output=True, timeout=15)
        response = decode(process)
        assert response['ok'] is ok, (action, response, process.stderr)
        self.revision = max(self.revision, response['revision'])
        return response['data']


def cycle(binary, root, kind):
    root.mkdir()
    if kind == 'installation':
        (root/'.ocmi-home.json').write_text(json.dumps(declaration()), encoding='utf-8')
    scope = root/'OCMI_Sync/coordinator' if kind == 'installation' else root/'selected'
    scope.mkdir(parents=True)
    (scope/'contract.md').write_text('Fictional accepted component.', encoding='utf-8')
    (scope/'input.txt').write_text('fixed selected input', encoding='utf-8')
    (scope/'user-note.txt').write_text('Fictional retained private note.', encoding='utf-8')
    user_note = (scope/'user-note.txt').read_bytes()
    observation = scope/'observed-tmp.json'
    relative = lambda p: p.relative_to(root).as_posix()
    client = Client(binary, root, kind)
    client.request('init', dict(projectId='fictional-component', deviceId='device', copyId='copy', owner='main'))
    code = ("import json,pathlib,sys,tempfile; p=pathlib.Path(tempfile.gettempdir()); "
            "(p/'generated.txt').write_text('fictional temporary result'); "
            "pathlib.Path(sys.argv[1]).write_text(json.dumps({'tmp':str(p)}))")
    client.request('task.bind', dict(taskId='component', outcomeId='component-outcome',
        taskRevision=1, completionScope='target', worker='worker-1', reviewer='main',
        contract=relative(scope/'contract.md'), inputs=[relative(scope/'input.txt')],
        command=dict(id='actual-child', executable=sys.executable, args=['-c', code, str(observation)],
                     cwd=relative(scope), timeoutMs=5000, outputLimitBytes=2048)))
    client.request('task.ready')
    client.request('stop', {'enabled': True})
    client.request('run', actor='worker-1', ok=False)
    assert not observation.exists(), 'STOP allowed an effect'
    client.request('stop', {'enabled': False})
    key = str(uuid.uuid4())
    result = client.request('run', actor='worker-1', key=key)
    assert result['operationState'] == 'succeeded' and result['exitCode'] == 0, result
    client.request('run', actor='worker-1', key=key, revision=0)
    observed = pathlib.Path(json.loads(observation.read_text(encoding='utf-8'))['tmp'])
    run_root = root/('local/runs' if kind == 'installation' else '.ocmi/local/runs')
    assert observed.resolve().is_relative_to(run_root.resolve()), 'child TMP used the wrong root profile'
    assert not observed.exists(), 'owned disposable run directory remains after success'
    rows = client.request('resource.list', {'limit': 100})
    assert rows['summary']['pending'] == 0, rows
    assert any(row['kind'] == 'process' and row['observedState'] == 'stopped' for row in rows['resources'])
    assert any(row['kind'] == 'directory' and row['observedState'] == 'absent' for row in rows['resources'])
    store = root/('local/state/state.sqlite' if kind == 'installation' else '.ocmi/local/state.sqlite')
    assert store.is_file(), 'selected state store missing'
    with closing(sqlite3.connect(store.as_uri()+'?mode=ro', uri=True)) as database:
        workspaces = [row[0] for row in database.execute("SELECT locator FROM resource_ledger WHERE id LIKE 'ocmi-workspace-%'")]
        assert len(workspaces) == 1 and workspaces[0].startswith(relative(run_root)+'/'), workspaces
    if kind == 'installation':
        client.request('stop', {'enabled': True})
        event = dict(hook_event_name='PreToolUse', tool_name='mcp__ocmi__ocmi_request',
                     tool_input=dict(action='run', payload={}))
        process = subprocess.run([binary, 'hook', '--project-root', str(root), '--root-kind', kind, '--actor', 'main'],
                                 input=json.dumps(event), text=True, encoding='utf-8', capture_output=True, timeout=15)
        assert process.returncode == 0, process.stderr
        hook = json.loads(process.stdout)
        assert hook.get('hookSpecificOutput', {}).get('permissionDecision') == 'deny', hook
        client.request('stop', {'enabled': False})
        for action in ('handover.export', 'handover.import', 'state.upgrade', 'checkpoint.capture', 'environment.inspect'):
            client.request(action, ok=False)
        assert not (root/'.ocmi').exists(), 'installation created a parallel legacy ledger'
        assert not (root/'OCMI_Sync/personal').exists(), 'optional personal store created implicitly'
    client.request('submit', actor='worker-1')
    client.request('review', {'accept': True, 'reason': 'Reviewed fictional actual child and root evidence.'})
    client.request('integrate', {'candidate': 'fictional-component-v1', 'reason': 'Verified fictional integration.'})
    client.request('resource.close', {'disposition': 'keep', 'reason': 'Retain necessary fictional evidence.'})
    assert client.request('status')['cleanup'] == 'done'
    assert (scope/'user-note.txt').read_bytes() == user_note
    print('PASS:', kind, 'actual child/STOP/replay/state/run accounting/native hook/acceptance/closure')


def check(binary, legacy_only=False):
    with tempfile.TemporaryDirectory(prefix='ocmi-root-layout-') as name:
        base = pathlib.Path(name)
        if legacy_only:
            cycle(binary, base/'project with spaces', 'project')
        else:
            cycle(binary, base/'installation with spaces', 'installation')


if __name__ == '__main__':
    parser = argparse.ArgumentParser()
    parser.add_argument('--binary', required=True)
    parser.add_argument('--legacy-only', action='store_true')
    options = parser.parse_args()
    check(str(pathlib.Path(options.binary).resolve(strict=True)), options.legacy_only)
