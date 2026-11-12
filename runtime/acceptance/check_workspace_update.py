"""Independent native retained-workspace reader/update/rollback qualification."""
import argparse
import hashlib
import json
from pathlib import Path
import subprocess
import sys
import uuid

sys.path.insert(0,str(Path(__file__).resolve().parents[2]/'scripts/acceptance'))
import check_install_home as install
import check_update_home as update
from check_cli import decode
from check_lifecycle import binding

CAPABILITY='operationWorkspaceInventoryPolicy'
A='a'*40
B='b'*40


def set_capability(home,value):
    path=home/'system/build-info.json'
    info=json.loads(path.read_text('utf-8'))
    info.pop(CAPABILITY,None)
    if value is not None:
        info[CAPABILITY]=value
    path.write_text(json.dumps(info),encoding='utf-8')
    install.rehash(home)


def factory(binary,root,source,capability):
    update.factory(binary,root,source)
    set_capability(root,capability)
    return root


class InstalledClient:
    def __init__(self,binary,home):
        self.binary=str(binary);self.root=home;self.revision=0

    def request(self,action,payload=None,actor='coordinator',ok=True):
        request=dict(schemaVersion=1,rootKind='installation',projectRoot=str(self.root),
            actor=actor,action=action,expectedRevision=self.revision,requestId=str(uuid.uuid4()),payload=payload or {})
        response=decode(subprocess.run([self.binary,'request'],input=json.dumps(request),
            capture_output=True,text=True,encoding='utf-8',timeout=30))
        assert response['ok'] is ok,(action,response)
        self.revision=max(self.revision,response['revision'])
        return response['data']


def keep_output(binary,home,large):
    private=home/'OCMI_Sync/coordinator/bounds';private.mkdir()
    (private/'contract.md').write_text('Retain only fictional registered output until reader compatibility is verified.',encoding='utf-8')
    (private/'input.txt').write_text('fixed',encoding='utf-8')
    child='''import pathlib,sys,tempfile,shutil
scratch=pathlib.Path(tempfile.gettempdir());report=pathlib.Path(sys.argv[1])
if sys.argv[3]=='large':
 shutil.copyfile(sys.argv[2],scratch/'native-artifact.bin')
 with (scratch/'native-artifact.bin').open('ab') as stream:
  if stream.tell()<21<<20: stream.truncate(21<<20)
 for n in range(292): (scratch/('part-%03d.txt'%n)).write_bytes(b'fictional scratch')
else: (scratch/'small.txt').write_bytes(b'fictional retained output')
(report/'scratch.txt').write_text(str(scratch),encoding='utf-8')
raise SystemExit(1)
'''
    compile(child,'<retained native output>','exec')
    client=InstalledClient(binary,home)
    client.request('resource.list')
    payload=binding(home)
    payload.update(reviewer='coordinator',contract='OCMI_Sync/coordinator/bounds/contract.md',inputs=['OCMI_Sync/coordinator/bounds/input.txt'])
    payload['command']=dict(id='retained-output',executable=sys.executable,args=['-c',child,str(private),str(binary),'large' if large else 'small'],cwd='.',timeoutMs=15000,outputLimitBytes=4096)
    client.request('task.bind',payload);client.request('task.ready')
    result=client.request('run',actor='worker-1')
    assert result['operationState']=='failed' and result['exitCode']==1,result
    scratch=Path((private/'scratch.txt').read_text('utf-8'))
    assert scratch.resolve().is_relative_to((home/'local/runs').resolve())
    resource='ocmi-workspace-'+result['operationId']
    state=client.request('resource.inspect',dict(resourceId=resource))
    assert state['lifecycleState']=='keep' and state['activeUses']==0,state
    return client,resource,scratch


def refresh(home,before):
    before['sync']=install.snapshot(home/'OCMI_Sync')
    before['rows']=update.rows(home)


def apply(tool,home,candidate,ok=True):
    return update.invoke(tool,'apply',home,'--candidate',candidate,'--expected-source',A,'--expected-candidate',B,ok=ok)


def refused(tool,home,action,candidate=None):
    before=install.snapshot(home)
    result=apply(tool,home,candidate,ok=False) if action=='apply' else update.invoke(tool,'rollback',home,ok=False)
    assert result['reason']=='workspace_inventory_reader_capability',result
    assert result.get('nextAction'),result
    assert install.snapshot(home)==before,'reader refusal mutated selected home'


def verify_legacy_closure(home,resource):
    client=InstalledClient(install.native_path(home),home)
    selected=client.request('resource.inspect',dict(resourceId=resource))
    assert selected['lifecycleState']=='removed' and selected['disposition']=='remove' and selected['pending'] is False,selected
    state=client.request('status')
    assert state['resourceSummary']['activeUses']==0 and state['operationState']=='failed' and state['taskState']=='in_progress',state
    listing=client.request('resource.list',dict(limit=100))
    assert not listing['truncated'] and len(listing['resources'])==state['resourceSummary']['total'],listing
    pending=[row for row in listing['resources']
             if row.get('pending') or row.get('dispositionPending')]
    # The deliberately failed, unaccepted task is not integrated/closed. Its
    # old runtime dependency changed during rollback and must remain visible;
    # the existing borrowed-history gates must not be weakened to hide it.
    assert state['resourceSummary']['pending']==len(pending)==1,(state,pending)
    row=pending[0]
    assert row['resourceId']==resource.replace('ocmi-workspace-','ocmi-runtime-',1)
    assert row['ownership']=='external' and row['observedState']=='unknown' and row['historicalReleased'] is False,row
    assert row['provider']=='local' and row['kind']=='file' and row['operationId']==state['operationId'],row
    assert row['logical']['taskId']==state['taskId'] and row['logical']['taskRevision']==state['taskRevision'],row
    assert row['logical']['purpose']=='runtime dependency',row
    print('WORKSPACE_UPDATE_LEGACY_OK actual old reader selected after physical workspace closure; unfinished-task runtime loan remains explicitly pending',flush=True)


def check(binary,legacy_binary=None,controls=False):
    with install.install_fixture(preserve_on_failure=True) as directory:
        base=Path(directory)
        if controls:
            home=factory(binary,base/'original',A,None)
            candidate=factory(binary,base/'candidate',B,None)
            tool=install.native_path(candidate)
            before=update.setup(home)
            _,_,scratch=keep_output(install.native_path(home),home,False)
            refresh(home,before)
            assert apply(tool,home,candidate)['phase']=='applied'
            update.preserved(home,before,B)
            assert scratch.exists()
            assert update.invoke(tool,'rollback',home)['phase']=='rolled_back'
            update.preserved(home,before,A)
            print('WORKSPACE_UPDATE_BASELINE_OK actual installed failed-run KEEP, same private rows/bytes, apply and rollback; reader declaration absent',flush=True)
            return

        home=factory(binary,base/'capable-original',A,1)
        candidate=factory(binary,base/'candidate',B,1)
        tool=install.native_path(candidate)
        before=update.setup(home)
        _,resource,scratch=keep_output(install.native_path(home),home,True)
        refresh(home,before)
        original=hashlib.sha256((scratch/'native-artifact.bin').read_bytes()).hexdigest()
        for declaration in (None,'1'):
            set_capability(candidate,declaration)
            refused(tool,home,'apply',candidate)
        set_capability(candidate,1)
        assert apply(tool,home,candidate)['phase']=='applied'
        update.preserved(home,before,B)
        assert update.invoke(tool,'rollback',home)['phase']=='rolled_back'
        update.preserved(home,before,A)
        assert hashlib.sha256((scratch/'native-artifact.bin').read_bytes()).hexdigest()==original
        current=InstalledClient(install.native_path(home),home);current.request('status')
        current.request('resource.dispose',dict(resourceId=resource,disposition='remove',reason='Verified compatible retained original output'))
        assert not scratch.parent.exists()
        print('WORKSPACE_UPDATE_CAPABLE_OK missing/invalid declaration refuses unchanged; compatible apply/rollback preserve enlarged private state and original cleanup',flush=True)

        assert legacy_binary,'actual old reader required for rollback boundary'
        legacy_kept=factory(legacy_binary,base/'legacy-retained-before-update',A,None)
        update.setup(legacy_kept)
        older,small_resource,small_scratch=keep_output(install.native_path(legacy_kept),legacy_kept,False)
        # Reject before beginning a transition whose fallback image could not
        # satisfy the declared retained-workspace policy after interruption.
        refused(tool,legacy_kept,'apply',candidate)
        older.request('resource.dispose',dict(resourceId=small_resource,disposition='remove',reason='Close supported small original output before transition'))
        assert not small_scratch.parent.exists()
        assert apply(tool,legacy_kept,candidate)['phase']=='applied'
        assert update.invoke(tool,'rollback',legacy_kept)['phase']=='rolled_back'
        print('WORKSPACE_UPDATE_ADMISSION_OK retained legacy output blocks unsafe fallback before update; after original closure transition and fallback succeed',flush=True)

        old=factory(legacy_binary,base/'legacy-original',A,None)
        previous=update.setup(old)
        assert apply(tool,old,candidate)['phase']=='applied'
        _,resource,scratch=keep_output(install.native_path(old),old,True)
        refresh(old,previous)
        refused(tool,old,'rollback')
        assert scratch.exists()
        current=InstalledClient(install.native_path(old),old);current.request('status')
        current.request('resource.dispose',dict(resourceId=resource,disposition='remove',reason='Close original output before legacy reader selection'))
        assert not scratch.parent.exists()
        refresh(old,previous)
        assert update.invoke(tool,'rollback',old)['phase']=='rolled_back'
        update.preserved(old,previous,A)
        verify_legacy_closure(old,resource)
    assert not base.exists(),'successful installed fixture not removed'


if __name__=='__main__':
    p=argparse.ArgumentParser();p.add_argument('--binary',required=True);p.add_argument('--legacy-binary');p.add_argument('--controls',action='store_true')
    a=p.parse_args()
    check(str(Path(a.binary).resolve(strict=True)),str(Path(a.legacy_binary).resolve(strict=True)) if a.legacy_binary else None,a.controls)
