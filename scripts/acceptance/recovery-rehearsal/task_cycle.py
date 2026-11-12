"""Protected rehearsal admission/acceptance through the published CLI."""
import argparse
import base64
import hashlib
import json
import os
import pathlib
import subprocess
import sys
import uuid

p = argparse.ArgumentParser()
p.add_argument('--binary', required=True)
p.add_argument('--project', required=True)
p.add_argument('--phase', choices=['bind','resume-bind','finish-bind','accept'], required=True)
p.add_argument('--report', required=True)
a = p.parse_args()
root = pathlib.Path(a.project).resolve()
binary = str(pathlib.Path(a.binary).resolve())
report = pathlib.Path(a.report)
events = []
revision = 0

def call(action, payload=None, actor='main', ok=True, key=None, rev=None):
    global revision
    request = dict(schemaVersion=1, action=action, projectRoot=str(root), actor=actor,
                   requestId=key or str(uuid.uuid4()), expectedRevision=revision if rev is None else rev,
                   payload=payload or {})
    proc = subprocess.run([binary,'request'], input=json.dumps(request), text=True,
                          encoding='utf-8', capture_output=True, timeout=40)
    data = json.loads(proc.stdout)
    events.append(dict(action=action,request=request,result=data,exitCode=proc.returncode,stderr=proc.stderr))
    report.write_text(json.dumps(events,indent=2),encoding='utf-8')
    assert data['ok'] is ok and proc.returncode == (0 if ok else 2), data
    revision = max(revision,data['revision'])
    return data['data']

def intent(rid, kind, mode, locator, disposition):
    if mode=='use': locator=str((root/locator).resolve())
    call('resource.intent',dict(resourceId=rid,provider='local',kind=kind,mode=mode,
        locator=locator,purpose='Fictional restored component rehearsal',lifetime='through acceptance',
        disposition=disposition,ownerRef='rehearsal-main'))
    call('resource.apply',dict(resourceId=rid))

if a.phase in ('bind','resume-bind','finish-bind'):
    assert (root/'labels.py').is_file() and (root/'acceptance/verify.py').is_file()
    # Fresh local setup, not restored user configuration: keep the new live store out of Git.
    if (root/'.git').is_dir():
        exclude=root/'.git/info/exclude'
        exclude.parent.mkdir(exist_ok=True)
        prior=exclude.read_text(encoding='utf-8') if exclude.exists() else ''
        if '.ocmi/' not in prior.splitlines():
            exclude.write_text(prior+'\n.ocmi/\n',encoding='utf-8')
        ignored=subprocess.run(['git','-C',str(root),'check-ignore','-q','--','.ocmi/local/state.sqlite'],capture_output=True,timeout=10)
        assert ignored.returncode==0,'new private state would enter Git'
    (root/'rehearsal-contract.md').write_text('Normalize labels: strip, Unicode casefold, unique, sorted; drop blank; preserve input. Main owns acceptance/verify.py. Only labels.py may change.\n',encoding='utf-8')
    if a.phase=='bind':
        call('init',dict(projectId='restored-example',deviceId='new-device',copyId='restored-copy',owner='main'))
        call('task.bind',dict(taskId='normalize-labels',outcomeId='normalized-labels',taskRevision=1,
            completionScope='target',worker='worker-1',reviewer='main',contract='rehearsal-contract.md',
            inputs=['acceptance/verify.py','scenario.json'],command=dict(id='verify-restored-result',
            executable=sys.executable,args=['-B','acceptance/verify.py','.'],cwd='.',timeoutMs=5000,outputLimitBytes=8192)))
        call('task.ready')
    else:
        current=call('status')
        assert current['taskId']=='normalize-labels' and current['taskState']=='ready',current
    if a.phase!='finish-bind':
        intent('source-use','file','use','labels.py','release')
    else:
        existing=call('resource.inspect',dict(resourceId='source-use'))
        assert existing['ownership']=='external' and existing['observedState']=='external' and existing['nativeIdentity'],existing
    call('resource.use',dict(resourceId='source-use',access='read',reason='Read restored component; patch authority is the addressed task, not ownership or deletion rights.'),actor='worker-1')
    intent('owned-scratch','directory','create','task-scratch','remove')
    call('resource.write',dict(resourceId='owned-scratch',path='temporary.txt',contentBase64=base64.b64encode(b'fictional disposable output\n').decode()),actor='worker-1')
    call('stop',dict(enabled=True))
    call('run',actor='worker-1',ok=False)
    assert not list((root/'.ocmi/local/runs').glob('*')) if (root/'.ocmi/local/runs').exists() else True
    print('TASK_BOUND_STOP_GUARD_PASS')
else:
    state=call('status')
    assert state['stopped'] is True, state
    call('stop',dict(enabled=False))
    key=str(uuid.uuid4())
    run=call('run',actor='worker-1',key=key)
    assert run['operationState']=='succeeded' and run['exitCode']==0,run
    replay=call('run',actor='worker-1',key=key,rev=0)
    assert replay['operationId']==run['operationId']
    call('submit',actor='worker-1')
    call('review',dict(accept=True,reason='Independent protected Unicode/duplicate/idempotence/nonmutation consumer passed.'))
    sha=hashlib.sha256((root/'labels.py').read_bytes()).hexdigest()
    call('integrate',dict(candidate='labels-sha256-'+sha,reason='Exact restored component independently accepted.'))
    call('resource.release',dict(resourceId='source-use',reason='Worker stopped using preserved source.'),actor='worker-1')
    call('resource.dispose',dict(resourceId='source-use',disposition='remove',reason='Use cannot grant deletion.'),ok=False)
    assert (root/'labels.py').is_file()
    call('resource.dispose',dict(resourceId='owned-scratch',disposition='remove',reason='Verified owned scratch no longer needed.'))
    assert not (root/'task-scratch').exists()
    rows=call('resource.list',dict(limit=100))['resources']
    assert any(r['kind']=='process' and r['observedState']=='stopped' for r in rows),rows
    assert not any(r['kind']=='process' and r['observedState']=='running' for r in rows),rows
    assert not any(p.name=='tmp' and p.is_dir() for p in (root/'.ocmi/local/runs').glob('*/tmp'))
    call('resource.close',dict(disposition='keep',reason='Closed synthetic project and exact acceptance evidence retained.'))
    final=call('status')
    assert final['taskState']=='integrated' and final['cleanup']=='done',final
    print('TASK_ACCEPTED_INTEGRATED_CLEANUP_PASS')
