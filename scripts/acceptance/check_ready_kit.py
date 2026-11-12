"""Independent ready-kit inventory and native selected-memory recovery consumer.

Fictional fixtures only. This does not confirm real client/UI trust or cloud behavior.
"""
import argparse
import hashlib
import json
import os
import pathlib
import platform
import re
import shutil
import stat
import subprocess
import tarfile
import tempfile
import uuid
import zipfile


ROOT_MEMBERS = {'.ocmi-home.json', 'README.md', 'README.ru.md', 'AGENTS.md',
                '.gitignore', '.gitattributes', 'SHA256SUMS'}


def digest(raw):
    return hashlib.sha256(raw).hexdigest()


def require(value, message):
    if not value:
        raise AssertionError(message)


def safe_name(name):
    parts = pathlib.PurePosixPath(name).parts
    normalized = parts[1:] if parts and parts[0].lower() == 'system' else parts
    normalized_name = '/'.join(normalized).lower()
    return (name and not name.startswith('/') and '\\' not in name and ':' not in name
            and all(p not in ('', '.', '..') for p in name.split('/'))
            and not any(p.lower() in ('.git', '.local', '.codex', '.ocmi', '__pycache__', 'node_modules') for p in normalized)
            and not (normalized and normalized[0].lower() in ('ocmi_sync', 'local'))
            and not normalized_name.startswith(('docs/memory/', 'docs/research/', 'locales/ru/docs/research/', 'diagnostics/'))
            and normalized_name not in ('.github/workflows/environment-binding-probe.yml', '.github/workflows/runtime-group-probe.yml'))


def archive_files(path):
    data, modes = {}, {}
    def take(name, size, mode, read):
        require(safe_name(name) and name.lower() not in {n.lower() for n in data}, 'unsafe/colliding archive member: '+name)
        require(0 <= size <= 128 << 20 and len(data) < 8192, 'archive limit')
        raw = read()
        require(len(raw) == size and sum(map(len, data.values())) + size <= 512 << 20, 'archive payload limit')
        data[name], modes[name] = raw, mode
    if str(path).endswith('.zip'):
        with zipfile.ZipFile(path) as z:
            require(z.testzip() is None, 'ZIP CRC')
            for m in z.infolist():
                require(not m.is_dir() and not stat.S_ISLNK(m.external_attr >> 16), 'non-file ZIP member')
                take(m.filename, m.file_size, None, lambda m=m: z.read(m))
    else:
        with tarfile.open(path, 'r:gz') as z:
            for m in z:
                require(m.isfile(), 'non-file TAR member')
                take(m.name, m.size, m.mode, lambda m=m: z.extractfile(m).read())
    return data, modes


def git(root, *args):
    p = subprocess.run(['git', '-c', 'core.fsmonitor=false', *args], cwd=root,
                       env=dict(os.environ, GIT_OPTIONAL_LOCKS='0'), capture_output=True, timeout=30)
    require(p.returncode == 0, 'source Git inspection failed')
    return p.stdout


def check_archive(path, source, commit, target):
    data, modes = archive_files(path)
    required = {'AGENTS.md', 'BOOTSTRAP.md', 'COORDINATOR.md', 'VERSION.md', 'README.md',
                '.gitignore', '.gitattributes', 'templates/coordinator/STARTUP.md',
                'locales/ru/BOOTSTRAP.md', 'templates/personal/INSTALL.md', 'templates/work/INSTALL.md'}
    require(ROOT_MEMBERS <= data.keys(), 'final home root members missing')
    require({'system/'+name for name in required} <= data.keys(), 'full system entrypoints missing')
    require('system/build-info.json' in data, 'package receipt missing')
    info = json.loads(data['system/build-info.json'])
    require(info['distributionKind'] == 'ready-kit' and info['sourceCommit'] == commit
            and info['sourceDirty'] is False
            and info['layoutVersion'] == 1 and info['layoutDeclaration'] == '.ocmi-home.json'
            and info['target'] == target, 'source/package identity')
    require(re.fullmatch('[0-9a-f]{40}', commit), 'exact source commit required')
    require(git(source, 'rev-parse', 'HEAD').decode().strip() == commit, 'wrong source checkout')
    names = [n.decode('utf-8') for n in git(source, 'ls-files', '-z').split(b'\0') if n]
    require(all(safe_name(n) for n in names), 'private/unsafe indexed source')
    indexed = {n: git(source, 'show', commit+':'+n) for n in names}
    version = re.search(rb'OCMI\s+(\d+\.\d+\.\d+)', indexed['VERSION.md'])
    require(version is not None and info['systemVersion'] == version.group(1).decode('ascii'), 'source/package version')
    text_keys = {'schemaVersion', 'rootReadme', 'rootAgent', 'syncReadme', 'coordinatorAgent',
                 'coordinatorIndex', 'coordinatorHandoff', 'startup'}
    text_en = json.loads(indexed['packaging/home/text.json'])
    text_ru = json.loads(indexed['locales/ru/packaging/home/text.json'])
    for label, text in (('en', text_en), ('ru', text_ru)):
        require(set(text) == text_keys and text['schemaVersion'] == 1
                and all(isinstance(text[key], str) and text[key] and len(text[key].encode('utf-8')) <= 32768
                        for key in text_keys-{'schemaVersion'}), 'invalid indexed home text: '+label)
        require(all(text['startup'].count(token) == 1
                    for token in ('<HOME>', '<MEMORY>', '<GUIDE>', '<PROJECT>', '<SOURCE>', '<LANGUAGE>')),
                'invalid indexed startup tokens: '+label)
    require(data['README.md'] == text_en['rootReadme'].encode('utf-8')
            and data['README.ru.md'] == text_ru['rootReadme'].encode('utf-8')
            and data['AGENTS.md'] == text_en['rootAgent'].encode('utf-8'), 'root text differs from indexed source')
    require(data['.gitignore'] == b'/OCMI_Sync/\n/local/\n/.local/\n/.codex/\n'
            and data['.gitattributes'] == b'/OCMI_Sync export-ignore\n/local export-ignore\n/.local export-ignore\n/.codex export-ignore\n',
            'root ignore/export rules differ')
    for n, raw in indexed.items():
        require(data.get('system/'+n) == raw, 'public source bytes differ: '+n)
    snapshot = info['systemSnapshot']
    entries = snapshot['files']
    require(len(entries) == len(indexed) and {x['path'] for x in entries} == set(indexed), 'system snapshot inventory')
    require(all(x['sha256'] == digest(indexed[x['path']]) for x in entries), 'system snapshot hashes')
    material = ''.join(x['sha256']+'  '+x['path']+'\n' for x in entries).encode()
    require(snapshot['sha256'] == digest(material), 'system snapshot identity')
    notices = json.loads(indexed['packaging/notices/manifest.json'])
    notice_paths = {'system/NOTICES/go/'+notices['toolchain']['notice']: notices['toolchain']['sha256']}
    for m in notices['modules']:
        notice_paths['system/NOTICES/modules/'+m['notice']] = m['sha256']
    require(any(m['path'] == 'github.com/pelletier/go-toml/v2' and m['version'] == 'v2.4.3'
                for m in notices['modules']), 'reviewed TOML notice missing')
    for n, sha in notice_paths.items():
        require(n in data and digest(data[n]) == sha, 'notice missing/changed: '+n)
    binary = 'system/bin/ocmi.exe' if target == 'windows-amd64' else 'system/bin/ocmi'
    require(info['binaryPath'] == binary, 'declared binary path')
    declaration = json.loads(data['.ocmi-home.json'])
    require(declaration == dict(schemaVersion=1, layoutVersion=1, kind='installation', paths={
        'system':'system', 'bin':'system/bin', 'sync':'OCMI_Sync', 'coordinator':'OCMI_Sync/coordinator',
        'personal':'OCMI_Sync/personal', 'recovery':'OCMI_Sync/recovery', 'local':'local', 'state':'local/state',
        'store':'local/state/state.sqlite', 'bindings':'local/bindings', 'cache':'local/cache', 'runs':'local/runs'}),
        'layout declaration differs')
    require(all(not n.startswith(('OCMI_Sync/', 'local/', '.local/', '.codex/')) for n in data), 'private state packaged')
    expected = {'system/'+n for n in indexed} | set(notice_paths) | ROOT_MEMBERS | {binary, 'system/build-info.json'}
    require(set(data) == expected, 'unexpected/missing complete package member')
    sums = {}
    for line in data['SHA256SUMS'].decode('utf-8-sig').splitlines():
        sha, name = line.split('  ', 1)
        require(name not in sums and re.fullmatch('[0-9a-f]{64}', sha), 'invalid checksum inventory')
        sums[name] = sha
    require(set(sums) == set(data)-{'SHA256SUMS'}, 'incomplete checksum inventory')
    require(all(digest(data[n]) == sha for n, sha in sums.items()), 'payload checksum mismatch')
    raw = data[binary]
    if target == 'windows-amd64':
        require(raw[:2] == b'MZ', 'wrong Windows binary')
        offset = int.from_bytes(raw[60:64], 'little')
        require(raw[offset:offset+6] == b'PE\0\0\x64\x86', 'wrong Windows architecture')
    elif target.startswith('darwin-'):
        require(raw[:4] == b'\xcf\xfa\xed\xfe', 'wrong Darwin binary')
        require(int.from_bytes(raw[4:8], 'little') == (0x100000c if target.endswith('arm64') else 0x1000007), 'wrong Darwin architecture')
    else:
        require(target == 'linux-amd64' and raw[:5] == b'\x7fELF\x02' and int.from_bytes(raw[18:20], 'little') == 62, 'wrong Linux binary')
    if target != 'windows-amd64':
        require(all(mode == (0o755 if n == binary else 0o644) for n, mode in modes.items()), 'Unix executable/payload mode')
    print('READY_KIT_INVENTORY_OK', target, len(data), 'binarySHA256='+digest(raw))
    return data, binary, info


def native_request(binary, root, action, payload, expected=True):
    request = dict(schemaVersion=1, action=action, projectRoot=str(root), actor='main', expectedRevision=0,
                   requestId=str(uuid.uuid4()), payload=payload)
    # No compiler/interpreter on PATH; the prebuilt core must stand alone.
    env = {k: v for k, v in os.environ.items() if k.upper() in ('SYSTEMROOT', 'WINDIR', 'TEMP', 'TMP', 'TMPDIR')}
    env['PATH'] = ''
    p = subprocess.run([str(binary), 'request'], input=json.dumps(request), text=True,
                       encoding='utf-8', env=env, capture_output=True, timeout=35)
    result = json.loads(p.stdout)
    require(result['ok'] is expected and p.returncode == (0 if expected else 2), action+' outcome: '+result['code'])
    return result['data']


def file_snapshot(root):
    return {p.relative_to(root).as_posix(): p.read_bytes() for p in root.rglob('*') if p.is_file()}


def exercise_memory(binary, home, scratch, commit):
    base = pathlib.Path(tempfile.mkdtemp(prefix='ready-memory-', dir=scratch))
    (base/'OWNERSHIP.json').write_text(json.dumps(dict(owner='ready-kit consumer', purpose='Fictional selected-memory file recovery', retainUntil='recovery acceptance', scope='native files and prompt rendering; no actual UI')))
    old = base/'old device'; memory = old/'system/.local/coordinator'; memory.mkdir(parents=True)
    system = home/'system'
    template = (system/'templates/coordinator/STARTUP.md').read_text(encoding='utf-8-sig')
    tokens = ('SYSTEM_PATH', 'COORDINATOR_MEMORY_PATH', 'SOURCE_REVISION', 'TEMPLATE_LANGUAGE', 'COMMUNICATION_LANGUAGE', 'CLIENT_PROJECT')
    def prompt(values):
        text = template
        for key, value in zip(tokens, values):
            require('<'+key+'>' in text, 'startup template token missing: '+key)
            text = text.replace('<'+key+'>', value)
        require(not any('<'+k+'>' in text for k in tokens), 'unfilled startup prompt')
        return text
    saved = {'AGENTS.md': (system/'templates/coordinator/AGENTS.md').read_bytes(),
             'memory/INDEX.md': b'Only this service area. See BRIEF.md and tasks/main/HANDOFF.md.\n',
             'memory/BRIEF.md': b'Confirmed fixture goal: preserve selected context. Unknown fixture operation remains unresolved.\n',
             'memory/tasks/main/HANDOFF.md': b'Historical source device; STOP retained. Do not relaunch fixture operation.\n',
             'history/decision.md': b'Unique original decision, not only a summary.\n',
             'material.bin': b'\x00\xffunique-original\r\n',
             'STARTUP.md': prompt((str(old/'system'), str(memory), 'fictional-previous-source', 'en', 'English', 'old fixture project')).encode()}
    for name, raw in saved.items():
        p = memory/name; p.parent.mkdir(parents=True, exist_ok=True); p.write_bytes(raw)
    (memory/'unselected.txt').write_text('UNSELECTED_MATERIAL')
    sibling = old/'other personal topic'; sibling.mkdir(); (sibling/'private.md').write_text('UNSELECTED_SIBLING')
    transport = base/'selected transport'; transport.mkdir(); package = transport/'coordinator-checkpoint'
    payload = dict(packagePath=str(package), checkpointId='fictional-coordinator', systemVersion='1.3.5',
                   sourceIdentity='fictional-previous-source', owner='fixture-main', purpose='Selected service restoration',
                   retainUntil='recovery acceptance', nextAction='Confirm new device binding; retain STOP and unknown operation.',
                   files=list(saved), references=[])
    before = file_snapshot(memory)
    result = native_request(binary, memory, 'checkpoint.capture', payload)
    require(result['readyToResume'] is False and file_snapshot(memory) == before, 'capture mutated or authorized source')
    require(not any(b'UNSELECTED_' in raw for raw in file_snapshot(package).values()), 'unselected scope exported')
    old.rename(base/'old device unavailable')
    destination = base/'new device system/.local/coordinator'; destination.parent.mkdir(parents=True); destination.mkdir()
    inspected = native_request(binary, base/'inspection scope', 'checkpoint.inspect', dict(packagePath=str(package)))
    require(inspected['readyToResume'] is False, 'inspection became readiness')
    result = native_request(binary, destination, 'checkpoint.restore', dict(packagePath=str(package)))
    require(result['state'] == 'requires_rebind' and result['readyToResume'] is False, 'restore imported binding authority')
    require(file_snapshot(destination) == saved, 'selected context/original bytes lost')
    # Render only the new prompt; this fixture does not assert real UI binding/trust.
    (destination/'history/STARTUP-old.md').write_bytes(saved['STARTUP.md'])
    fresh = prompt((str(system), str(destination), commit, 'en', 'English', 'new fixture project; binding pending'))
    (destination/'STARTUP.md').write_text(fresh, encoding='utf-8')
    require(str(old) not in fresh and 'binding pending' in fresh, 'old machine binding replayed')
    occupied = file_snapshot(destination)
    native_request(binary, destination, 'checkpoint.restore', dict(packagePath=str(package)), False)
    require(file_snapshot(destination) == occupied, 'repeat restore overwrote current memory')
    broken = transport/'incomplete checkpoint'; shutil.copytree(package, broken); (broken/'COMPLETE').unlink()
    native_request(binary, system, 'checkpoint.inspect', dict(packagePath=str(broken)), False)
    corrupt = transport/'corrupt checkpoint'; shutil.copytree(package, corrupt)
    (corrupt/'data/material.bin').write_bytes(b'changed')
    native_request(binary, system, 'checkpoint.inspect', dict(packagePath=str(corrupt)), False)
    # A topic remains file-only and scoped, with no task database requirement.
    topic = base/'one selected topic'; topic.mkdir(); (topic/'HANDOFF.md').write_bytes(b'Continue only this topic.\n')
    topic_package = transport/'one topic checkpoint'
    topic_payload = dict(payload, packagePath=str(topic_package), checkpointId='fictional-topic', files=['HANDOFF.md'])
    native_request(binary, topic, 'checkpoint.capture', topic_payload)
    restored = base/'new selected topic'; restored.mkdir()
    native_request(binary, restored, 'checkpoint.restore', dict(packagePath=str(topic_package)))
    require(file_snapshot(restored) == {'HANDOFF.md': b'Continue only this topic.\n'} and not (restored/'.ocmi').exists(), 'topic became a work database or mixed scope')
    print('READY_KIT_MEMORY_OK native standalone core, selected originals, missing source, corruption/conflict refusal, no old authority; UI binding pending')
    print('READY_KIT_FIXTURE_RETAINED='+str(base))


def unpack_factory(data, binary_name, scratch, label):
    home = pathlib.Path(tempfile.mkdtemp(prefix=label, dir=scratch))
    for name, raw in data.items():
        dest = home/name; dest.parent.mkdir(parents=True, exist_ok=True); dest.write_bytes(raw)
        if name == binary_name: dest.chmod(0o755)
    return home


def exercise_install_home(binary, home, language):
    checker = pathlib.Path(__file__).with_name('check_install_home.py')
    command = [__import__('sys').executable, '-B', str(checker), '--binary', str(binary),
               '--home', str(home), '--language', language]
    result = subprocess.run(command, text=True, encoding='utf-8', capture_output=True, timeout=120)
    require(result.returncode == 0, 'final home native consumer: '+result.stdout[-512:]+result.stderr[-512:])
    print(result.stdout, end='')


def main():
    p = argparse.ArgumentParser()
    p.add_argument('--archive', required=True); p.add_argument('--source-root', required=True)
    p.add_argument('--source-commit', required=True); p.add_argument('--target', required=True)
    p.add_argument('--exercise-recovery', action='store_true'); p.add_argument('--scratch-root')
    a = p.parse_args()
    data, binary_name, info = check_archive(pathlib.Path(a.archive), pathlib.Path(a.source_root), a.source_commit, a.target)
    if a.exercise_recovery:
        actual = {'win32': 'windows', 'darwin': 'darwin', 'linux': 'linux'}[__import__('sys').platform]
        arch = 'arm64' if platform.machine().lower() in ('arm64', 'aarch64') else 'amd64'
        require(a.target == actual+'-'+arch, 'cannot qualify a foreign target by execution')
        scratch = pathlib.Path(a.scratch_root).resolve(); require(scratch.is_dir(), 'owned scratch required')
        home = unpack_factory(data, binary_name, scratch, 'ready-home-en-')
        russian_home = unpack_factory(data, binary_name, scratch, 'ready-home-ru-')
        binary = (home/binary_name).resolve()
        exercise_install_home(binary, home, 'en')
        exercise_install_home((russian_home/binary_name).resolve(), russian_home, 'ru')
        exercise_memory(binary, home, scratch, info['sourceCommit'])
        print('READY_KIT_SYSTEM_RETAINED='+str(home))


if __name__ == '__main__':
    main()
