"""Independent real-CLI transfer controls. Fixtures are fictional, never user roots."""
import argparse
import contextlib
import copy
import json
import pathlib
import sqlite3
import sys
import tempfile

from check_cli import Client, scenario


def rows(root, query):
    with contextlib.closing(sqlite3.connect(root / '.ocmi/local/state.sqlite')) as db:
        return db.execute(query).fetchall()


def check(binary):
    failures = []
    with tempfile.TemporaryDirectory(prefix='ocmi-portable-') as temp:
        base = pathlib.Path(temp)

        def case(name, callback):
            try:
                callback()
                print('PASS:', name)
            except (AssertionError, KeyError) as exc:
                failures.append(name)
                print('FAIL:', name, str(exc)[:350])

        source = scenario(binary, base / 'source')
        receipt = source.request('run', actor='worker-1')
        source.request('submit', actor='worker-1')
        source.request('review', {'accept': True, 'reason': 'Synthetic complete review.'})
        source.request('integrate', {'candidate': 'synthetic-v1', 'reason': 'Synthetic integration.'})
        source.request('resource.close', {'disposition': 'keep', 'reason': 'Synthetic evidence.'})
        package = source.request('handover.export', {'transferId': 'transfer-1', 'files': ['contract.md', 'input.txt']})['package']

        def tampering():
            bad = copy.deepcopy(package)
            bad['source']['projectId'] = 'changed-project'
            root = base / 'tampered'; root.mkdir()
            Client(binary, root).request('handover.import', {'package': bad, 'deviceId': 'laptop', 'copyId': 'copy'}, ok=False)
            assert list(root.iterdir()) == []
        case('metadata checksum rejects mutation before writing', tampering)

        def nonempty():
            root = base / 'unrelated'; root.mkdir()
            (root / 'unrelated.txt').write_text('keep')
            Client(binary, root).request('handover.import', {'package': package, 'deviceId': 'laptop', 'copyId': 'copy'}, ok=False)
            assert sorted(p.name for p in root.iterdir()) == ['unrelated.txt']
            assert (root / 'unrelated.txt').read_text() == 'keep'
        case('foreign destination preserved', nonempty)

        root = base / 'destination'; root.mkdir()
        dest = Client(binary, root)
        payload = {'package': package, 'deviceId': 'laptop', 'copyId': 'copy'}
        dest.request('handover.import', payload, key='original-import')

        def history():
            assert any(row[0] == receipt['operationId'] for row in rows(root, 'SELECT id FROM operation'))
            assert rows(root, 'SELECT reason FROM review_record')
            assert rows(root, 'SELECT candidate FROM integration_record')
            assert rows(root, 'SELECT disposition FROM resource')
            assert package['originalTaskState'] == 'integrated'
            assert package['outputOmitted'] is True
            assert not any(row[0] for row in rows(root, 'SELECT output FROM operation'))
        case('IDs and full decision/resource history preserved, output explicitly omitted', history)
        case('completed identical import is idempotent', lambda: dest.request('handover.import', payload, key='original-import'))

        def continuation():
            state = dest.request('status')
            assert state['stopped'] is True and state['taskState'] == 'draft', state
            dest.request('run', actor='worker-1', ok=False)
            dest.request('stop', {'enabled': False})
            dest.request('task.ready', ok=False)
            dest.request('handover.rebind', {'command': dict(id='new-device-check', executable=sys.executable,
                args=[str(pathlib.Path(__file__).with_name('check_cli.py').resolve()), '--fixture', str(root), '0'],
                cwd='.', timeoutMs=5000, outputLimitBytes=8192)})
            dest.request('task.ready')
            fresh = dest.request('run', actor='worker-1')
            assert fresh['operationId'] != receipt['operationId'] and fresh['exitCode'] == 0
            assert (root/'effect.txt').read_text() == 'once\n'
        case('destination needs explicit rebind and fresh real execution', continuation)

        def freeze():
            source.request('handover.ack', {'transferId': package['transferId'], 'generation': package['generation']})
            # Rebinding or clearing STOP must never reactivate the departed source.
            source.request('handover.rebind', {'command': package['task']['Command']}, ok=False)
            source.request('stop', {'enabled': False}, ok=False)
        case('acknowledged source cannot be reactivated by ordinary transitions', freeze)

        def unknown():
            c = scenario(binary, base / 'unknown')
            # Model a durable prepared intent after supervisor loss, using the real schema.
            with contextlib.closing(sqlite3.connect(c.root/'.ocmi/local/state.sqlite')) as db:
                db.execute("INSERT INTO operation(id,task_id,state,output,truncated,before_fingerprint,after_fingerprint,created_at) VALUES('lost','task-1','prepared','',0,'x','','2099-01-01')")
                db.execute("UPDATE task SET state='in_progress'")
                db.commit()
            state = c.request('status')
            assert state['operationState'] == 'unknown', state
            c.request('operation.reconcile', {'operationId': 'lost', 'disposition': 'abandon', 'reason': 'Reason alone proves nothing.'}, ok=False)
            c.request('handover.export', {'transferId': 'unsafe', 'files': ['contract.md', 'input.txt']}, ok=False)
            c.request('task.bind', {'taskId': 'replacement'}, ok=False)
            assert rows(c.root, "SELECT state FROM operation WHERE id='lost'") == [('prepared',)]
        case('unknown execution cannot be laundered into transferable evidence', unknown)

        def large():
            c = scenario(binary, base / 'large')
            # Exercise the actual base64 JSON boundary above the former 1 MiB transport limit.
            (c.root/'selected.txt').write_bytes(b'x' * (900 * 1024))
            pkg = c.request('handover.export', {'transferId': 'large', 'files': ['contract.md', 'input.txt', 'selected.txt']})['package']
            target = base/'large destination'; target.mkdir()
            Client(binary, target).request('handover.import', {'package': pkg, 'deviceId': 'other', 'copyId': 'other'})
            assert (target/'selected.txt').read_bytes() == b'x'*(900*1024)
        case('supported transfer fits actual CLI transport', large)
    if failures:
        raise SystemExit('Transfer acceptance failures: '+', '.join(failures))


if __name__ == '__main__':
    parser = argparse.ArgumentParser()
    parser.add_argument('--binary', required=True)
    args = parser.parse_args()
    check(str(pathlib.Path(args.binary).resolve(strict=True)))
