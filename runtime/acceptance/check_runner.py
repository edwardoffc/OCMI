"""Real subprocess output/deadline acceptance; reused across all native targets."""
import argparse
import pathlib
import subprocess
import sys
import tempfile
import time

from check_cli import Client


def setup(binary, root, code, timeout=1000, limit=32):
    root.mkdir()
    c = Client(binary, root)
    c.request('init', dict(projectId='p', deviceId='d', copyId='c', owner='main'))
    (root/'contract.md').write_text('synthetic runner check', encoding='utf-8')
    (root/'input.txt').write_text('fixed', encoding='utf-8')
    c.request('task.bind', dict(taskId='t', outcomeId='o', taskRevision=1, completionScope="target", worker='worker', reviewer='main',
        contract='contract.md', inputs=['input.txt'], command=dict(id='run', executable=sys.executable,
        args=['-c', code], cwd='.', timeoutMs=timeout, outputLimitBytes=limit)))
    c.request('task.ready')
    return c


def check(binary):
    with tempfile.TemporaryDirectory(prefix='ocmi-runner-') as tmp:
        base = pathlib.Path(tmp)
        c = setup(binary, base/'large-output', "import sys;sys.stdout.write('x'*100000)")
        result = c.request('run', actor='worker')
        assert result['operationState'] == 'incomplete', result
        assert c.request('status')['nextAction'] == 'diagnose_and_revise'
        c.request('submit', actor='worker', ok=False)
        print('PASS: actual large output is incomplete and cannot be submitted')
        foreign = subprocess.Popen([sys.executable, '-c', 'import time;time.sleep(20)'],
                                   stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
        try:
            code = "import subprocess,sys;subprocess.Popen([sys.executable,'-c','import time;time.sleep(30)'])"
            c = setup(binary, base/'inherited-pipe', code, timeout=100)
            started = time.monotonic()
            result = c.request('run', actor='worker')
            elapsed = time.monotonic()-started
            assert elapsed < 8, ('deadline plus safe-close exceeded', elapsed)
            assert result['operationState'] == 'incomplete', result
            assert foreign.poll() is None, 'foreign process killed'
            c.request('submit', actor='worker', ok=False)
            print('PASS: inherited pipe deadline is bounded; foreign control survives')
        finally:
            if foreign.poll() is None:
                foreign.terminate()
            foreign.wait(timeout=5)


if __name__ == '__main__':
    parser = argparse.ArgumentParser()
    parser.add_argument('--binary', required=True)
    args = parser.parse_args()
    check(str(pathlib.Path(args.binary).resolve(strict=True)))
