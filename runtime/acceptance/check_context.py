"""Independent native continuation consumer; fictional explicitly selected roots only."""
import argparse
from contextlib import closing
import hashlib
import json
import os
from pathlib import Path
import sqlite3
import subprocess
import sys
import uuid

import check_cli as cli

sys.path.insert(0, str(Path(__file__).resolve().parents[2]/'scripts/acceptance'))
from check_ready_kit import owned_fixture


def sha(raw):
    return hashlib.sha256(raw).hexdigest()


def snapshot(root):
    return {p.relative_to(root).as_posix(): sha(p.read_bytes())
            for p in root.rglob('*') if p.is_file() and not p.is_symlink()}


def call(client, payload=None, code=None, revision=None):
    req = dict(schemaVersion=1, action='context.inspect', projectRoot=str(client.root),
               actor='main', requestId=str(uuid.uuid4()),
               expectedRevision=client.revision if revision is None else revision,
               payload={} if payload is None else payload)
    p = subprocess.run([str(client.binary), 'request'], input=json.dumps(req),
                       capture_output=True, text=True, encoding='utf-8', timeout=30)
    r = cli.decode(p)
    if code is not None:
        assert r['code'] == code, (code, r)
    if not r['ok'] and r['code'].startswith('CONTEXT_'):
        assert all(isinstance(r['data'].get(k), str) and r['data'][k]
                   for k in ('reason', 'stage', 'nextAction')), ('missing context diagnosis', r)
    return r


def setup(binary, base):
    c = cli.scenario(binary, base/'project Ω with spaces')
    (c.root/'src').mkdir()
    (c.root/'docs').mkdir()
    (c.root/'src/main.go').write_text('package fixture\nfunc compute() int { return 1 }\n', encoding='utf-8')
    (c.root/'src/caller.go').write_text('package fixture\nfunc caller() int { return compute() }\n', encoding='utf-8')
    (c.root/'docs/development.md').write_text('# Shared technical source\nПроверенный вымышленный пример.\n', encoding='utf-8')
    (c.root/'UNSELECTED.txt').write_text('UNSELECTED-CANARY-CONTENT', encoding='utf-8')
    c.request('stop', dict(enabled=True))
    return c


def exercise_windows_and_boundaries(binary, base):
    c = setup(binary, base)
    filler = ''.join('// Unrelated fictional source line %d\n' % n for n in range(1200))
    large = 'package fixture\n'+filler+'func compute() int { return 7 }\nfunc caller() int { return compute() }\n'
    (c.root/'src/large.go').write_text(large, encoding='utf-8')
    payload = dict(mode='selected', files=['src/large.go'], terms=['compute'], budgetBytes=2048)
    result = call(c, payload, 'CONTEXT_SELECTED')
    d = result['data']; snippets = [x for x in d['selected'] if x['path'] == 'src/large.go']
    assert result['ok'] and snippets and all(x['startLine'] > 1 for x in snippets), d
    assert all(x['kind'] in ('definition-candidate', 'reference-candidate') for x in snippets), d
    assert any('func compute' in x['text'] for x in snippets) and any('return compute()' in x['text'] for x in snippets), d
    assert d['metrics']['snippetBytes'] <= 2048 and len(large.encode()) > 30000, d
    # A machine-fingerprinted immutable source is not automatically a demand to
    # feed the entire file to the model. Explicit whole originals still win.
    with closing(sqlite3.connect(c.root/'.ocmi/local/state.sqlite')) as db:
        command = json.loads(db.execute('SELECT command_json FROM task WHERE id=?', ('task-1',)).fetchone()[0])
    c.request('task.revise', dict(taskId='task-1', outcomeId='outcome-1', taskRevision=2,
        completionScope='target', worker='worker-1', reviewer='main', contract='contract.md',
        inputs=['input.txt', 'src/large.go'], command=command, reason='Fictional immutable source binding.'))
    c.request('task.ready')
    bound = call(c, payload, 'CONTEXT_SELECTED')
    assert bound['ok'] and bound['data']['inputFreshness'] == 'verified', bound
    assert any(x['path'] == 'src/large.go' for x in bound['data']['selected']), bound
    whole = call(c, dict(payload, requiredFiles=['src/large.go']), 'CONTEXT_INCOMPLETE')
    assert any(x['path'] == 'src/large.go' and x['kind'] == 'required' for x in whole['data']['omitted']), whole
    (c.root/'src/substring.go').write_text('package fixture\nfunc precompute() int { return 0 }\n', encoding='utf-8')
    no_match = call(c, dict(payload, files=['src/substring.go']), 'CONTEXT_INCOMPLETE')
    assert not any(x['path'] == 'src/substring.go' for x in no_match['data']['selected']), no_match
    (c.root/'.cache').mkdir()
    (c.root/'.cache/hidden.go').write_text('package fixture\nfunc compute() {} // PRIVATE-CONTEXT-CANARY\n', encoding='utf-8')
    (c.root/'src/certificate.cer').write_text('PRIVATE-CONTEXT-CANARY compute', encoding='utf-8')
    for path in ('.cache/hidden.go', 'src/certificate.cer'):
        refused = call(c, dict(payload, files=[path]), 'CONTEXT_SCOPE_REFUSED')
        assert 'PRIVATE-CONTEXT-CANARY' not in json.dumps(refused) and not refused['data'].get('selected'), refused
    missing = call(c, dict(payload, requiredFiles=['docs/missing.md']), 'CONTEXT_INCOMPLETE')
    assert missing['data']['notices'] or missing['data']['omitted'], missing
    (c.root/'docs/oversized.md').write_text('x'*(128*1024+1), encoding='utf-8')
    limited = call(c, dict(payload, requiredFiles=['docs/oversized.md']), 'CONTEXT_INCOMPLETE')
    assert limited['data']['notices'] or limited['data']['omitted'], limited
    many = 'package fixture\n'+''.join('func f%d() {}\n' % n for n in range(520))+'func compute() {}\n'
    (c.root/'src/many.go').write_text(many, encoding='utf-8')
    limited = call(c, dict(payload, files=['src/many.go']), 'CONTEXT_INCOMPLETE')
    assert any(any(term in x['reason'].lower() for term in ('outline', 'symbol'))
               for x in limited['data']['notices']), limited
    print('CONTEXT_WINDOWS_OK large source definition/reference windows, literal terms, hidden/certificate scope, missing/size/symbol notices')


def exercise_remaining_review(binary, base):
    c = setup(binary, base)
    refs = 'package fixture\n'+''.join('func caller%d() { compute() }\n' % n for n in range(60))
    (c.root/'src/references.go').write_text(refs, encoding='utf-8')
    (c.root/'src/definition.go').write_text('package fixture\nfunc compute() int { return 1 }\n', encoding='utf-8')
    payload = dict(mode='selected', files=['src/references.go', 'src/definition.go'], terms=['compute'], budgetBytes=768)
    ranked = call(c, payload)
    assert ranked['code'] in ('CONTEXT_SELECTED', 'CONTEXT_INCOMPLETE'), ranked
    assert any(x['path'] == 'src/definition.go' and x['kind'] == 'definition-candidate'
               for x in ranked['data']['selected']), 'references exhausted budget before the required definition priority'
    dense = 'package fixture\n'+''.join('func f%d(){ compute() }\n' % n for n in range(650))+'func compute(){}\n'
    (c.root/'src/dense.go').write_text(dense, encoding='utf-8')
    limited = call(c, dict(payload, files=['src/dense.go']), 'CONTEXT_INCOMPLETE')
    windows = sum(x['kind'].endswith('-candidate') for x in limited['data']['selected'])
    windows += sum(x['kind'] == 'candidate' for x in limited['data']['omitted'])
    assert windows <= 512, 'window cap applies before collecting/returning every omitted reference'
    assert any('window' in x['reason'].lower() or ('candidate' in x['reason'].lower() and 'limit' in x['reason'].lower())
               for x in limited['data']['notices']), 'reference-window truncation must be visible'
    (c.root/'src/unsupported.go').write_bytes(b'\xff\xfe\x00')
    unsupported = call(c, dict(payload, files=['src/unsupported.go']), 'CONTEXT_INCOMPLETE')
    assert unsupported['data']['omitted'] or unsupported['data']['notices'], unsupported
    assert not any(x['path'] == 'src/unsupported.go' for x in unsupported['data']['selected']), unsupported
    # These are valid stored identities in the existing core. The derived view
    # must refuse oversized metadata instead of dumping or truncating identities.
    long_root = base/'long-identity'; long_root.mkdir()
    long_client = cli.Client(binary, long_root)
    long_id = 'p'*131072
    long_client.request('init', dict(projectId=long_id, deviceId='device', copyId='copy', owner='main'))
    long_client.bind(); long_client.request('task.ready')
    summary = call(long_client)
    assert len(json.dumps(summary).encode()) <= 65536, 'unbounded derived summary metadata'
    if summary['ok']:
        assert summary['data']['provenance']['projectId'] == long_id, 'identity was silently truncated'
    normal_root = base/'normal-manifest'
    normal = cli.scenario(binary, normal_root)
    names = []
    for n in range(15):
        name = 'context-input-'+'a'*65+'-%02d.md' % n
        names.append(name); (normal.root/name).write_text('fixture\n', encoding='utf-8')
    with closing(sqlite3.connect(normal.root/'.ocmi/local/state.sqlite')) as db:
        command = json.loads(db.execute('SELECT command_json FROM task WHERE id=?', ('task-1',)).fetchone()[0])
    normal.request('task.revise', dict(taskId='task-1', outcomeId='outcome-1', taskRevision=2,
        completionScope='target', worker='worker-1', reviewer='main', contract='contract.md',
        inputs=['input.txt']+names, command=command, reason='Fictional normal multi-input task.'))
    normal.request('task.ready')
    valid = call(normal, code='CONTEXT_SELECTED')
    assert valid['ok'] and len(valid['data']['assignment']['requiredInputs']) == 17, valid['code']
    print('CONTEXT_REVIEW_OK global definition priority, unsupported text diagnostics and bounded identity metadata')


def validate_summary(r, state):
    assert r['ok'] and r['code'] == 'CONTEXT_SELECTED', r
    d = r['data']
    assert d['formatVersion'] == 1 and d['mode'] == 'summary', d
    assert d['state'] == state and d['readiness'] == 'not-evaluated', d
    assert d['inputFreshness'] == 'not-read' and d['metrics']['filesRead'] == 0, d
    assert d['metrics']['tokens'] == 'unknown', d
    assert all(d[k] == [] for k in ('files', 'selected', 'outline', 'omitted', 'notices')), d
    assert d['assignment']['worker'] == 'worker-1' and d['assignment']['reviewer'] == 'main', d
    assert d['assignment']['contractPath'] == 'contract.md', d
    expected = [('contract.md', None), ('input.txt', None)]
    assert [x['path'] for x in d['assignment']['requiredInputs']] == [x[0] for x in expected], d
    p = d['provenance']
    assert p['rootKind'] == 'project' and p['rootId'] and p['projectId'] == 'synthetic-project', d
    assert p['deviceId'] == 'device-1' and p['copyId'] == 'copy-1', d
    assert p['taskId'] == state['taskId'] and p['taskRevision'] == state['taskRevision'], d
    assert p['revision'] == r['revision'] and len(p['statusFingerprint']) == 64, d
    assert isinstance(d['summary'], str) and d['summary'] and len(d['summary'].encode()) <= 4096, d
    assert 'UNSELECTED-CANARY-CONTENT' not in json.dumps(r), r
    return d


def qualify(binary, base):
    c = setup(binary, base)
    state = c.request('status')
    assert state['stopped'] is True and state['taskId'] == 'task-1' and state['taskState'] == 'ready', state
    before = snapshot(c.root)
    assert c.request('status') == state and snapshot(c.root) == before
    p = c.root/'input.txt'
    original = p.read_bytes()
    p.write_bytes(b'wrong-copy-over')
    assert snapshot(c.root) != before, 'preservation oracle accepted targeted wrong behavior'
    p.write_bytes(original)
    assert snapshot(c.root) == before
    print('CONTEXT_CONTROL_OK actual native task/STOP fixture and source/state preservation oracle')


def exercise(binary, base):
    c = setup(binary, base)
    state = c.request('status')
    before = snapshot(c.root)
    validate_summary(call(c), state)
    assert snapshot(c.root) == before, 'summary wrote selected root'
    args = dict(mode='selected', files=['src/main.go', 'src/caller.go'],
                requiredFiles=['docs/development.md'], terms=['compute'], budgetBytes=16000)
    r = call(c, args, 'CONTEXT_SELECTED')
    assert r['ok'] and r['data']['inputFreshness'] == 'verified', r
    d = r['data']
    assert d['state'] == state and d['cache']['storage'] == 'session-memory-only', d
    assert d['cache']['reused'] == 0, 'fresh CLI imported a previous index'
    assert {x['path'] for x in d['files']} == {'contract.md', 'input.txt', 'docs/development.md', 'src/main.go', 'src/caller.go'}, d
    for path in ('contract.md', 'input.txt', 'docs/development.md'):
        item = next(x for x in d['selected'] if x['path'] == path and x['kind'] == 'required')
        assert item['text'] == (c.root/path).read_text(encoding='utf-8') and item['sha256'] == sha((c.root/path).read_bytes()), item
    assert any(x['path'] == 'src/main.go' and x['name'] == 'compute' for x in d['outline']), d
    assert any(x['path'] == 'src/caller.go' for x in d['selected']), d
    assert d['metrics']['snippetBytes'] == sum(len(x['text'].encode('utf-8')) for x in d['selected']), d
    assert snapshot(c.root) == before, 'retrieval wrote selected root'
    p = c.root/'src/main.go'; original = p.read_bytes(); stamp = p.stat()
    p.write_bytes(original.replace(b'return 1', b'return 2')); os.utime(p, ns=(stamp.st_atime_ns, stamp.st_mtime_ns))
    changed = call(c, args, 'CONTEXT_SELECTED')['data']
    assert next(x for x in changed['files'] if x['path'] == 'src/main.go')['sha256'] == sha(p.read_bytes())
    assert any('return 2' in x['text'] for x in changed['selected']), changed
    p.write_bytes(original)
    budget = call(c, dict(args, budgetBytes=128), 'CONTEXT_INCOMPLETE')
    assert not budget['ok'] and budget['data']['omitted'] and budget['data']['metrics']['snippetBytes'] <= 128, budget
    assert all(x['text'] == (c.root/x['path']).read_text(encoding='utf-8') for x in budget['data']['selected'] if x['kind'] == 'required')
    bad = c.root/'input.txt'; original = bad.read_bytes(); bad.write_bytes(b'changed!')
    stale = call(c, args, 'CONTEXT_INPUTS_CHANGED')
    assert not stale['ok'] and not stale['data'].get('selected'), stale
    bad.write_bytes(original)
    for payload in ({'unknown': True}, {'mode': 'summary', 'files': ['src/main.go']},
                    dict(args, budgetBytes=0), dict(args, terms=['bad.*']), dict(args, files=None)):
        rejected = call(c, payload, 'INVALID_PAYLOAD'); assert not rejected['ok'], rejected
    for path in ('../foreign.md', str(base/'foreign.md'), '.ocmi/local/state.sqlite', '.git/config', 'src/../main.go', 'src/alias.go.', 'src//main.go'):
        rejected = call(c, dict(args, files=[path]), 'CONTEXT_SCOPE_REFUSED')
        assert not rejected['ok'] and not rejected['data'].get('selected'), rejected
    stale = call(c, {}, 'STALE_REVISION', revision=c.revision+100)
    assert not stale['ok'], stale
    assert snapshot(c.root) == before, 'refusal or retrieval changed original root'
    empty = base/'empty'; empty.mkdir()
    e = cli.Client(binary, empty)
    e.request('init', dict(projectId='empty', deviceId='empty-device', copyId='empty-copy', owner='main'))
    assert not call(e, code='TASK_UNBOUND')['ok']
    print('CONTEXT_NATIVE_OK summary/assignment/STOP, selected definitions/references/shared docs, UTF-8 budget, fresh WIP, binding drift, strict scope and no writes')
    boundary_scope = base/'bounded-windows'; boundary_scope.mkdir()
    exercise_windows_and_boundaries(binary, boundary_scope)
    residual_scope = base/'review-boundaries'; residual_scope.mkdir()
    exercise_remaining_review(binary, residual_scope)


if __name__ == '__main__':
    p = argparse.ArgumentParser()
    p.add_argument('--binary', type=Path, required=True)
    p.add_argument('--scratch-root', type=Path)
    p.add_argument('--qualify-control', action='store_true')
    args = p.parse_args()
    with owned_fixture(args.scratch_root, 'ocmi-context-', 'Fictional native continuation and selected source qualification') as root:
        (qualify if args.qualify_control else exercise)(args.binary.resolve(), root)
