"""Main-owned native rollback oracle: original proof failures, no runtime imports."""
import argparse
import ctypes
import json
import os
from pathlib import Path
import re
import subprocess

import check_install_home as install
import check_update_home as update


def rollback(tool, home):
    result = subprocess.run([str(tool), 'update', 'rollback', '--home', str(home)],
                            capture_output=True, text=True, encoding='utf-8', timeout=30)
    response = json.loads(result.stdout)
    assert result.returncode == 2 and response['ok'] is False, (result.returncode, response)
    assert response['code'] == 'UPDATE_REFUSED', response
    assert response['data']['reason'] == 'provenance' and response['data']['stage'] == 'recovery', response
    return response


def lock_read(path):
    from ctypes import wintypes
    kernel = ctypes.WinDLL('kernel32', use_last_error=True)
    kernel.CreateFileW.argtypes = [wintypes.LPCWSTR, wintypes.DWORD, wintypes.DWORD,
                                   ctypes.c_void_p, wintypes.DWORD, wintypes.DWORD, wintypes.HANDLE]
    kernel.CreateFileW.restype = wintypes.HANDLE
    kernel.CloseHandle.argtypes = [wintypes.HANDLE]
    kernel.CloseHandle.restype = wintypes.BOOL
    handle = kernel.CreateFileW(str(path), 0x80000000, 0, None, 3, 0x80, None)
    assert handle != ctypes.c_void_p(-1).value, ('sharing-lock control failed', ctypes.get_last_error())
    return lambda: kernel.CloseHandle(handle)


def check(binary, controls=False):
    cases = ['content', 'identity', 'metadata'] + (['io'] if os.name == 'nt' else [])
    for case in cases:
        fixture_options = {} if controls else {'preserve_on_failure': True}
        with install.install_fixture(**fixture_options) as scratch:
            root = Path(scratch)
            home = update.factory(binary, root / 'home', 'a' * 40)
            candidate = update.factory(binary, root / 'candidate', 'b' * 40)
            tool = install.native_path(candidate)
            before = update.setup(home)
            private = home / 'OCMI_Sync/personal/topic/WIP.txt'
            private.write_bytes(b'PRIVATE-DIAGNOSTIC-CANARY-DO-NOT-LOG\n')
            before['sync'] = install.snapshot(home / 'OCMI_Sync')
            assert update.invoke(tool, 'apply', home, '--candidate', candidate,
                                 '--expected-source', 'a' * 40, '--expected-candidate', 'b' * 40)['phase'] == 'applied'
            journal_path = home / 'local/bindings/update.json'
            journal_before = journal_path.read_bytes()
            journal = json.loads(journal_before)
            original = (home / 'README.md').read_bytes()
            startup = home / 'local/bindings/coordinator/STARTUP.md'
            original_startup = startup.read_bytes()
            if case == 'content':
                (home / 'README.md').write_bytes(b'Fictional changed public content.\n')
            elif case == 'identity':
                target = home / 'README.md'
                old_id = target.stat().st_ino
                mode = target.stat().st_mode & 0o777
                replacement = home / 'replacement.txt'
                replacement.write_bytes(original)
                replacement.chmod(mode)
                os.replace(replacement, target)
                assert target.stat().st_ino != old_id, 'native replacement premise failed'
            elif case == 'metadata':
                startup.write_bytes(b'Fictional changed startup metadata.\n')
            snapshot_before = install.snapshot(home)
            close = lock_read(home / journal['stage'] / 'old/system/VERSION.md') if case == 'io' else None
            try:
                response = rollback(tool, home)
            finally:
                if close:
                    assert close(), 'sharing lock did not close'
            assert install.snapshot(home) == snapshot_before, 'before-admission refusal changed selected fixture'
            assert journal_path.read_bytes() == journal_before, 'refusal rewrote journal'
            assert install.snapshot(home / 'OCMI_Sync') == before['sync'], 'private WIP changed'
            assert update.rows(home) == before['rows'], 'state changed'
            diagnostic = response['data'].get('diagnostic')
            if controls:
                assert diagnostic is None, 'baseline has unexpected diagnostic; use strict mode'
            else:
                assert isinstance(diagnostic, dict), ('missing diagnostic', case, response)
                assert diagnostic['boundary'] == 'before_admission', diagnostic
                expected_validator = 'recovery_start.metadata_placement' if case == 'metadata' else 'recovery_start.root_placement'
                assert diagnostic['validator'] == expected_validator, (case, diagnostic)
                assert isinstance(diagnostic['predicate'], str) and diagnostic['predicate'], diagnostic
                assert diagnostic['kind'] == ('io_error' if case == 'io' else 'mismatch'), (case, diagnostic)
                if case in ('content', 'identity'):
                    assert case in diagnostic['predicate'], (case, diagnostic)
                if case == 'io':
                    assert diagnostic['operation'] and type(diagnostic['osCode']) is int and diagnostic['osCode'] > 0, diagnostic
                assert diagnostic['phase'] == 'applied' and type(diagnostic['recoveryStep']) is int, diagnostic
                serialized = json.dumps(diagnostic)
                assert len(serialized.encode()) <= 4096, 'unbounded diagnostic'
                def texts(value):
                    if isinstance(value, dict):
                        return list(value) + [s for v in value.values() for s in texts(v)]
                    if isinstance(value, list):
                        return [s for v in value for s in texts(v)]
                    return [value] if isinstance(value, str) else []
                values = texts(diagnostic)
                for forbidden in (str(home), str(root), 'PRIVATE-DIAGNOSTIC-CANARY', original.decode().strip(), 'Unique fictional bytes:'):
                    assert all(forbidden not in value for value in values), 'diagnostic leaked fixture paths/content'
                assert all(re.fullmatch(r'[A-Za-z0-9_.-]{1,80}', value) for value in values), 'non-enum diagnostic string'
            if case in ('content', 'metadata', 'io'):
                if case == 'content':
                    (home / 'README.md').write_bytes(original)
                elif case == 'metadata':
                    startup.write_bytes(original_startup)
                assert update.invoke(tool, 'rollback', home)['phase'] == 'rolled_back', 'known-correct restoration control failed'
                update.preserved(home, before, 'a' * 40)
            print('UPDATE_DIAGNOSTICS_CONTROL_OK' if controls else 'UPDATE_DIAGNOSTICS_NATIVE_OK', case)
    print('UPDATE_DIAGNOSTICS_BASELINE_MISSING confirmed' if controls else 'UPDATE_DIAGNOSTICS_ALL_OK actual native predicates, privacy and unchanged guard decisions')


if __name__ == '__main__':
    parser = argparse.ArgumentParser()
    parser.add_argument('--binary', type=Path, required=True)
    parser.add_argument('--controls', action='store_true')
    args = parser.parse_args()
    check(args.binary.resolve(strict=True), args.controls)
