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
import time
import uuid
import zipfile
from contextlib import contextmanager


ROOT_MEMBERS = {'.ocmi-home.json', 'README.md', 'README.ru.md', 'AGENTS.md',
                '.gitignore', '.gitattributes', 'SHA256SUMS'}
UPDATE_COMPATIBILITY = dict(schemaVersion=1, layoutVersion=1, installationReceiptVersion=1,
                            coordinatorMemoryVersion=1, coordinatorBindingVersion=1, stateSchemaVersion=4)


def digest(raw):
    return hashlib.sha256(raw).hexdigest()


def require(value, message):
    if not value:
        raise AssertionError(message)


def fixture_record(root, purpose, retention):
    (root/'OWNERSHIP.json').write_text(json.dumps(dict(
        owner='ready-kit consumer', purpose=purpose, retention=retention,
        scope='self-created fictional native qualification fixture only'), indent=2)+'\n', encoding='utf-8')


def linked_path(path):
    attributes = path.stat(follow_symlinks=False).st_file_attributes if os.name == 'nt' else 0
    return path.is_symlink() or bool(attributes & stat.FILE_ATTRIBUTE_REPARSE_POINT)


def cleanup_fixture(root, identity):
    for attempt in range(4):
        if not root.exists():
            return
        current = root.stat(follow_symlinks=False)
        require(not linked_path(root) and (current.st_dev, current.st_ino) == identity,
                'owned ready-kit fixture root changed; preserve for review')
        for parent, directories, files in os.walk(root, followlinks=False):
            for name in directories + files:
                candidate = pathlib.Path(parent)/name
                require(not linked_path(candidate), 'owned ready-kit fixture contains link; preserve for review')
        try:
            shutil.rmtree(root)
            return
        except OSError as error:
            if os.name != 'nt' or getattr(error, 'winerror', None) not in (5, 32, 33) or attempt == 3:
                raise
            print('READY_KIT_CLEANUP_WAIT windows access/sharing denial; bounded retry', attempt+1)
            time.sleep((0.1, 0.2, 0.4)[attempt])


@contextmanager
def owned_fixture(scratch, label, purpose):
    root = pathlib.Path(tempfile.mkdtemp(prefix=label, dir=scratch))
    identity_stat = root.stat(follow_symlinks=False)
    identity = (identity_stat.st_dev, identity_stat.st_ino)
    fixture_record(root, purpose, 'remove automatically after successful verification; retain unchanged after failure or cleanup refusal')
    try:
        yield root
    except BaseException as error:
        print('READY_KIT_FIXTURE_RETAINED='+str(root)+' reason='+type(error).__name__)
        raise
    else:
        try:
            cleanup_fixture(root, identity)
        except BaseException as error:
            print('READY_KIT_FIXTURE_RETAINED='+str(root)+' cleanupReason='+type(error).__name__)
            raise
        print('READY_KIT_FIXTURE_DISPOSED='+str(root))


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


def check_archive(path, source, commit, target, legacy=False):
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
    require(info.get('updateCompatibility') == UPDATE_COMPATIBILITY or (legacy and 'updateCompatibility' not in info), 'update compatibility')
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
    with owned_fixture(scratch, 'ready-memory-', 'Fictional selected-memory recovery') as base:
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


def exercise_existing_pair(previous_archive, previous_source, previous_commit, target, candidate_data, binary_name, scratch):
    previous_data, previous_binary, _ = check_archive(pathlib.Path(previous_archive), pathlib.Path(previous_source), previous_commit, target, legacy=True)
    require(previous_binary == binary_name, 'previous/current binary path differs')
    with owned_fixture(scratch, 'ready-update-pair-', 'Actual previous/current update pair') as fixture:
        previous_home = unpack_factory(previous_data, previous_binary, fixture, 'ready-home-previous-')
        candidate_home = unpack_factory(candidate_data, binary_name, fixture, 'ready-home-candidate-')
        import check_update_home
        check_update_home.exercise_existing_pair(previous_home, candidate_home)


def main():
    p = argparse.ArgumentParser()
    p.add_argument('--archive', required=True); p.add_argument('--source-root', required=True)
    p.add_argument('--source-commit', required=True); p.add_argument('--target', required=True)
    p.add_argument('--exercise-recovery', action='store_true'); p.add_argument('--scratch-root')
    p.add_argument('--previous-archive'); p.add_argument('--previous-source-root'); p.add_argument('--previous-source-commit')
    a = p.parse_args()
    require(all((a.previous_archive, a.previous_source_root, a.previous_source_commit)) or not any((a.previous_archive, a.previous_source_root, a.previous_source_commit)), 'complete previous factory inputs required')
    data, binary_name, info = check_archive(pathlib.Path(a.archive), pathlib.Path(a.source_root), a.source_commit, a.target)
    if a.exercise_recovery:
        actual = {'win32': 'windows', 'darwin': 'darwin', 'linux': 'linux'}[__import__('sys').platform]
        arch = 'arm64' if platform.machine().lower() in ('arm64', 'aarch64') else 'amd64'
        require(a.target == actual+'-'+arch, 'cannot qualify a foreign target by execution')
        scratch = pathlib.Path(a.scratch_root).resolve(); require(scratch.is_dir(), 'owned scratch required')
        if a.previous_archive:
            exercise_existing_pair(a.previous_archive, a.previous_source_root, a.previous_source_commit, a.target, data, binary_name, scratch)
        with owned_fixture(scratch, 'ready-current-homes-', 'Actual current English/Russian installed homes') as fixture:
            home = unpack_factory(data, binary_name, fixture, 'ready-home-en-')
            russian_home = unpack_factory(data, binary_name, fixture, 'ready-home-ru-')
            binary = (home/binary_name).resolve()
            exercise_install_home(binary, home, 'en')
            exercise_install_home((russian_home/binary_name).resolve(), russian_home, 'ru')
            # Exercise the packaged executable, not a developer-built binary or
            # an external context parser. The harness selects only fictional files.
            import sys
            sys.path.insert(0, str(pathlib.Path(__file__).resolve().parents[2]/'runtime/acceptance'))
            import check_context
            context_scope = fixture/'selected-context'
            context_scope.mkdir()
            check_context.exercise(binary, context_scope)
            # Independently exercise ordinary task execution through this exact
            # packaged executable. Go is a qualification-tool prerequisite only.
            tool_scratch = fixture/'tool-storage-qualification'
            tool_scratch.mkdir()
            tool_env = dict(os.environ, OCMI_ROOT_BINARY=str(binary),
                            GOTMPDIR=str(tool_scratch), TEMP=str(tool_scratch),
                            TMP=str(tool_scratch), TMPDIR=str(tool_scratch))
            result = subprocess.run(['go', '-C', str(pathlib.Path(a.source_root).resolve()/'runtime'),
                                     'test', '-mod=readonly', '-count=1', '-timeout=90s',
                                     '-v', '-run', '^TestMain(Tool(StorageNativeCLI|AuthoringNativeCLI(Probes)?|AuthoringRelocationNativeCLI|AuthoringMaintenanceNativeCLI)|ErgonomicsNativeCLI|BorrowedHistoryNativeCLI|Compact(Metadata|Text|TextNoEffects)?NativeCLI|Workflow(Profile|BoundConfig)?NativeCLI)$', './internal'],
                                    env=tool_env, capture_output=True, text=True,
                                    encoding='utf-8', timeout=180)
            print(result.stdout[-16384:])
            if result.stderr:
                print(result.stderr[-16384:])
            require(result.returncode == 0 and
                    all('--- PASS: ' + name + ' (' in result.stdout for name in (
                        'TestMainToolStorageNativeCLI', 'TestMainToolAuthoringNativeCLI',
                        'TestMainToolAuthoringNativeCLIProbes', 'TestMainToolAuthoringRelocationNativeCLI',
                        'TestMainToolAuthoringMaintenanceNativeCLI', 'TestMainErgonomicsNativeCLI',
                        'TestMainBorrowedHistoryNativeCLI', 'TestMainWorkflowNativeCLI',
                        'TestMainWorkflowProfileNativeCLI', 'TestMainWorkflowBoundConfigNativeCLI', 'TestMainCompactNativeCLI', 'TestMainCompactMetadataNativeCLI',
                        'TestMainCompactTextNativeCLI', 'TestMainCompactTextNoEffectsNativeCLI')),
                    'packaged tool-storage authoring/ordinary-run consumer failed or did not execute')
            print('READY_KIT_TOOL_STORAGE_OK actual packaged ordinary-run profile consumer')
            print('READY_KIT_TOOL_AUTHORING_OK actual packaged coordinator and Go/npm provider consumers')
            print('READY_KIT_TOOL_RELOCATION_OK actual packaged preserving native-profile transition and reverse')
            print('READY_KIT_TOOL_MAINTENANCE_OK actual packaged bounded read-only cache assessment')
            print('READY_KIT_ERGONOMICS_OK actual packaged help/call/registered-output consumer')
            print('READY_KIT_BORROWED_HISTORY_OK actual packaged closed-loan history consumer')
            print('READY_KIT_WORKFLOW_OK actual packaged derived START and published profile schema consumer')
            print('READY_KIT_COMPACT_CONTEXT_OK actual packaged stateless retained-context receipt consumer')
            print('READY_KIT_CONTEXT_TEXT_OK actual packaged complete read-only text consumer and pre-effect format refusals')
            exercise_memory(binary, home, scratch, info['sourceCommit'])


if __name__ == '__main__':
    main()
