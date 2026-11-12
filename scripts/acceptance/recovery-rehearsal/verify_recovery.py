"""Independent selected-recovery oracle, separate from the transport consumer."""
import argparse
import base64
import hashlib
import json
import os
import pathlib
import shutil
import subprocess
import sys
import uuid

p=argparse.ArgumentParser()
p.add_argument('--root',required=True)
p.add_argument('--binary')
p.add_argument('--source-root')
p.add_argument('--phase',choices=['seal','check'],required=True)
p.add_argument('--report')
a=p.parse_args();root=pathlib.Path(a.root).resolve();transport=root/'closed-transport'
events=[]

def git(project,*args):
    env={k:v for k,v in os.environ.items() if not k.upper().startswith('GIT_')}
    env.update(GIT_CONFIG_NOSYSTEM='1',GIT_CONFIG_GLOBAL=os.devnull,GIT_TERMINAL_PROMPT='0',GIT_OPTIONAL_LOCKS='0')
    result=subprocess.run(['git','-c','core.hooksPath=','-c','core.fsmonitor=false','-c','gc.auto=0',*args],cwd=project,env=env,capture_output=True,timeout=15)
    assert result.returncode==0,result.stderr
    return result.stdout

def inventory(project,include_git=False):
    return {f.relative_to(project).as_posix():hashlib.sha256(f.read_bytes()).hexdigest()
        for f in project.rglob('*') if f.is_file() and (include_git or '.git' not in f.relative_to(project).parts)}

def snapshot(project):
    return dict(head=git(project,'rev-parse','HEAD').decode().strip(),attached=git(project,'symbolic-ref','HEAD').decode().strip(),
        index=base64.b64encode(git(project,'ls-files','--stage','-z')).decode(),files=inventory(project))

def call(project,action,payload,ok=True):
    request=dict(schemaVersion=1,action=action,projectRoot=str(project),actor='main',expectedRevision=0,requestId=str(uuid.uuid4()),payload=payload)
    result=subprocess.run([a.binary,'request'],input=json.dumps(request),text=True,encoding='utf-8',capture_output=True,timeout=210)
    data=json.loads(result.stdout)
    events.append(dict(action=action,expectedOK=ok,response=data,exitCode=result.returncode,stderr=result.stderr))
    if a.report:pathlib.Path(a.report).write_text(json.dumps(events,indent=2),encoding='utf-8')
    assert data['ok'] is ok and result.returncode==(0 if ok else 2),data
    return data['data']

if a.phase=='seal':
    old=pathlib.Path(a.source_root).resolve() if a.source_root else root/'old-device/project'
    observed=snapshot(old)
    assert git(old,'cat-file','blob',':labels.py')!= (old/'labels.py').read_bytes(),'fixture lacks staged/unstaged distinction'
    assert not (old/'delete.txt').exists() and (old/'ignored.txt').is_file()
    (transport/'independent-oracle.json').write_text(json.dumps(observed,indent=2),encoding='utf-8')
    print('RECOVERY_ORACLE_SEALED head/index/all-selected-file-bytes independently observed')
else:
    assert not (root/'old-device').exists(),'old source available on new device'
    project=root/'new-device/project';expected=json.loads((transport/'independent-oracle.json').read_text())
    assert snapshot(project)==expected,'HEAD/index/WIP differs from independent old-device observation'
    assert 'fixture@example.com' not in (project/'.git/config').read_text()
    assert not git(project,'remote').strip(),'old remote imported'
    inspect=call(root,'checkpoint.inspect',dict(packagePath=str(transport/'coordinator')))
    assert inspect['readyToResume'] is False and inspect['references'][0]['status']=='unverified'
    handoff=(root/'new-device/coordinator/memory/tasks/main/HANDOFF.md').read_text()
    assert 'STOP' in handoff and 'unknown remote operation' in handoff
    assert not (root/'new-device/coordinator/.ocmi').exists()
    assert not (root/'new-device/topic/.ocmi').exists()
    corrupted=root/'altered-package';shutil.copytree(transport/'coordinator',corrupted)
    (corrupted/'data/STARTUP.md').write_text('altered bytes',encoding='utf-8')
    call(root,'checkpoint.inspect',dict(packagePath=str(corrupted)),False)
    occupied=root/'foreign-destination';occupied.mkdir();(occupied/'sentinel.txt').write_bytes(b'preserve foreign data\n')
    before=inventory(occupied,True)
    call(occupied,'checkpoint.restore',dict(packagePath=str(transport/'coordinator')),False)
    assert inventory(occupied,True)==before
    before=inventory(project,True)
    call(project,'checkpoint.git.restore',dict(packagePath=str(transport/'git')),False)
    assert inventory(project,True)==before,'Git replay changed WIP'
    scratch=root/'environment-scratch';scratch.mkdir()
    payload=dict(scratchRoot=str(scratch),sources=[dict(kind='python-project',path='pyproject.toml'),dict(kind='guide',path='docs/setup.md')],tools=[],
        provisions=[dict(kind='data',source='docs/setup.md',line=1),dict(kind='access',source='docs/setup.md',line=1)],continuation=dict(source='docs/setup.md',line=1))
    missing=call(project,'environment.inspect',payload)
    assert any(r['kind']=='python' and r['status']=='missing' for r in missing['requirements'])
    payload['tools']=[dict(kind='python',executable=str(pathlib.Path(sys.executable).resolve(strict=True)))]
    matched=call(project,'environment.inspect',payload)
    assert any(r['kind']=='python' and r['status']=='match' for r in matched['requirements'])
    assert all(r['status']=='unverified' for r in matched['provisions']) and matched['readyToResume'] is False
    assert inventory(project,True)==before,'environment diagnostics changed restored project'
    print('RECOVERY_INDEPENDENT_PASS exact head/index/files; altered/conflict/replay refusal; STOP/unknown; missing/match tool, data/access remain unverified')
