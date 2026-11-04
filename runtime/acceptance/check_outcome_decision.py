"""Project owner can continue with new verified evidence, without resetting or endless replay."""
import argparse
import hashlib
import pathlib
import tempfile

from check_cli import Client
from check_lifecycle import binding, revise, setup


def blocked(binary,root):
    c=setup(binary,root,7)
    c.request('run',actor='worker-1');revise(c,2,7);c.request('task.ready');c.request('run',actor='worker-1')
    c.request('outcome.reassess',dict(outcomeId='outcome-A',reason='Two observed failures.',
        changedApproach='One changed diagnostic.',nextEvidence='One receipt.'))
    revise(c,3,7);c.request('task.ready');c.request('run',actor='worker-1')
    c.request('outcome.review',dict(outcomeId='outcome-A',decision='continue',findings='Reviewed repeated failure.',
        changedApproach='One discriminating attempt.',nextEvidence='One receipt.'))
    revise(c,4,7);c.request('task.ready');c.request('run',actor='worker-1')
    revise(c,5,7);c.request('task.ready')
    return c


def check(binary):
    with tempfile.TemporaryDirectory(prefix='ocmi-owner-decision-') as tmp:
        base=pathlib.Path(tmp);c=blocked(binary,base/'source')
        report=c.root/'diagnostic.md';report.write_text('A newly observed fictional cause.',encoding='utf-8')
        evidence=dict(path='diagnostic.md',sha256=hashlib.sha256(report.read_bytes()).hexdigest())
        decision=dict(outcomeId='outcome-A',decision='continue',basis='Project owner reviewed new local evidence.',
            changedApproach='One bounded new diagnostic.',nextEvidence='Actual command receipt.',evidence=[evidence])
        c.request('outcome.decide',decision,actor='worker-1',ok=False)
        bad={**decision,'evidence':[dict(path='diagnostic.md',sha256='0'*64)]}
        c.request('outcome.decide',bad,ok=False)
        bad={**decision,'evidence':[dict(path='../foreign.md',sha256=evidence['sha256'])]}
        c.request('outcome.decide',bad,ok=False)
        private=c.root/'.ocmi/local/state.sqlite'
        bad={**decision,'evidence':[dict(path='.ocmi/local/state.sqlite',sha256=hashlib.sha256(private.read_bytes()).hexdigest())]}
        c.request('outcome.decide',bad,ok=False)
        c.request('outcome.decide',decision,key='owner-continuation')
        assert c.request('run',actor='worker-1')['operationState']=='failed'
        revise(c,6,7);c.request('task.ready')
        c.request('outcome.decide',decision,ok=False)
        c.request('outcome.decide',decision,key='owner-continuation',revision=0)
        c.request('run',actor='worker-1',ok=False)
        history=c.request('task.history')
        assert next(p for p in history['outcomes'] if p['outcomeId']=='outcome-A')['failureCount']==5
        assert any(d['outcomeId']=='outcome-A' and d['decision']=='continue' and d['basis']==decision['basis'] for d in history['decisions'])
        assert (c.root/'effect.txt').read_text()=='once\n'*5
        print('PASS: bounded project-owner continuation requires current selected evidence; replay cannot replenish it')

        package=c.request('handover.export',dict(transferId='owner-proof',files=['contract.md','input.txt','diagnostic.md']))['package']
        root=base/'destination';root.mkdir();dest=Client(binary,root)
        dest.request('handover.import',dict(package=package,deviceId='other',copyId='other'))
        dest.request('stop',dict(enabled=False))
        dest.request('handover.rebind',dict(command=binding(root,revision=6,code=7)['command']))
        dest.request('task.ready')
        dest.request('outcome.decide',decision,ok=False)
        report=root/'diagnostic.md';report.write_text('Different verified fictional evidence after continuation.',encoding='utf-8')
        fresh={**decision,'evidence':[dict(path='diagnostic.md',sha256=hashlib.sha256(report.read_bytes()).hexdigest())]}
        dest.request('outcome.decide',fresh)
        assert dest.request('run',actor='worker-1')['operationState']=='failed'
        assert next(p for p in dest.request('task.history')['outcomes'] if p['outcomeId']=='outcome-A')['failureCount']==6
        print('PASS: evidence-grant history survives transfer; genuinely changed proof permits one local owner decision')


if __name__=='__main__':
    parser=argparse.ArgumentParser();parser.add_argument('--binary',required=True)
    args=parser.parse_args();check(str(pathlib.Path(args.binary).resolve(strict=True)))
