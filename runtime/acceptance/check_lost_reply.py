"""Main-owned native lost-reply oracle; first committed stdout is never decoded."""
import argparse
import json
from pathlib import Path
import subprocess
import sys

sys.path.insert(0, str(Path(__file__).resolve().parents[2] / 'scripts/acceptance'))
from check_install_home import install_fixture
from check_cli import scenario, decode
from check_resources import declare, record


def check(binary, controls=False):
    for action in ('run', 'resource.apply'):
        for explicit in (True, False):
            with install_fixture(preserve_on_failure=True) as temp:
                c = scenario(binary, Path(temp) / 'project')
                payload = {}
                if action == 'resource.apply':
                    declare(c, 'lost-output', 'artifact')
                    payload = {'resourceId': 'lost-output'}
                args = [binary, 'call', '--root', str(c.root), '--actor', 'worker-1' if action == 'run' else 'main',
                        '--action', action, '--revision', str(c.revision), '--payload', json.dumps(payload)]
                if explicit:
                    args += ['--request-id', 'retained-before-dispatch']
                # Keep the exact command/revision before launch; discard the whole reply.
                first = subprocess.run(args, stdout=subprocess.DEVNULL, stderr=subprocess.PIPE, timeout=20)
                assert first.returncode == 0, (action, first.returncode)
                if action == 'run':
                    observed = c.request('status')['operationId']
                    assert (c.root / 'effect.txt').read_text() == 'once\n'
                else:
                    observed = record(c, 'lost-output')['nativeIdentity']
                    assert (c.root / 'artifact').is_dir()
                replay = decode(subprocess.run(args, capture_output=True, text=True, encoding='utf-8', timeout=20))
                if controls and not explicit:
                    assert not replay['ok'] and replay['code'] == 'STALE_REVISION', replay
                else:
                    assert replay['ok'], ('lost committed reply not recovered', action, replay)
                    assert replay['data']['requestId'], replay
                    if action == 'run':
                        assert replay['data']['operationId'] == observed, replay
                    else:
                        assert record(c, 'lost-output')['nativeIdentity'] == observed
                    again = decode(subprocess.run(args, capture_output=True, text=True, encoding='utf-8', timeout=20))
                    assert again == replay, 'exact replay changed original receipt'
                if action == 'run':
                    assert (c.root / 'effect.txt').read_text() == 'once\n', 'effect executed again'
                else:
                    assert record(c, 'lost-output')['nativeIdentity'] == observed, 'resource recreated'
                print('LOST_REPLY_CONTROL_OK' if controls else 'LOST_REPLY_NATIVE_OK', action,
                      'explicit' if explicit else 'derived', flush=True)
    print('LOST_REPLY_BASELINE_CONFIRMED' if controls else 'LOST_REPLY_ALL_OK exact retained command, same receipt, one effect', flush=True)


if __name__ == '__main__':
    p = argparse.ArgumentParser()
    p.add_argument('--binary', required=True)
    p.add_argument('--controls', action='store_true')
    args = p.parse_args()
    check(str(Path(args.binary).resolve(strict=True)), args.controls)
