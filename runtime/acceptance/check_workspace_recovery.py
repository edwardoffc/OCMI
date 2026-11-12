"""Native crash-workspace recovery consumer; no implementation or database writes."""
import argparse
import json
import pathlib
import re
import shutil
import subprocess
import sys
import tempfile
import time
import uuid

from check_cli import Client


def wait_for(fn, message, timeout=9):
    end = time.monotonic() + timeout
    while time.monotonic() < end:
        value = fn()
        if value:
            return value
        time.sleep(.05)
    raise AssertionError(message)


def candidate(c, resource):
    data = c.request('resource.inspect', {'resourceId': resource})
    recovery = data.get('recovery', {})
    assert recovery.get('eligible') is True, data
    assert re.fullmatch('[0-9a-f]{64}', recovery.get('inventorySHA256', '')), recovery
    assert isinstance(recovery.get('entries'), list) and recovery['entries'], recovery
    return recovery


def check(binary, prior_failed_disposal):
    with tempfile.TemporaryDirectory(prefix='ocmi-workspace-recovery-') as temp:
        base = pathlib.Path(temp)
        root = base / 'selected fictional project'
        root.mkdir()
        (root / 'contract.md').write_text('Review and dispose only this fictional registered crash workspace.', encoding='utf-8')
        (root / 'input.txt').write_text('fixed', encoding='utf-8')
        foreign = root / 'foreign.txt'
        foreign.write_bytes(b'foreign data must survive')
        code = '''import pathlib,sys,tempfile,time
root=pathlib.Path(sys.argv[1]); scratch=pathlib.Path(tempfile.gettempdir())
(scratch/'generated.txt').write_text('fictional temporary evidence',encoding='utf-8')
(root/'scratch.txt').write_text(str(scratch),encoding='utf-8')
end=time.monotonic()+20
while not (root/'finish-child').exists() and time.monotonic()<end: time.sleep(.02)
'''
        c = Client(binary, root)
        c.request('init', dict(projectId='crash-workspace', deviceId='fictional-device', copyId='fictional-copy', owner='main'))
        c.request('task.bind', dict(taskId='crash-task', outcomeId='crash-outcome', taskRevision=1,
            completionScope='target', worker='worker-1', reviewer='main', contract='contract.md',
            inputs=['input.txt'], command=dict(id='bounded-child', executable=sys.executable,
                args=['-c', code, str(root)], cwd='.', timeoutMs=25000, outputLimitBytes=4096)))
        c.request('task.ready')
        request = dict(schemaVersion=1, action='run', projectRoot=str(root), actor='worker-1',
            requestId=str(uuid.uuid4()), expectedRevision=c.revision, payload={})
        run = subprocess.Popen([binary, 'request'], stdin=subprocess.PIPE, stdout=subprocess.PIPE,
            stderr=subprocess.PIPE, text=True, encoding='utf-8')
        try:
            run.stdin.write(json.dumps(request)); run.stdin.close(); run.stdin = None
            wait_for(lambda: (root / 'scratch.txt').exists(), 'real child did not create scratch')
            scratch = pathlib.Path((root / 'scratch.txt').read_text(encoding='utf-8'))
            assert scratch.resolve().is_relative_to((root / '.ocmi/local/runs').resolve())
            operation = c.request('status')['operationId']
            resource = 'ocmi-workspace-' + operation
            live = c.request('operation.inspect', dict(operationId=operation))
            assert live['ownership']['observation'] == 'live', live
            assert not c.request('resource.inspect', dict(resourceId=resource)).get('recovery', {}).get('eligible')
            payload = dict(resourceId=resource, reason='Reviewed fictional crash workspace', inventorySHA256='0'*64)
            c.request('resource.reconcile', payload, ok=False)
            run.kill(); run.communicate(timeout=5)
            # Unix has no parent-death Job guarantee: let only this known fixture
            # child exit, then require actual native group absence, never infer it.
            (root / 'finish-child').write_bytes(b'finish own fixture child')
            proof = wait_for(lambda: (v if v['ownership']['canReconcile'] else None)
                if (v := c.request('operation.inspect', dict(operationId=operation))) else None,
                'registered native process/group absence not established')
            assert proof['ownership']['observation'] == 'absent'
            c.request('stop', dict(enabled=True))
            c.request('operation.reconcile', dict(operationId=operation, disposition='abandon', reason='Observed own supervisor death and native absence'))
            c.request('resource.reconcile', payload, ok=False)
            for row in c.request('resource.list')['resources']:
                if row.get('operationId') == operation:
                    detail = c.request('resource.inspect', dict(resourceId=row['resourceId']))
                    if detail['activeUses']:
                        c.request('resource.release', dict(resourceId=row['resourceId'], reason='Own registered operation is verified terminal and absent'))
            # Both actual persisted states matter: ordinary interrupted active
            # creation and the historical failed-disposal state retained by callers.
            if prior_failed_disposal:
                c.request('resource.dispose', dict(resourceId=resource, disposition='remove', reason='Inventory not yet recovered'), ok=False)
            assert (scratch / 'generated.txt').read_text(encoding='utf-8') == 'fictional temporary evidence'
            observed = candidate(c, resource)
            payload['inventorySHA256'] = observed['inventorySHA256']
            c.request('resource.reconcile', payload, actor='worker-1', ok=False)
            c.request('resource.reconcile', payload, revision=max(0, c.revision-1), ok=False)
            c.request('resource.reconcile', dict(resourceId=resource, reason='No snapshot is not approval'), ok=False)
            c.request('resource.hold', dict(resourceId=resource, enabled=True, reason='Preserve evidence', until='review'))
            c.request('resource.reconcile', payload, ok=False)
            c.request('resource.hold', dict(resourceId=resource, enabled=False, reason='Evidence reviewed', until='now'))
            # Review must describe this exact snapshot, not a stale previous hash.
            (scratch / 'generated.txt').write_text('fictional reviewed revision', encoding='utf-8')
            c.request('resource.reconcile', payload, ok=False)
            observed = candidate(c, resource)
            payload['inventorySHA256'] = observed['inventorySHA256']
            # Copying all metadata and marker bytes cannot transfer original native provenance.
            copied = base / 'copied store'
            shutil.copytree(root, copied)
            other = Client(binary, copied); other.request('status')
            other.request('resource.reconcile', payload, ok=False)
            key = str(uuid.uuid4())
            c.request('resource.reconcile', payload, key=key)
            assert scratch.exists(), 'reconcile deleted diagnostics without disposition'
            c.request('resource.reconcile', payload, key=key, revision=0)
            state = c.request('status')
            assert state['operationState'] == 'incomplete' and state['failureCount'] == 1 and state['stopped'], state
            assert (scratch / 'generated.txt').read_text(encoding='utf-8') == 'fictional reviewed revision'
            c.request('resource.dispose', dict(resourceId=resource, disposition='remove', reason='Exact reviewed owned diagnostic can now be removed'))
            assert not scratch.parent.exists(), 'managed operation workspace still exists'
            final = c.request('status')
            assert final['resourceSummary']['activeUses'] == 0 and final['resourceSummary']['pending'] == 0, final
            assert final['operationState'] == 'incomplete' and final['failureCount'] == 1
            assert foreign.read_bytes() == b'foreign data must survive'
            print('PASS: ' + ('old failed-disposal' if prior_failed_disposal else 'fresh crash without prior failed disposal') +
                ', actual native supervisor death, complete snapshot, '
                'live/use/owner/revision/hold/stale/copy guards, replay, diagnostics preservation and exact cleanup')
        finally:
            if run.poll() is None:
                run.kill(); run.communicate(timeout=5)


if __name__ == '__main__':
    parser = argparse.ArgumentParser()
    parser.add_argument('--binary', required=True)
    args = parser.parse_args()
    binary = str(pathlib.Path(args.binary).resolve(strict=True))
    check(binary, prior_failed_disposal=False)
    check(binary, prior_failed_disposal=True)
