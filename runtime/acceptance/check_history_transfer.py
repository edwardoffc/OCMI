"""Whole ledger/policy transfer and real v1 package compatibility through the CLI."""
import argparse
import pathlib
import tempfile

from check_cli import Client
from check_lifecycle import binding, revise, setup


def imported(binary, root, package):
    root.mkdir()
    c=Client(binary,root)
    c.request('handover.import',dict(package=package,deviceId='other-device',copyId='other-copy'))
    return c


def check(binary, old_binary):
    with tempfile.TemporaryDirectory(prefix='ocmi-history-transfer-') as tmp:
        base=pathlib.Path(tmp)
        c=setup(binary,base/'ledger')
        c.request('run',actor='worker-1');c.request('submit',actor='worker-1')
        c.request('review',dict(accept=False,reason='Retained first review.'))
        revise(c,2);c.request('task.ready');c.request('run',actor='worker-1')
        c.request('submit',actor='worker-1');c.request('review',dict(accept=True,reason='Retained accepted review.'))
        c.request('integrate',dict(candidate='candidate-A2',reason='Retained integration.'))
        c.request('resource.close',dict(disposition='keep',reason='Retained resource.'))
        c.request('task.bind',binding(c.root,task='B',outcome='outcome-B'))
        before=c.request('task.history')
        package=c.request('handover.export',dict(transferId='full-ledger',files=['contract.md','input.txt']))['package']
        dest=imported(binary,base/'destination',package)
        after=dest.request('task.history')
        for key,identity in [('tasks',lambda r:r['taskId']),('revisions',lambda r:(r['taskId'],r['taskRevision'])),
                             ('operations',lambda r:r['operationId'])]:
            assert {identity(r) for r in before[key]}=={identity(r) for r in after[key]},key
        for key in ('reviews','integrations','resources'):
            assert len(after[key])==len(before[key]),key
        assert any(r.get('reason')=='Retained first review.' for r in after['reviews'])
        assert dest.request('status')['stopped'] is True
        refused=base/'old-reader';refused.mkdir()
        Client(old_binary,refused).request('handover.import',dict(package=package,deviceId='old',copyId='old'),ok=False)
        assert list(refused.iterdir())==[], 'v1 reader wrote newer package'
        print('PASS: full task/revision/decision ledger transfer and explicit old-reader refusal')

        c=setup(binary,base/'blocked-policy',code=7)
        c.request('run',actor='worker-1');revise(c,2,7);c.request('task.ready');c.request('run',actor='worker-1')
        package=c.request('handover.export',dict(transferId='blocked-outcome',files=['contract.md','input.txt']))['package']
        dest=imported(binary,base/'blocked-destination',package)
        policy=next(o for o in dest.request('task.history')['outcomes'] if o['outcomeId']=='outcome-A')
        assert policy['failureCount']==2,policy
        dest.request('stop',dict(enabled=False))
        dest.request('handover.rebind',dict(command=binding(dest.root,code=7)['command']))
        dest.request('task.ready');dest.request('run',actor='worker-1',ok=False)
        assert not (dest.root/'effect.txt').exists()
        print('PASS: transfer/rebind cannot reset failed-outcome admission')

        old=setup(old_binary,base/'actual-v1-package')
        receipt=old.request('run',actor='worker-1')
        package=old.request('handover.export',dict(transferId='legacy-package',files=['contract.md','input.txt']))['package']
        dest=imported(binary,base/'legacy-import',package)
        history=dest.request('task.history')
        assert any(o['operationId']==receipt['operationId'] for o in history['operations'])
        assert dest.request('status')['stopped'] is True
        print('PASS: actual old-binary v1 export imported into v2 quarantine without losing history')


if __name__=='__main__':
    parser=argparse.ArgumentParser();parser.add_argument('--binary',required=True);parser.add_argument('--old-binary',required=True)
    args=parser.parse_args();check(str(pathlib.Path(args.binary).resolve(strict=True)),str(pathlib.Path(args.old_binary).resolve(strict=True)))
