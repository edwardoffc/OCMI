"""Actual old-preview store -> explicit upgrade -> readable old backup, via public CLI."""
import argparse
import contextlib
import hashlib
import pathlib
import shutil
import sqlite3
import tempfile

from check_cli import Client
from check_lifecycle import setup


def digest(path):
    return hashlib.sha256(path.read_bytes()).hexdigest()


def check(binary, old_binary):
    with tempfile.TemporaryDirectory(prefix='ocmi-migration-') as tmp:
        base=pathlib.Path(tmp)
        old=setup(old_binary,base/'legacy')
        receipt=old.request('run',actor='worker-1',key='legacy-command')
        old.request('submit',actor='worker-1')
        old.request('review',dict(accept=False,reason='Original retained review.'))
        store=old.root/'.ocmi/local/state.sqlite'
        before=digest(store)
        new=Client(binary,old.root)
        new.request('status',ok=False)
        new.request('task.history',ok=False)
        assert digest(store)==before, 'read silently changed legacy state'
        new.request('state.upgrade',actor='worker-1',ok=False)
        new.request('state.upgrade',revision=old.revision-1,ok=False)
        upgraded=new.request('state.upgrade',key='explicit-upgrade',revision=old.revision)
        assert new.request('state.upgrade',key='explicit-upgrade',revision=0)==upgraded
        backup=pathlib.Path(upgraded['backupPath']).resolve(strict=True)
        assert backup.is_relative_to((old.root/'.ocmi/local').resolve()),backup
        assert backup!=store.resolve()
        with contextlib.closing(sqlite3.connect(store)) as db:
            assert db.execute('SELECT schema_version FROM meta').fetchone()==(4,)
        history=new.request('task.history')
        assert any(o['operationId']==receipt['operationId'] for o in history['operations'])
        assert any(r['reason']=='Original retained review.' for r in history['reviews'])
        assert next(o for o in history['outcomes'] if o['outcomeId']=='outcome-A')['failureCount']==1
        replay=new.request('run',actor='worker-1',key='legacy-command',revision=0)
        assert replay['operationId']==receipt['operationId']
        assert (old.root/'effect.txt').read_text()=='once\n'
        restored=base/'backup restore';(restored/'.ocmi/local').mkdir(parents=True)
        shutil.copyfile(backup,restored/'.ocmi/local/state.sqlite')
        status=Client(old_binary,restored).request('status')
        assert status['taskId']=='A' and status['taskState']=='draft',status
        before=digest(store)
        old.request('status',ok=False)
        assert digest(store)==before, 'old binary changed newer store'
        print('PASS: actual v1 upgrade, owner/CAS/replay, history/policy retention and old-readable consistent backup')

        bad=setup(old_binary,base/'unsupported')
        store=bad.root/'.ocmi/local/state.sqlite'
        with contextlib.closing(sqlite3.connect(store)) as db:
            db.execute('UPDATE meta SET schema_version=999');db.commit()
        before=digest(store)
        Client(binary,bad.root).request('state.upgrade',ok=False)
        assert digest(store)==before
        print('PASS: unsupported legacy schema preserved without reset')


if __name__=='__main__':
    parser=argparse.ArgumentParser();parser.add_argument('--binary',required=True);parser.add_argument('--old-binary',required=True)
    args=parser.parse_args();check(str(pathlib.Path(args.binary).resolve(strict=True)),str(pathlib.Path(args.old_binary).resolve(strict=True)))
