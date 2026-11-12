"""Main-owned selected resource maintenance oracle; no runtime implementation imports."""
import argparse
import base64
import json
import os
from pathlib import Path
import subprocess
import sys
import time

sys.path.insert(0, str(Path(__file__).resolve().parents[2] / 'scripts/acceptance'))
from check_install_home import install_fixture
from check_cli import scenario, decode
from check_resources import declare, record, finish, close, attempt


def check(binary, controls=False):
    with install_fixture(preserve_on_failure=True) as temp:
        base = Path(temp)
        c = scenario(binary, base / 'project')
        declare(c, 'artifacts', 'artifacts')
        c.request('resource.apply', {'resourceId': 'artifacts'})
        c.request('resource.write', {'resourceId': 'artifacts', 'path': 'report.txt',
                  'contentBase64': base64.b64encode(b'PRIVATE-ARTIFACT-CANARY\n').decode()}, actor='worker-1')
        group = c.root / 'artifacts'
        old = time.time() - 4 * 86400
        os.utime(group / 'report.txt', (old, old))
        def inspect(resource_id='artifacts', **overrides):
            payload = dict(resourceId=resource_id, sizeLimitBytes=1, ageLimitDays=1,
                           maxEntries=64, maxBytes=16 << 20)
            payload.update(overrides)
            request = dict(schemaVersion=1, projectRoot=str(c.root), action='resource.maintain.inspect',
                           actor='main', requestId='read-only-measurement', expectedRevision=c.revision if controls else 0, payload=payload)
            r = decode(subprocess.run([binary, 'request'], input=json.dumps(request), capture_output=True,
                                      text=True, encoding='utf-8', timeout=20))
            assert r['revision'] == c.revision, 'inspection mutated revision'
            return r
        before = record(c, 'artifacts')['nativeIdentity']
        result = inspect()
        if controls:
            assert not result['ok'] and result['code'] == 'UNKNOWN_ACTION', result
            assert record(c, 'artifacts')['nativeIdentity'] == before
            c.request('resource.use', {'resourceId': 'artifacts', 'access': 'read', 'reason': 'Original fixture read'}, actor='worker-1')
            c.request('resource.hold', {'resourceId': 'artifacts', 'enabled': True, 'reason': 'Original fixture hold', 'until': 'review'})
            refused = attempt(c, 'resource.dispose', {'resourceId': 'artifacts', 'disposition': 'remove', 'reason': 'Must refuse held/used fixture'})
            assert not refused['ok'] and refused['code'] == 'RESOURCE_PENDING', refused
            finish(c)
            close(c, ok=False)
            c.request('resource.release', {'resourceId': 'artifacts', 'reason': 'Verified stopped fixture reader'})
            refused = attempt(c, 'resource.dispose', {'resourceId': 'artifacts', 'disposition': 'remove', 'reason': 'Held only'})
            assert not refused['ok'] and refused['code'] == 'RESOURCE_PENDING', refused
            c.request('resource.hold', {'resourceId': 'artifacts', 'enabled': False, 'reason': 'Review done', 'until': 'now'})
            c.request('resource.dispose', {'resourceId': 'artifacts', 'disposition': 'remove', 'reason': 'Original qualified closure'})
            assert not group.exists()
            close(c)
            assert c.request('status')['cleanup'] == 'done'
            print('MAINTENANCE_BASELINE_CONFIRMED original create/write/use/hold/release/dispose valid, summary missing', flush=True)
            return
        def valid(result):
            assert result['ok'] and result['code'] == 'RESOURCE_MAINTENANCE_INSPECTED', result
            data = result['data']
            assert data['complete'] is True and data['mutationSupported'] is False, data
            assert data['maintenanceRequired'] is True and data['nextAction'], data
            assert 'PRIVATE-ARTIFACT-CANARY' not in json.dumps(data) and str(base) not in json.dumps(data), 'content/path leaked'
            return data
        data = valid(result)
        expected = sum(p.stat().st_size for p in group.rglob('*') if p.is_file())
        assert data['observedBytes'] == expected and data['oldestAgeDays'] >= 3, data
        assert data['sizeExceeded'] is True and data['ageExceeded'] is True, data
        c.request('resource.use', {'resourceId': 'artifacts', 'access': 'read', 'reason': 'Selected active fixture'}, actor='worker-1')
        assert valid(inspect())['activeUses'] == 1
        c.request('resource.hold', {'resourceId': 'artifacts', 'enabled': True, 'reason': 'Keep failure evidence', 'until': 'owner review'})
        assert valid(inspect())['held'] is True
        c.request('stop', {'enabled': True})
        args = [binary, 'call', '--root', str(c.root), '--actor', 'main', '--action',
                'resource.maintain.inspect', '--payload', json.dumps(dict(resourceId='artifacts',
                    sizeLimitBytes=1, ageLimitDays=1, maxEntries=64, maxBytes=16<<20))]
        valid(decode(subprocess.run(args, capture_output=True, text=True, encoding='utf-8', timeout=20)))
        event = dict(hook_event_name='PreToolUse', tool_name='mcp__ocmi__ocmi_request',
                     tool_input=dict(action='resource.maintain.inspect', payload={'resourceId': 'artifacts'}))
        hook = subprocess.run([binary, 'hook', '--project-root', str(c.root), '--actor', 'main'],
                              input=json.dumps(event), capture_output=True, text=True, encoding='utf-8', timeout=20)
        assert hook.returncode == 0 and json.loads(hook.stdout).get('ocmi') == 'safe_selected_call', hook.stdout
        c.request('stop', {'enabled': False})
        refused = attempt(c, 'resource.dispose', {'resourceId': 'artifacts', 'disposition': 'remove', 'reason': 'Cannot delete held/used group'})
        assert not refused['ok'] and refused['code'] == 'RESOURCE_PENDING', refused
        for limits in ({'maxEntries': 1}, {'maxBytes': 1}):
            bounded = inspect(**limits)
            assert not bounded['ok'] and bounded['code'] == 'RESOURCE_MAINTENANCE_INCOMPLETE' and bounded['data'].get('complete') is False and bounded['data']['mutationSupported'] is False, bounded
        foreign = base / 'foreign'
        foreign.mkdir()
        (foreign / 'keep').write_bytes(b'FOREIGN-RESOURCE-CANARY')
        declare(c, 'external', foreign, mode='use')
        c.request('resource.apply', {'resourceId': 'external'})
        unsupported = inspect('external')
        assert not unsupported['ok'] and unsupported['code'] == 'RESOURCE_MAINTENANCE_INCOMPLETE' and unsupported['data']['mutationSupported'] is False, unsupported
        assert (foreign / 'keep').read_bytes() == b'FOREIGN-RESOURCE-CANARY'
        state = valid(inspect('ocmi-state'))
        assert state['observedBytes'] > 0 and state['protected'] is True, state
        assert state['nextAction'] and state['mutationSupported'] is False
        finish(c)
        close(c, ok=False)
        c.request('resource.release', {'resourceId': 'artifacts', 'reason': 'Verified stopped reader'})
        refused = attempt(c, 'resource.dispose', {'resourceId': 'artifacts', 'disposition': 'remove', 'reason': 'Cannot delete held-only group'})
        assert not refused['ok'] and refused['code'] == 'RESOURCE_PENDING', refused
        c.request('resource.hold', {'resourceId': 'artifacts', 'enabled': False, 'reason': 'Review done', 'until': 'now'})
        c.request('resource.dispose', {'resourceId': 'artifacts', 'disposition': 'remove', 'reason': 'Known owned unchanged fixture'})
        assert not group.exists(), 'qualified owner cleanup failed'
        assert (foreign / 'keep').read_bytes() == b'FOREIGN-RESOURCE-CANARY'
        close(c)
        final = c.request('status')
        assert final['taskState'] == 'integrated' and final['cleanup'] == 'done' and final['resourceSummary']['pending'] == 0 and final['resourceSummary']['activeUses'] == 0, final
        print('MAINTENANCE_NATIVE_OK selected byte/age indicators, hold/use protection, bounds, external refusal, live-state protection and owned cleanup', flush=True)


if __name__ == '__main__':
    p = argparse.ArgumentParser()
    p.add_argument('--binary', required=True)
    p.add_argument('--controls', action='store_true')
    args = p.parse_args()
    check(str(Path(args.binary).resolve(strict=True)), args.controls)
