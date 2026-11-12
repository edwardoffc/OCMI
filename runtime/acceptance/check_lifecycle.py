"""Independent CLI controls for sequential tasks, revisions and outcome retry admission."""
import argparse
import pathlib
import sys
import tempfile

from check_cli import Client


def binding(root, task='A', outcome='outcome-A', revision=1, code=0, scope="target"):
    return dict(taskId=task, outcomeId=outcome, taskRevision=revision, completionScope=scope, worker='worker-1', reviewer='main',
        contract='contract.md', inputs=['input.txt'], command=dict(id='check', executable=sys.executable,
        args=[str(pathlib.Path(__file__).with_name('check_cli.py').resolve()), '--fixture', str(root), str(code)],
        cwd='.', timeoutMs=5000, outputLimitBytes=8192))


def setup(binary, root, code=0):
    root.mkdir()
    (root/'contract.md').write_text('Fictional contract version one.',encoding='utf-8')
    (root/'input.txt').write_text('fixed',encoding='utf-8')
    c=Client(binary,root)
    c.request('init',dict(projectId='lifecycle',deviceId='device',copyId='copy',owner='main'))
    c.request('task.bind',binding(root,code=code))
    c.request('task.ready')
    return c


def revise(c, revision, code=0):
    p=binding(c.root,revision=revision,code=code)
    p['reason']='Explicit revision preserves the same outcome.'
    return c.request('task.revise',p)


def check(binary):
    with tempfile.TemporaryDirectory(prefix='ocmi-lifecycle-') as tmp:
        base=pathlib.Path(tmp)
        c=setup(binary,base/'sequential')
        c.request('task.bind',binding(c.root,task='B',outcome='outcome-B'),ok=False)
        first=c.request('run',actor='worker-1')
        c.request('submit',actor='worker-1')
        c.request('review',dict(accept=False,reason='One explicit fictional requirement changes.'))
        (c.root/'contract.md').write_text('Fictional contract version two.',encoding='utf-8')
        p=binding(c.root,revision=2);p['reason']='Changed fictional contract.'
        c.request('task.revise',p,key='revision-two')
        c.request('task.revise',p,key='revision-two',revision=0)
        c.request('task.ready')
        c.request('submit',actor='worker-1',ok=False)
        second=c.request('run',actor='worker-1')
        assert second['operationId']!=first['operationId']
        c.request('submit',actor='worker-1')
        c.request('review',dict(accept=True,reason='Independent fictional review.'))
        c.request('integrate',dict(candidate='synthetic-v2',reason='Verified fictional candidate.'))
        c.request('task.bind',binding(c.root,task='B',outcome='outcome-B'),ok=False)
        c.request('resource.close',dict(disposition='keep',reason='Retain fictional task history.'))
        c.request('task.bind',binding(c.root,task='B',outcome='outcome-B'))
        state=c.request('status')
        assert state['taskId']=='B' and state['taskState']=='draft' and state['cleanup']=='pending',state
        assert state.get('operationId') not in (first['operationId'],second['operationId']),state
        history=c.request('task.history')
        assert {t['taskId'] for t in history['tasks']}=={'A','B'},history
        revs=[r for r in history['revisions'] if r['taskId']=='A']
        assert sorted(r['taskRevision'] for r in revs)==[1,2],revs
        assert any('one' in r['contractSnapshot'] for r in revs if r['taskRevision']==1),revs
        assert any(o['operationId']==first['operationId'] and o['taskRevision']==1 for o in history['operations'])
        assert any(o['operationId']==second['operationId'] and o['taskRevision']==2 for o in history['operations'])
        print('PASS: sequential tasks, immutable contract revisions, scoped evidence and retained decisions')

        c=setup(binary,base/'retry-policy',code=7)
        for revision in (1,2):
            if revision>1:
                revise(c,revision,7);c.request('task.ready')
            assert c.request('run',actor='worker-1')['operationState']=='failed'
        revise(c,3,7);c.request('task.ready')
        c.request('run',actor='worker-1',ok=False)
        assert (c.root/'effect.txt').read_text()=='once\nonce\n'
        c.request('outcome.reassess',dict(outcomeId='outcome-A',reason=''),ok=False)
        c.request('outcome.reassess',dict(outcomeId='outcome-A',reason='Two real failures.',
            changedApproach='A bounded changed diagnostic attempt.',nextEvidence='One actual command receipt.'))
        assert c.request('run',actor='worker-1')['operationState']=='failed'
        revise(c,4,7);c.request('task.ready')
        c.request('outcome.reassess',dict(outcomeId='outcome-A',reason='Repeated same permission.',
            changedApproach='New wording cannot reset the outcome.',nextEvidence='Same check.'),ok=False)
        c.request('run',actor='worker-1',ok=False)
        c.request('outcome.review',dict(outcomeId='outcome-A',decision='continue',findings='Bounded process review.',
            changedApproach='One reviewed discriminating attempt.',nextEvidence='One command receipt.'))
        assert c.request('run',actor='worker-1')['operationState']=='failed'
        revise(c,5,7);c.request('task.ready')
        c.request('run',actor='worker-1',ok=False)
        c.request('outcome.review',dict(outcomeId='outcome-A',decision='continue',findings='Repeated review.',
            changedApproach='Same outcome cannot gain endless attempts.',nextEvidence='Same check.'),ok=False)
        assert (c.root/'effect.txt').read_text()=='once\nonce\nonce\nonce\n'
        history=c.request('task.history')
        policy=next(p for p in history['outcomes'] if p['outcomeId']=='outcome-A')
        assert policy['failureCount']==4 and policy['gate']=='owner_decision_required',policy
        assert len(history['operations'])==4
        print('PASS: persistent outcome failure gate, one reassessment attempt and one process-review attempt')


if __name__=='__main__':
    parser=argparse.ArgumentParser();parser.add_argument('--binary',required=True)
    args=parser.parse_args();check(str(pathlib.Path(args.binary).resolve(strict=True)))
