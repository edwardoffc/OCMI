"""Independent native home/coordinator consumer; synthetic data, no client trust claim."""
import argparse
import base64
from concurrent.futures import ThreadPoolExecutor
from contextlib import closing, contextmanager
import hashlib
import json
import os
from pathlib import Path
import platform
import shutil
import sqlite3
import subprocess
import tempfile
import time
import uuid


PATHS = dict(system='system', bin='system/bin', sync='OCMI_Sync',
    coordinator='OCMI_Sync/coordinator', personal='OCMI_Sync/personal',
    recovery='OCMI_Sync/recovery', local='local', state='local/state',
    store='local/state/state.sqlite', bindings='local/bindings', cache='local/cache', runs='local/runs')
TEXT = dict(schemaVersion=1,
    rootReadme='# OCMI\n\nOpen system/BOOTSTRAP.md for selected setup.\n',
    rootAgent='Read system/BOOTSTRAP.md; no role or installation from opening alone.\n',
    syncReadme='Selected portable semantic files only; no local runtime state.\n',
    coordinatorAgent='Follow <GUIDE_RELATIVE>; read INDEX.md and HANDOFF.md only.\n',
    coordinatorIndex='Current semantic context: HANDOFF.md. Pointers do not grant access.\n',
    coordinatorHandoff='No personal or work project selected. Current device handover stays local.\n',
    startup='Act as the selected OCMI coordinator.\nHome: <HOME>\nMemory: <MEMORY>\nGuide: <GUIDE>\nProject: <PROJECT>\nSource: <SOURCE>\nLanguage: <LANGUAGE>\nVerify actual current client association before recording handover.\n')


def digest(raw):
    return hashlib.sha256(raw).hexdigest()


@contextmanager
def install_fixture():
    directory=tempfile.TemporaryDirectory(prefix='ocmi-install-home-')
    root=Path(directory.name)
    identity=root.stat(follow_symlinks=False)
    try:
        yield directory.name
    finally:
        for attempt in range(4):
            if root.exists():
                current=root.stat(follow_symlinks=False)
                if root.is_symlink() or (current.st_dev,current.st_ino)!=(identity.st_dev,identity.st_ino):
                    raise RuntimeError('Owned installation fixture root changed; preserve for review')
            try:
                directory.cleanup()
                break
            except OSError as error:
                if os.name!='nt' or getattr(error,'winerror',None) not in (5,32,33) or attempt==3:
                    raise
                print('INSTALL_HOME_CLEANUP_WAIT windows access/sharing denial; bounded retry',attempt+1)
                time.sleep((0.1,0.2,0.4)[attempt])


def target():
    system = {'Windows': 'windows', 'Darwin': 'darwin', 'Linux': 'linux'}[platform.system()]
    arch = 'arm64' if platform.machine().lower() in ('aarch64', 'arm64') else 'amd64'
    return system+'-'+arch


def native_path(home):
    return home/('system/bin/ocmi.exe' if os.name == 'nt' else 'system/bin/ocmi')


def snapshot(root):
    # Only a synthetic or explicitly passed consumer home; never enumerate user roots.
    return {p.relative_to(root).as_posix(): digest(p.read_bytes())
            for p in root.rglob('*') if p.is_file() and not p.is_symlink()}


def invoke(home, *arguments, ok=True):
    env = dict(os.environ, PATH='')
    process = subprocess.run([str(native_path(home)), *arguments, '--home', str(home)],
        capture_output=True, text=True, encoding='utf-8', env=env, timeout=20)
    try:
        response = json.loads(process.stdout)
    except Exception as error:
        raise AssertionError(('native envelope', process.returncode, process.stdout[:512], process.stderr[:512])) from error
    assert response.get('schemaVersion') == 1 and response.get('ok') is ok, (arguments, response)
    assert process.returncode == (0 if ok else 2), (arguments, response, process.returncode)
    if not ok:
        assert all(isinstance(response['data'].get(key),str) and response['data'][key].strip()
            for key in ('reason','stage','nextAction')), ('missing setup diagnosis',arguments,response)
    return response['data']


def factory(binary, home):
    home.mkdir()
    entry = native_path(home); entry.parent.mkdir(parents=True)
    shutil.copyfile(binary, entry); entry.chmod(0o755)
    declaration = dict(schemaVersion=1, layoutVersion=1, kind='installation', paths=PATHS)
    (home/'.ocmi-home.json').write_text(json.dumps(declaration), encoding='utf-8')
    for name, raw in {'README.md':TEXT['rootReadme'], 'README.ru.md':TEXT['rootReadme'],
            'AGENTS.md':TEXT['rootAgent'], '.gitignore':'/OCMI_Sync/\n/local/\n/.local/\n/.codex/\n',
            '.gitattributes':'/OCMI_Sync export-ignore\n/local export-ignore\n/.local export-ignore\n/.codex export-ignore\n',
            'system/VERSION.md':'# OCMI 1.3.5\n\nOne release number covers the system and all profiles.\n\n[Start](BOOTSTRAP.md).\n',
            'system/README.md':'# Fictional OCMI source\n',
            'system/README.ru.md':'# Вымышленный исходный комплект OCMI\n',
            'system/AGENTS.md':'Use BOOTSTRAP.md only for selected installation.\n',
            'system/BOOTSTRAP.md':'# Fictional selected installation route\n',
            'system/locales/ru/BOOTSTRAP.md':'# Вымышленный выбранный путь установки\n',
            'system/COORDINATOR.md':'# Fictional coordinator guide\n',
            'system/locales/ru/COORDINATOR.md':'# Вымышленное руководство координатора\n',
            'system/packaging/home/text.json':json.dumps(TEXT),
            'system/locales/ru/packaging/home/text.json':json.dumps(TEXT)}.items():
        path=home/name; path.parent.mkdir(parents=True, exist_ok=True); path.write_text(raw, encoding='utf-8')
    info=dict(distributionKind='ready-kit', systemVersion='1.3.5', sourceCommit='a'*40,
        sourceDirty=False, layoutVersion=1, layoutDeclaration='.ocmi-home.json',
        binaryPath=entry.relative_to(home).as_posix(), target=target())
    (home/'system/build-info.json').write_text(json.dumps(info), encoding='utf-8')
    records = sorted(snapshot(home).items())
    (home/'SHA256SUMS').write_text(''.join(sha+'  '+name+'\n' for name,sha in records), encoding='utf-8')
    assert len(records) == len(snapshot(home))-1
    assert all(digest((home/name).read_bytes()) == sha for name,sha in records)
    return home


def exercise(home, language='en'):
    public_before=snapshot(home)
    assert invoke(home,'install','--inspect')['phase']=='uninitialized'
    assert snapshot(home)==public_before, 'inspection wrote files'
    assert not (home/'local').exists() and not (home/'OCMI_Sync').exists()
    invoke(home,'install','--home','','--inspect',ok=False)
    assert snapshot(home)==public_before, 'empty duplicate setup flag was accepted or wrote state'
    invoke(home,'coordinator','confirm','--client-project','Fictional project',
        '--evidence-source','user-current','--evidence','Fictional confirmation.',ok=False)
    assert snapshot(home)==public_before
    installed=invoke(home,'install'); assert installed['phase']=='installed', installed
    receipt_path=home/'local/bindings/installation.json'
    receipt=json.loads(receipt_path.read_text('utf-8'))
    for key in ('installationId','deviceId','copyId'): uuid.UUID(receipt[key])
    assert installed['installationId']==receipt['installationId']
    with closing(sqlite3.connect((home/'local/state/state.sqlite').as_uri()+'?mode=ro',uri=True)) as db:
        assert db.execute('SELECT device_id,copy_id,owner FROM project').fetchone()==(receipt['deviceId'],receipt['copyId'],'coordinator')
    state=receipt_path.read_bytes(); invoke(home,'install'); assert receipt_path.read_bytes()==state
    assert invoke(home,'coordinator','inspect')['phase']=='not_prepared'
    assert not (home/'OCMI_Sync/coordinator').exists()
    before=snapshot(home)
    invoke(home,'coordinator','prepare','--language',language,'--client-project','','--client-project','Fictional project',ok=False)
    assert snapshot(home)==before, 'empty duplicate project flag changed coordinator state'
    prepared=invoke(home,'coordinator','prepare','--language',language,'--client-project','Fictional project')
    assert prepared['phase']=='handover_pending', prepared
    memory=home/'OCMI_Sync/coordinator'; prompt=home/'local/bindings/coordinator/STARTUP.md'
    assert Path(prepared['startupPromptPath'])==prompt and Path(prepared['coordinatorMemoryPath'])==memory
    semantic=json.loads((memory/'coordinator.json').read_text('utf-8'))
    assert set(semantic)=={'schemaVersion','coordinatorId','language'} and semantic['schemaVersion']==1
    assert semantic['language']==language
    binding=json.loads((home/'local/bindings/coordinator/coordinator.json').read_text('utf-8'))
    required={'schemaVersion','installationId','deviceId','copyId','coordinatorId','home',
        'clientProject','language','phase','sourceCommit'}
    assert required <= set(binding), ('binding field contract',sorted(binding))
    assert not any(key[0].isupper() for key in binding), 'machine binding uses noncanonical fields'
    uuid.UUID(semantic['coordinatorId'])
    body=prompt.read_text('utf-8'); assert str(home) in body and 'Fictional project' in body and '<HOME>' not in body
    for name in ('AGENTS.md','INDEX.md','HANDOFF.md'): assert (memory/name).is_file()
    guide='../../system/'+('locales/ru/' if language=='ru' else '')+'COORDINATOR.md'
    assert guide in (memory/'AGENTS.md').read_text('utf-8'), 'wrong selected guide'
    for path in memory.rglob('*'):
        if path.is_file():
            raw=path.read_bytes(); assert str(home).encode() not in raw and b'Fictional project' not in raw
            assert all(receipt[key].encode() not in raw for key in ('installationId','deviceId','copyId'))
    (memory/'HANDOFF.md').write_text('Unique fictional semantic continuation.\n',encoding='utf-8')
    saved=snapshot(memory)
    invoke(home,'coordinator','prepare','--language',language,'--client-project','Fictional project')
    assert snapshot(memory)==saved, 'prepare overwrote semantic WIP'
    before=snapshot(home)
    invoke(home,'coordinator','confirm','--client-project','Wrong project','--evidence-source','user-current','--evidence','Fictional.',ok=False)
    assert snapshot(home)==before
    invoke(home,'coordinator','confirm','--client-project','Fictional project','--evidence-source','unknown','--evidence','Fictional.',ok=False)
    assert snapshot(home)==before
    confirmed=invoke(home,'coordinator','confirm','--client-project','Fictional project','--evidence-source','user-current','--evidence','Fixture records a reported confirmation only.')
    assert confirmed['phase']=='confirmed' and snapshot(memory)==saved
    before=snapshot(home)
    invoke(home,'coordinator','confirm','--client-project','Fictional project','--evidence-source','user-current','--evidence','Fixture records a reported confirmation only.')
    assert snapshot(home)==before, 'same confirmation was not idempotent'
    assert invoke(home,'coordinator','inspect')['phase']=='confirmed'
    assert not (home/'.ocmi').exists() and not (home/'OCMI_Sync/personal').exists()
    assert not (home/'OCMI_Sync/recovery').exists()
    for name,sha in public_before.items(): assert digest((home/name).read_bytes())==sha
    print('INSTALL_HOME_OK native inspect/install/coordinator/prompt/explicit reported handover/replay/semantic-device separation; client UI unverified')
    return receipt, saved


def exercise_recovery(factory_home, initialized, old_receipt, saved):
    memory=factory_home/'OCMI_Sync/coordinator'; memory.parent.mkdir(parents=True)
    shutil.copytree(initialized/'OCMI_Sync/coordinator',memory)
    invoke(factory_home,'install')
    prepared=invoke(factory_home,'coordinator','prepare','--language','en','--client-project','Travel workspace')
    assert prepared['phase']=='handover_pending' and snapshot(memory)==saved
    new_receipt=json.loads((factory_home/'local/bindings/installation.json').read_text('utf-8'))
    assert all(new_receipt[key]!=old_receipt[key] for key in ('installationId','deviceId','copyId'))
    body=(factory_home/'local/bindings/coordinator/STARTUP.md').read_text('utf-8')
    assert str(factory_home) in body and str(initialized) not in body and 'Travel workspace' in body
    assert invoke(factory_home,'coordinator','inspect')['phase']=='handover_pending'
    with closing(sqlite3.connect((factory_home/'local/state/state.sqlite').as_uri()+'?mode=ro',uri=True)) as db:
        observed=db.execute("SELECT mode,ownership,locator FROM resource_ledger WHERE kind='directory'").fetchall()
    assert any(mode=='use' and ownership=='external' and Path(locator)==memory
        for mode,ownership,locator in observed), 'restored memory lacks observation-only resource accounting'
    assert not (factory_home/'OCMI_Sync/personal').exists()
    print('INSTALL_HOME_RESTORE_OK selected semantic bytes/identity retained; fresh local IDs/prompt/pending binding; no old authority')


def rehash(home):
    records={name:sha for name,sha in snapshot(home).items()
        if name!='SHA256SUMS' and not name.startswith(('local/','OCMI_Sync/'))}
    (home/'SHA256SUMS').write_text(''.join(sha+'  '+name+'\n' for name,sha in sorted(records.items())),encoding='utf-8')


def refused_unchanged(home,*arguments):
    before=snapshot(home)
    invoke(home,*arguments,ok=False)
    assert snapshot(home)==before, ('refusal changed original files',arguments)


def exercise_context_boundaries(binary,base,initialized):
    root=factory(binary,base/'unowned startup')
    invoke(root,'install')
    prompt=root/'local/bindings/coordinator/STARTUP.md'; prompt.parent.mkdir(parents=True)
    canary=base/'unowned startup outside canary.md'
    canary.write_bytes(b'Unowned fictional startup must stay unchanged.\n')
    os.link(canary,prompt)
    before=canary.read_bytes()
    refused_unchanged(root,'coordinator','prepare','--language','en','--client-project','Fictional project')
    assert canary.read_bytes()==before, 'unowned startup write escaped selected placement'
    root=factory(binary,base/'copied binding only')
    binding=root/'local/bindings/coordinator/coordinator.json'
    binding.parent.mkdir(parents=True)
    shutil.copyfile(initialized/'local/bindings/coordinator/coordinator.json',binding)
    refused_unchanged(root,'coordinator','inspect')
    refused_unchanged(root,'coordinator','confirm','--client-project','Fictional project',
        '--evidence-source','user-current','--evidence','Fictional report.')
    refused_unchanged(root,'coordinator','prepare','--language','en','--client-project','Fictional project')
    root=factory(binary,base/'copied machine state')
    shutil.copytree(initialized/'local',root/'local')
    shutil.copytree(initialized/'OCMI_Sync',root/'OCMI_Sync')
    refused_unchanged(root,'install','--inspect')
    refused_unchanged(root,'coordinator','inspect')
    root=factory(binary,base/'malformed binding')
    invoke(root,'install')
    binding=root/'local/bindings/coordinator/coordinator.json'
    binding.parent.mkdir(parents=True); binding.write_text('{',encoding='utf-8')
    refused_unchanged(root,'coordinator','prepare','--language','en','--client-project','Fictional project')
    root=factory(binary,base/'restored language conflict')
    memory=root/'OCMI_Sync/coordinator'; memory.parent.mkdir(parents=True)
    shutil.copytree(initialized/'OCMI_Sync/coordinator',memory)
    invoke(root,'install')
    refused_unchanged(root,'coordinator','prepare','--language','ru','--client-project','Fictional project')
    root=factory(binary,base/'missing semantic continuation')
    exercise(root)
    (root/'OCMI_Sync/coordinator/HANDOFF.md').unlink()
    refused_unchanged(root,'coordinator','inspect')
    refused_unchanged(root,'coordinator','confirm','--client-project','Fictional project',
        '--evidence-source','user-current','--evidence','Fictional report.')
    root=factory(binary,base/'source drift')
    invoke(root,'install')
    info=root/'system/build-info.json'; value=json.loads(info.read_text('utf-8'))
    value['sourceCommit']='b'*40; info.write_text(json.dumps(value),encoding='utf-8'); rehash(root)
    refused_unchanged(root,'install','--inspect')
    for name in ('unowned.txt','system/OCMI_Sync/coordinator/private.txt','system/local/bindings/private.json','system/docs/memory/private.md','system/.LOCAL/private.txt'):
        root=factory(binary,base/('declared '+str(uuid.uuid4())))
        extra=root/name; extra.parent.mkdir(parents=True,exist_ok=True); extra.write_text('Unowned fictional bytes.',encoding='utf-8')
        rehash(root)
        refused_unchanged(root,'install')
    root=factory(binary,base/'missing required root')
    (root/'AGENTS.md').unlink(); rehash(root)
    refused_unchanged(root,'install')
    root=factory(binary,base/'version mismatch')
    (root/'system/VERSION.md').write_text('# OCMI 0.0.0\n',encoding='utf-8'); rehash(root)
    refused_unchanged(root,'install')
    root=factory(binary,base/'linked private bindings')
    invoke(root,'install')
    original=root/'local/bindings'; foreign=base/'foreign bindings'
    original.rename(foreign)
    try:
        original.symlink_to(foreign,target_is_directory=True)
    except OSError as error:
        print('INSTALL_HOME_PRIVATE_LINK_SKIP native link creation unavailable:',type(error).__name__)
    else:
        before=snapshot(foreign)
        refused_unchanged(root,'install','--inspect')
        refused_unchanged(root,'coordinator','prepare','--language','en','--client-project','Fictional project')
        assert snapshot(foreign)==before
        original.unlink()
    foreign.rename(original)
    print('INSTALL_HOME_CONTEXT_OK copied binding/state, malformed metadata, language conflict, missing memory, source drift, closed namespace and private links refuse unchanged')


def exercise_diagnosis(binary,base):
    payload=factory(binary,base/'diagnosis bad payload')
    (payload/'system/VERSION.md').write_text('corrupted fictional payload',encoding='utf-8')
    payload_error=invoke(payload,'coordinator','inspect',ok=False)
    binding=factory(binary,base/'diagnosis bad binding')
    invoke(binding,'install')
    local=binding/'local/bindings/coordinator/coordinator.json'; local.parent.mkdir(parents=True)
    local.write_text('{',encoding='utf-8')
    binding_error=invoke(binding,'coordinator','prepare','--language','en','--client-project','Fictional project',ok=False)
    restored=factory(binary,base/'diagnosis language')
    memory=restored/'OCMI_Sync/coordinator'; memory.mkdir(parents=True)
    (memory/'coordinator.json').write_text(json.dumps(dict(schemaVersion=1,coordinatorId=str(uuid.uuid4()),language='en')),encoding='utf-8')
    for name in ('AGENTS.md','INDEX.md','HANDOFF.md'): (memory/name).write_text('Fictional selected continuation.\n',encoding='utf-8')
    invoke(restored,'install')
    language_error=invoke(restored,'coordinator','prepare','--language','ru','--client-project','Fictional project',ok=False)
    def cause(error): return error['stage'],error['reason']
    assert cause(payload_error)!=cause(binding_error), 'payload and binding errors give identical diagnosis'
    assert cause(language_error)!=cause(binding_error), 'language conflict and malformed binding give identical diagnosis'
    assert cause(payload_error)==('factory','factory_payload'), ('payload diagnosis contract',payload_error)
    assert cause(binding_error)==('binding','binding_metadata'), ('binding diagnosis contract',binding_error)
    assert cause(language_error)==('language','restored_language'), ('language diagnosis contract',language_error)
    print('INSTALL_HOME_DIAGNOSIS_OK public payload, malformed binding and selected language conflict distinguishable without contents')


def check(binary):
    with install_fixture() as name:
        base=Path(name)
        home=factory(binary,base/'home with spaces Ω')
        receipt,saved=exercise(home)
        exercise_recovery(factory(binary,base/'new device'),home,receipt,saved)
        exercise_context_boundaries(binary,base,home)
        exercise_diagnosis(binary,base)
        exercise(factory(binary,base/'Russian home'), 'ru')
        for mutation in ('tampered','unknown-root','malformed-marker','partial-local','old-project','wrong-target'):
            root=factory(binary,base/mutation)
            if mutation=='tampered': (root/'system/VERSION.md').write_text('changed')
            elif mutation=='unknown-root': (root/'unowned.txt').write_text('retained foreign data')
            elif mutation=='malformed-marker': (root/'.ocmi-home.json').write_text('{')
            elif mutation=='partial-local': (root/'local/state').mkdir(parents=True); (root/'local/state/foreign.txt').write_text('do not adopt')
            elif mutation=='old-project': (root/'.ocmi/local').mkdir(parents=True); (root/'.ocmi/local/state.sqlite').write_bytes(b'foreign state')
            else:
                info_path=root/'system/build-info.json'; info=json.loads(info_path.read_text()); info['target']='unsupported'
                old=info_path.read_bytes(); new=json.dumps(info).encode(); info_path.write_bytes(new)
                sums=root/'SHA256SUMS'; sums.write_text(sums.read_text().replace(digest(old)+'  system/build-info.json',digest(new)+'  system/build-info.json'))
            before=snapshot(root); invoke(root,'install',ok=False); assert snapshot(root)==before, mutation
        root=factory(binary,base/'linked payload')
        link=root/'system/VERSION.md'; canary=base/'outside canary.txt'; canary.write_bytes(link.read_bytes())
        link.unlink()
        try:
            link.symlink_to(canary)
        except OSError as error:
            print('INSTALL_HOME_LINK_SKIP native link creation unavailable:',type(error).__name__)
        else:
            before=snapshot(root); canary_before=canary.read_bytes()
            invoke(root,'install',ok=False)
            assert snapshot(root)==before and canary.read_bytes()==canary_before
        root=factory(binary,base/'concurrent')
        with ThreadPoolExecutor(max_workers=2) as pool:
            def attempt(_):
                p=subprocess.run([str(native_path(root)),'install','--home',str(root)],capture_output=True,text=True,encoding='utf-8',env=dict(os.environ,PATH=''),timeout=20)
                value=json.loads(p.stdout); assert p.returncode in (0,2); return value
            results=list(pool.map(attempt,range(2)))
        assert any(value['ok'] and value['data']['phase']=='installed' for value in results), results
        assert invoke(root,'install','--inspect')['phase']=='installed'
        print('INSTALL_HOME_REFUSAL_OK tamper/unknown/marker/partial/legacy/target preserve originals; concurrent init coherent')


if __name__=='__main__':
    parser=argparse.ArgumentParser(); parser.add_argument('--binary',required=True)
    parser.add_argument('--home',help='Exercise the actual unpacked kit instead of synthetic factory cases')
    parser.add_argument('--language',choices=['en','ru'],default='en')
    args=parser.parse_args()
    if args.home:
        home=Path(args.home).resolve(strict=True)
        assert Path(args.binary).resolve(strict=True)==native_path(home).resolve(strict=True), 'wrong bundled consumer binary'
        exercise(home,args.language)
    else: check(str(Path(args.binary).resolve(strict=True)))
