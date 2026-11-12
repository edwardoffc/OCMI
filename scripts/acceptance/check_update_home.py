"""Independent installed-home update consumer; fictional selected factories only."""
import argparse
from contextlib import closing
import hashlib
import json
import os
from pathlib import Path
import re
import sqlite3
import subprocess
import tempfile
import time

import check_install_home as install


COMPATIBILITY=dict(schemaVersion=1,layoutVersion=1,installationReceiptVersion=1,
    coordinatorMemoryVersion=1,coordinatorBindingVersion=1,stateSchemaVersion=4)
PHASES={'intent','staged','old_system','new_system','root_switch','roots',
    'metadata_switch','metadata','applied','recovering','rolled_back','unknown'}
PROGRESS=('oldCopied','newCopied','rootProgress','metadataProgress','recoveryStep')
DIAGNOSTIC_ENUMS={
    'boundary':{'before_admission','recover_public','unknown'},
    'validator':{'recovery_start.root_placement','recovery_start.metadata_placement',
        'recovery_start','recovery_state','recover_public','unknown'},
    'kind':{'mismatch','io_error','unknown'},
    'operation':{'read','stat','native_id','read_dir','walk','rename','write','path_check'},
    'phase':PHASES,
    'predicate':set(('unknown root_content root_mode root_identity root_placement root_progress '
        'system_content system_mode system_member_identity system_identity system_placement '
        'metadata_content metadata_read stage_identity home_identity slot_identity new_slot_identity '
        'stage_inventory stage_member root_inventory root_directory root_member root_slot '
        'coordinator_presence stage_path slot_path system_path stage_proof system_inventory '
        'private_root admission admission_path admission_identity admission_inventory system_slot '
        'backup_content public_content public_identity public_mode public_inventory '
        'recovery_rename recovery_write recovery_journal activation_phase recovery_phase stage_presence').split())}
_report_context={}


def safe_failure_report(response,before,after):
    """Closed vocabulary only: never serialize native envelopes/journals wholesale."""
    def numbers(value,keys):
        return {key:value[key] for key in keys if type(value.get(key)) is int and 0<=value[key]<=2**63-1}

    def diagnostic(value,probe=False):
        if not isinstance(value,dict):
            return {}
        out={key:item for key,allowed in DIAGNOSTIC_ENUMS.items()
            if isinstance(item:=value.get(key),str) and item in allowed}
        if probe and value.get('kind')=='match':
            out['kind']='match'
        out.update(numbers(value,(*PROGRESS,'osCode','memberOrdinal','probe')))
        if not probe and isinstance(value.get('observations'),list):
            out['observations']=[diagnostic(item,probe=True) for item in value['observations'][:5]]
        return out

    def journal(value):
        if not isinstance(value,dict):
            return {}
        out=numbers(value,(*PROGRESS,'errno'))
        for key,length in (('sha256',64),('oldSource',40),('newSource',40)):
            item=value.get(key)
            if isinstance(item,str) and re.fullmatch('[0-9a-f]{'+str(length)+'}',item):
                out[key]=item
        if isinstance(value.get('phase'),str) and value['phase'] in PHASES:
            out['phase']=value['phase']
        if value.get('observation') in ('io','oversize'):
            out['observation']=value['observation']
        return out

    report={'schemaVersion':1,'before':journal(before),'after':journal(after)}
    if isinstance(response,dict) and response.get('ok') is False and response.get('code')=='UPDATE_REFUSED':
        data=response.get('data')
        if isinstance(data,dict):
            report['diagnostic']=diagnostic(data.get('diagnostic'))
    raw=json.dumps(report,sort_keys=True,separators=(',',':')).encode('utf-8')
    if len(raw)>4096:
        raw=b'{"schemaVersion":1,"report":"bounds_exceeded"}'
    return raw


def write_failure_report(base):
    before=_report_context.get('before',{})
    home=_report_context.get('home')
    after=observe_rollback_journal(home,remember=False) if home is not None else {}
    raw=safe_failure_report(_report_context.get('response'),before,after)
    descriptor,name=tempfile.mkstemp(prefix='ocmi-update-report-',suffix='.json',dir=base.parent)
    with os.fdopen(descriptor,'wb') as stream:
        stream.write(raw)
    print('UPDATE_HOME_SANITIZED_REPORT_WRITTEN',flush=True)


def invoke(binary, action, home, *args, ok=True, timeout=30):
    result=subprocess.run([str(binary),'update',action,'--home',str(home),*map(str,args)],
        capture_output=True,text=True,encoding='utf-8',timeout=timeout)
    response=json.loads(result.stdout)
    _report_context['response']=response
    assert response.get('schemaVersion')==1,response
    assert response.get('ok') is ok,(action,result.returncode,response,result.stderr)
    assert result.returncode==(0 if ok else 2),(action,result.returncode,response)
    data=response.get('data')
    assert isinstance(data,dict),response
    if not ok:
        assert all(isinstance(data.get(k),str) and data[k] for k in ('reason','stage','nextAction')),response
    return data


def factory(binary, root, source):
    install.factory(binary,root)
    info=json.loads((root/'system/build-info.json').read_text('utf-8'))
    info['sourceCommit']=source
    info['updateCompatibility']=COMPATIBILITY
    (root/'system/build-info.json').write_text(json.dumps(info),encoding='utf-8')
    install.rehash(root)
    return root


def rows(home):
    path=home/'local/state/state.sqlite'
    with closing(sqlite3.connect(f'{path.as_uri()}?mode=ro',uri=True)) as db:
        tables=[r[0] for r in db.execute("SELECT name FROM sqlite_master WHERE type='table' ORDER BY name")]
        return {t:sorted(db.execute('SELECT * FROM "'+t.replace('"','""')+'"').fetchall(),key=repr) for t in tables}


def setup(home,language='en'):
    install.invoke(home,'install')
    install.invoke(home,'coordinator','prepare','--language',language,'--client-project','Fictional project')
    install.invoke(home,'coordinator','confirm','--client-project','Fictional project',
        '--evidence-source','user-current','--evidence','Fictional reported association only.')
    for relative in ('OCMI_Sync/coordinator/HANDOFF.md','OCMI_Sync/personal/topic/WIP.txt',
            'OCMI_Sync/recovery/selected.bin','local/cache/keep.bin','local/unknown/keep.bin'):
        p=home/relative;p.parent.mkdir(parents=True,exist_ok=True)
        p.write_bytes(('Unique fictional bytes: '+relative+'\n').encode())
    return {'sync':install.snapshot(home/'OCMI_Sync'),'cache':install.snapshot(home/'local/cache'),
        'unknown':install.snapshot(home/'local/unknown'),'rows':rows(home),
        'receipt':json.loads((home/'local/bindings/installation.json').read_text('utf-8')),
        'binding':json.loads((home/'local/bindings/coordinator/coordinator.json').read_text('utf-8'))}


def preserved(home,before,source):
    assert install.snapshot(home/'OCMI_Sync')==before['sync'],'Semantic/private WIP changed'
    assert install.snapshot(home/'local/cache')==before['cache'],'Cache changed'
    assert install.snapshot(home/'local/unknown')==before['unknown'],'Unknown private bytes changed'
    assert rows(home)==before['rows'],'Core state/history/STOP changed'
    for filename,key in [('installation.json','receipt'),('coordinator/coordinator.json','binding')]:
        actual=json.loads((home/'local/bindings'/filename).read_text('utf-8'))
        expected=dict(before[key],sourceCommit=source)
        assert actual==expected,(filename,actual,expected)
    startup=(home/'local/bindings/coordinator/STARTUP.md').read_text('utf-8')
    assert source in startup and str(home) in startup and 'Fictional project' in startup
    assert install.invoke(home,'coordinator','inspect')['phase']=='confirmed'


def no_effect(binary,action,home,*args):
    before=install.snapshot(home)
    invoke(binary,action,home,*args,ok=False)
    assert install.snapshot(home)==before,'Refusal mutated selected home'


def observe_rollback_journal(home,*,remember=True):
    """Read metadata once; this is not native proof/placement validation."""
    try:
        with (home/'local/bindings/update.json').open('rb') as stream:
            raw=stream.read((16 << 20)+1)
    except OSError as error:
        observation={'observation':'io','errno':error.errno}
    else:
        observation={'sha256':hashlib.sha256(raw).hexdigest()} if len(raw)<=16 << 20 else {'observation':'oversize'}
        try:
            value=json.loads(raw) if len(raw)<=16 << 20 else {}
        except (ValueError,UnicodeError):
            value={}
        if not isinstance(value,dict):
            value={}
        phase=value.get('phase')
        observation['phase']=phase if isinstance(phase,str) and phase in PHASES else 'unknown'
        for key in PROGRESS:
            progress=value.get(key)
            if type(progress) is int and 0<=progress<=2**63-1:
                observation[key]=progress
        for key in ('oldSource','newSource'):
            source=value.get(key)
            if isinstance(source,str) and re.fullmatch('[0-9a-f]{40}',source):
                observation[key]=source
    if remember:
        _report_context.update(home=home,before=observation)
    print('UPDATE_HOME_PRE_ROLLBACK',json.dumps(observation,sort_keys=True),flush=True)
    return observation


def exercise_live_image(tool,home,candidate,old,new):
    if os.name!='nt':
        return
    child=subprocess.Popen([str(install.native_path(home)),'mcp','--project-root',str(home),
        '--root-kind','installation','--actor','coordinator'],stdin=subprocess.PIPE,stdout=subprocess.PIPE,stderr=subprocess.PIPE)
    try:
        time.sleep(.15)
        assert child.poll() is None,'Actual native MCP image did not remain live'
        no_effect(tool,'apply',home,'--candidate',candidate,'--expected-source',old,'--expected-candidate',new)
    finally:
        child.terminate()
        try:child.communicate(timeout=5)
        except subprocess.TimeoutExpired:child.kill();child.communicate(timeout=5)
    print('UPDATE_HOME_BUSY_IMAGE_OK actual live Windows executable refuses without target writes')


def exercise(binary,base):
    a='a'*40;b='b'*40;c='c'*40
    candidate=factory(binary,base/'candidate',b)
    tool=install.native_path(candidate)
    for language in ('en','ru'):
        home=factory(binary,base/('home-'+language),a)
        old_public=install.snapshot(home)
        before=setup(home,language)
        snapshot=install.snapshot(home)
        assert invoke(tool,'inspect',home,'--candidate',candidate)['phase']=='ready'
        assert install.snapshot(home)==snapshot,'Update inspect wrote state'
        args=('--candidate',candidate,'--expected-source',a,'--expected-candidate',b)
        no_effect(tool,'apply',home,*('--candidate',candidate,'--expected-source',c,'--expected-candidate',b))
        no_effect(tool,'apply',home,*args,'--home','')
        no_effect(tool,'apply',home,*args,'--expected-source','')
        assert invoke(tool,'apply',home,*args)['phase']=='applied'
        preserved(home,before,b)
        for name,digest in install.snapshot(candidate).items():
            assert hashlib.sha256((home/name).read_bytes()).hexdigest()==digest,('public payload',name)
        replay=install.snapshot(home)
        assert invoke(tool,'apply',home,*args)['phase']=='applied'
        assert install.snapshot(home)==replay,'Same update replay wrote files'
        assert invoke(install.native_path(home),'inspect',home)['sourceCommit']==b
        (home/'OCMI_Sync/personal/topic/WIP.txt').write_bytes(b'Later fictional private WIP\n')
        before['sync']=install.snapshot(home/'OCMI_Sync')
        observation=observe_rollback_journal(home)
        (base/('pre-rollback-'+language+'.json')).write_text(json.dumps(observation,sort_keys=True),encoding='utf-8')
        assert invoke(tool,'rollback',home)['phase']=='rolled_back'
        preserved(home,before,a)
        for name,digest in old_public.items():
            assert hashlib.sha256((home/name).read_bytes()).hexdigest()==digest,('rollback public',name)
        print('UPDATE_HOME_OK',language,'external updater, actual installed entry, replay and rollback preserve context')
    home=factory(binary,base/'refusal-home',a)
    setup(home)
    args=('--candidate',candidate,'--expected-source',a,'--expected-candidate',b)
    exercise_live_image(tool,home,candidate,a,b)
    no_effect(tool,'apply',home,*('--candidate',home,'--expected-source',a,'--expected-candidate',a))
    no_effect(install.native_path(home),'apply',home,*args)
    (home/'system/README.md').write_text('Fictional public WIP',encoding='utf-8')
    no_effect(tool,'apply',home,*args)
    (home/'system/README.md').write_text('# Fictional OCMI source\n',encoding='utf-8')
    candidate_info=candidate/'system/build-info.json'
    original=candidate_info.read_bytes()
    for key,value in [('stateSchemaVersion',999),('layoutVersion',999)]:
        info=json.loads(original);info['updateCompatibility'][key]=value
        candidate_info.write_text(json.dumps(info),encoding='utf-8');install.rehash(candidate)
        no_effect(tool,'apply',home,*args)
    candidate_info.write_bytes(original);install.rehash(candidate)
    # Closing another fd for this DB in the lock-owning process can release POSIX
    # record locks. Snapshot outside the held transaction, not through no_effect.
    before_busy=install.snapshot(home)
    with closing(sqlite3.connect(home/'local/state/state.sqlite')) as db:
        db.execute('BEGIN IMMEDIATE')
        try: invoke(tool,'apply',home,*args,ok=False)
        finally: db.rollback()
    assert install.snapshot(home)==before_busy,'Busy refusal mutated selected home'
    next_candidate=factory(binary,base/'next-candidate',c)
    assert invoke(tool,'apply',home,*args)['phase']=='applied'
    next_tool=install.native_path(next_candidate)
    assert invoke(next_tool,'apply',home,'--candidate',next_candidate,'--expected-source',b,'--expected-candidate',c)['phase']=='applied'
    observation=observe_rollback_journal(home)
    (base/'pre-rollback-last-good.json').write_text(json.dumps(observation,sort_keys=True),encoding='utf-8')
    assert invoke(next_tool,'rollback',home)['sourceCommit']==b
    print('UPDATE_HOME_REFUSAL_OK flags/source/self/dirty/format/busy; subsequent update and last-good rollback')


def qualify(binary,base):
    home=factory(binary,base/'control','a'*40)
    before=setup(home)
    preserved(home,before,'a'*40)
    p=home/'OCMI_Sync/personal/topic/WIP.txt';raw=p.read_bytes();p.write_bytes(b'Wrong copy-over behavior')
    rejected=False
    try: preserved(home,before,'a'*40)
    except AssertionError: rejected=True
    finally: p.write_bytes(raw)
    assert rejected,'Preservation oracle accepted targeted wrong behavior'
    assert rows(home)==before['rows']
    print('UPDATE_CONTROL_OK actual native installed/confirmed fixture and SQLite row snapshot; copy-over canary mutation rejected')


def exercise_existing_pair(previous_home,candidate):
    """Actual already-qualified full factories; source/member/notices checks remain producer-owned."""
    previous_home=Path(previous_home);candidate=Path(candidate)
    old=json.loads((previous_home/'system/build-info.json').read_text('utf-8'))['sourceCommit']
    new=json.loads((candidate/'system/build-info.json').read_text('utf-8'))['sourceCommit']
    assert old!=new,'Joint package qualification requires distinct real source revisions'
    public=install.snapshot(previous_home)
    before=setup(previous_home,'ru')
    tool=install.native_path(candidate)
    exercise_live_image(tool,previous_home,candidate,old,new)
    # Complete source kits contain hundreds of durable files. Bound these two
    # full-payload operations separately; small fixtures and refusals keep 30 s.
    assert invoke(tool,'apply',previous_home,'--candidate',candidate,
        '--expected-source',old,'--expected-candidate',new,timeout=180)['phase']=='applied'
    preserved(previous_home,before,new)
    assert invoke(install.native_path(previous_home),'inspect',previous_home)['sourceCommit']==new
    for name,digest in install.snapshot(candidate).items():
        assert hashlib.sha256((previous_home/name).read_bytes()).hexdigest()==digest,('actual updated member',name)
    (previous_home/'OCMI_Sync/personal/topic/WIP.txt').write_bytes(b'Later actual-pair fictional WIP\n')
    before['sync']=install.snapshot(previous_home/'OCMI_Sync')
    observe_rollback_journal(previous_home)
    assert invoke(tool,'rollback',previous_home,timeout=180)['sourceCommit']==old
    preserved(previous_home,before,old)
    for name,digest in public.items():
        assert hashlib.sha256((previous_home/name).read_bytes()).hexdigest()==digest,('actual rollback member',name)
    print('UPDATE_KIT_PAIR_OK actual previous/current full public factories; source, private state, installed entry and rollback')


if __name__=='__main__':
    parser=argparse.ArgumentParser()
    parser.add_argument('--binary',type=Path,required=True)
    parser.add_argument('--qualify-control',action='store_true')
    args=parser.parse_args()
    with install.install_fixture(preserve_on_failure=True) as scratch:
        try:
            (qualify if args.qualify_control else exercise)(args.binary.resolve(),Path(scratch))
        except BaseException:
            try:
                write_failure_report(Path(scratch))
            except Exception:
                print('UPDATE_HOME_SANITIZED_REPORT_UNAVAILABLE',flush=True)
            raise
