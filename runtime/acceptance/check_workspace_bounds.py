"""Independent native output-size/count consumer; bounded public outcome markers."""
import argparse
import json
from pathlib import Path
import subprocess
import sys

sys.path.insert(0,str(Path(__file__).resolve().parents[2]/'scripts/acceptance'))
from check_install_home import install_fixture
from check_cli import Client
from check_lifecycle import binding
from check_workspace_recovery import check as check_recovery


def check(binary, controls=False, legacy_binary=None):
    with install_fixture(preserve_on_failure=True) as directory:
        base=Path(directory)
        root=base/'selected fictional output';root.mkdir()
        (root/'contract.md').write_text('Create and close only original supervised scratch.',encoding='utf-8')
        (root/'input.txt').write_text('fixed',encoding='utf-8')
        (root/'foreign.txt').write_bytes(b'foreign input')
        c=Client(binary,root)
        c.request('init',dict(projectId='workspace-bounds',deviceId='device',copyId='copy',owner='main'))
        child='''import pathlib,sys,tempfile,shutil
root=pathlib.Path(sys.argv[1]);scratch=pathlib.Path(tempfile.gettempdir())
shutil.copyfile(sys.argv[2],scratch/'native-artifact.bin')
with (scratch/'native-artifact.bin').open('ab') as stream:
 if stream.tell()<21<<20: stream.truncate(21<<20)
for n in range(292): (scratch/('part-%03d.txt'%n)).write_bytes(b'fictional scratch')
(root/'scratch.txt').write_text(str(scratch),encoding='utf-8')
'''
        compile(child,'<actual native output>','exec')
        p=binding(root)
        p['command']=dict(id='large-output',executable=sys.executable,args=['-c',child,str(root),binary],cwd='.',timeoutMs=15000,outputLimitBytes=4096)
        c.request('task.bind',p);c.request('task.ready')
        result=c.request('run',actor='worker-1')
        assert result['operationState']=='succeeded',result
        scratch=Path((root/'scratch.txt').read_text('utf-8'))
        assert scratch.resolve().is_relative_to((root/'.ocmi/local/runs').resolve())
        state=c.request('status')
        if controls:
            assert scratch.exists() and (scratch/'native-artifact.bin').stat().st_size>=21<<20
            assert len(list(scratch.glob('part-*.txt')))==292
            assert state['cleanup']=='pending' and state['resourceSummary']['pending']>0,state
            print('WORKSPACE_BOUNDS_BASELINE_OK actual successful binary+292files exceeds old proof bounds',flush=True)
        else:
            assert not scratch.parent.exists(),'successful original owned output not removed'
            assert state['resourceSummary']['pending']==0 and state['resourceSummary']['activeUses']==0,state
            c.request('submit',actor='worker-1')
            c.request('review',dict(accept=True,reason='Actual qualified output closed'))
            c.request('integrate',dict(candidate='reviewed-fixture',reason='Verified owned cleanup'))
            c.request('resource.close',dict(disposition='keep',reason='Keep bounded operation evidence'))
            assert c.request('status')['cleanup']=='done'
            print('WORKSPACE_BOUNDS_NORMAL_OK actual21MiB artifact+292files, one registered group, successful physical cleanup',flush=True)
        assert (root/'foreign.txt').read_bytes()==b'foreign input'
    assert not base.exists(),'successful qualification fixture retained'
    check_recovery(binary,False,artifact_source=binary,expect_supported=not controls,legacy_binary=legacy_binary)
    print('WORKSPACE_BOUNDS_ALL_OK' if not controls else 'WORKSPACE_BOUNDS_CONTROLS_OK',flush=True)


if __name__=='__main__':
    parser=argparse.ArgumentParser()
    parser.add_argument('--binary',required=True)
    parser.add_argument('--legacy-binary')
    parser.add_argument('--controls',action='store_true')
    args=parser.parse_args()
    check(str(Path(args.binary).resolve(strict=True)),args.controls,
          str(Path(args.legacy_binary).resolve(strict=True)) if args.legacy_binary else None)
