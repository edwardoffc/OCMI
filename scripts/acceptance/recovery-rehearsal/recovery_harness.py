"""Neutral, stdlib-only selected-recovery consumer.

Prepare and restore deliberately use different roots.  It invokes only a published
native binary; it never builds OCMI or executes saved START content.
"""
import argparse, base64, hashlib, json, os, pathlib, shutil, subprocess, sys, uuid

TIMEOUT = 60

def require(ok, message):
    if not ok: raise AssertionError(message)

def files(root):
    return {p.relative_to(root).as_posix(): p.read_bytes() for p in root.rglob('*') if p.is_file()}

def run(argv, cwd=None, data=None, timeout=TIMEOUT):
    return subprocess.run(argv, cwd=cwd, input=data, text=True, encoding='utf-8',
        capture_output=True, timeout=timeout)

def native(binary, root, action, payload, expected=True):
    request = dict(schemaVersion=1, action=action, projectRoot=str(root), actor='main',
        expectedRevision=0, requestId=str(uuid.uuid4()), payload=payload)
    p = run([str(binary), 'request'], data=json.dumps(request))
    try: response = json.loads(p.stdout)
    except json.JSONDecodeError as e: raise AssertionError('native non-JSON response: '+p.stderr[:400]) from e
    require(response.get('ok') is expected and p.returncode == (0 if expected else 2),
        '%s expected=%s got=%s stderr=%s' % (action, expected, response.get('code'), p.stderr[:400]))
    return response.get('data', {})

def owned(root, part):
    value = (root / part).resolve()
    require(str(value).startswith(str(root.resolve()) + os.sep), 'unsafe owned path')
    return value

def git(root, *args):
    p = run(['git', *args], cwd=root)
    require(p.returncode == 0, 'git '+repr(args)+' failed: '+p.stderr[:400])
    return p.stdout.strip()

def metadata(package, kind):
    require((package/'COMPLETE').is_file() and (package/'manifest.json').is_file(), 'closed package missing')
    m=json.loads((package/'manifest.json').read_text(encoding='utf-8'))
    require(m['kind'] == kind, 'wrong package kind')
    return m

def checkpoint_payload(package, checkpoint_id, selected):
    return dict(packagePath=str(package), checkpointId=checkpoint_id, systemVersion='1.3.5',
        sourceIdentity='fictional-previous-device', owner='recovery-consumer', purpose='fictional selected recovery',
        retainUntil='recovery acceptance', nextAction='Confirm fresh binding; STOP and unknown operation remain pending.',
        files=selected, references=[dict(id='fictional-remote-operation', required=True, status='unverified',
        nextAction='Do not replay; reconcile only after current confirmation.')])

def prepare(binary, root):
    old=owned(root, 'old-device'); transport=owned(root, 'closed-transport'); old.mkdir(); transport.mkdir()
    memory=old/'coordinator'; memory.mkdir()
    selected={'memory/BRIEF.md':b'Fictional selected coordinator only.\n',
      'memory/tasks/main/HANDOFF.md':b'STOP: do not relaunch unknown remote operation.\n',
      'history/decision.md':b'Unique fictional decision.\n', 'STARTUP.md':b'OLD START MUST NOT EXECUTE\n'}
    for n,v in selected.items(): (memory/n).parent.mkdir(parents=True,exist_ok=True);(memory/n).write_bytes(v)
    (memory/'unselected.txt').write_text('DO NOT TRANSFER',encoding='utf-8')
    topic=old/'selected-topic';topic.mkdir();(topic/'HANDOFF.md').write_text('Selected fictional topic.\n')
    package=transport/'coordinator'; native(binary,memory,'checkpoint.capture',checkpoint_payload(package,'fictional-coordinator',list(selected)))
    native(binary,topic,'checkpoint.capture',checkpoint_payload(transport/'topic','fictional-topic',['HANDOFF.md']))
    metadata(package,'files'); require(b'DO NOT TRANSFER' not in b''.join(files(package).values()),'nonselected memory exported')
    project=old/'project';project.mkdir();git(project,'init','-b','main');git(project,'config','user.email','fixture@example.com');git(project,'config','user.name','Fixture')
    (project/'.gitignore').write_text('ignored.txt\n');(project/'labels.py').write_text('def normalize_labels(values):\n    return sorted(v.strip() for v in values if v.strip())\n');(project/'docs').mkdir();(project/'docs/setup.md').write_text('Fictional setup only; no saved instruction is executable.\n');(project/'pyproject.toml').write_text('[project]\nname="fictional-labels"\nrequires-python=">=3.10,<4"\n');(project/'delete.txt').write_text('delete\n');git(project,'add','.');git(project,'commit','-m','base')
    (project/'labels.py').write_text('def normalize_labels(values):\n    return sorted(v.strip() for v in values if v.strip())\n# staged note\n');git(project,'add','labels.py');(project/'labels.py').write_text('def normalize_labels(values):\n    return sorted(v.strip() for v in values if v.strip())\n# staged note\n# unstaged note\n');(project/'delete.txt').unlink();(project/'untracked.txt').write_text('selected\n');(project/'ignored.txt').write_text('ignored\n');(project/'scenario.json').write_text('{"input":[" Beta ","beta","","ALPHA"],"expected":["alpha","beta"]}\n')
    git_payload=dict(packagePath=str(transport/'git'),checkpointId='fictional-git',systemVersion='1.3.5',sourceIdentity='fictional-old-project',owner='recovery-consumer',purpose='fictional Git WIP',retainUntil='recovery acceptance',nextAction='Confirm new project binding.',files=['untracked.txt','ignored.txt','scenario.json'],refs=['refs/heads/main'],references=[])
    native(binary,project,'checkpoint.git.capture',git_payload);metadata(transport/'git','git-wip')
    (transport/'expected.json').write_text(json.dumps({'selected':{k:base64.b64encode(v).decode() for k,v in selected.items()}},indent=2),encoding='utf-8')
    print('PREPARE_OK closed selected coordinator/topic/Git packages')

def restore(binary, root):
    transport=owned(root,'closed-transport'); state=json.loads((transport/'expected.json').read_text(encoding='utf-8')); selected={k:base64.b64decode(v) for k,v in state['selected'].items()}
    new=owned(root,'new-device');new.mkdir(); memory=new/'coordinator';memory.mkdir()
    package=transport/'coordinator'; native(binary,new,'checkpoint.inspect',dict(packagePath=str(package)))
    result=native(binary,memory,'checkpoint.restore',dict(packagePath=str(package)))
    require(result.get('readyToResume') is False and files(memory)==selected,'memory restore/binding state')
    require(b'OLD START MUST NOT EXECUTE' in (memory/'STARTUP.md').read_bytes(),'old START preserved as history only')
    before=files(memory);native(binary,memory,'checkpoint.restore',dict(packagePath=str(package)),False);require(files(memory)==before,'replay overwrote')
    corrupt=owned(root,'corrupt');shutil.copytree(package,corrupt);(corrupt/'COMPLETE').unlink(); empty=owned(root,'empty');empty.mkdir();native(binary,empty,'checkpoint.restore',dict(packagePath=str(corrupt)),False);require(not files(empty),'partial wrote destination')
    topic=new/'topic';topic.mkdir();native(binary,topic,'checkpoint.restore',dict(packagePath=str(transport/'topic')));require((topic/'HANDOFF.md').is_file(),'selected topic missing')
    project=new/'project';project.mkdir();native(binary,project,'checkpoint.git.restore',dict(packagePath=str(transport/'git')))
    require('unstaged note' in (project/'labels.py').read_text() and not (project/'delete.txt').exists() and (project/'untracked.txt').is_file() and (project/'ignored.txt').is_file(),'Git WIP mismatch')
    require(not (project/'.git/config').read_text(encoding='utf-8').find('fixture@example.com') >= 0,'old Git config imported')
    print('RESTORE_OK selected-only STOP/unknown retained, corrupt/occupied/replay refused, Git WIP recovered implementationProject='+str(project))

def preflight(binary, root):
    project=owned(root,'preflight');project.mkdir(exist_ok=True);(project/'pyproject.toml').write_text('[project]\nrequires-python=">=3.10"\n')
    payload=dict(scratchRoot=str(owned(root,'scratch')),sources=[dict(kind='python-project',path='pyproject.toml')],tools=[],provisions=[],continuation=None)
    (root/'scratch').mkdir(exist_ok=True); report=native(binary,project,'environment.inspect',payload)
    rows=[x for x in report.get('requirements',[]) if x.get('kind')=='python'];require(rows and rows[0]['status']=='missing','missing Python binding not visible')
    print('PREFLIGHT_OK environment.inspect reports missing Python binding without installation')

def main():
    p=argparse.ArgumentParser();p.add_argument('--binary',required=True);p.add_argument('--root',required=True);p.add_argument('--stage',choices=('prepare','restore','all','preflight'),default='all');a=p.parse_args()
    binary=pathlib.Path(a.binary).resolve(strict=True);root=pathlib.Path(a.root).resolve();root.mkdir(parents=True,exist_ok=True)
    require(binary.is_file(),'published native binary required')
    if a.stage=='preflight': preflight(binary,root)
    elif a.stage=='prepare': prepare(binary,root)
    elif a.stage=='restore': restore(binary,root)
    else: prepare(binary,root);restore(binary,root)
if __name__=='__main__': main()
