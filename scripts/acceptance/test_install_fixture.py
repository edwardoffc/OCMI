"""Real filesystem controls for synthetic fixture retention; no updater mocking."""
from contextlib import redirect_stdout
import gc
import hashlib
import io
import json
import os
from pathlib import Path
import shutil
import stat
import subprocess
import sys
import tempfile
import unittest

import check_install_home as install
import check_update_home as update


class FixtureRetentionTests(unittest.TestCase):
    def setUp(self):
        self.workspace=Path(tempfile.mkdtemp(prefix='fixture-control-owner-'))
        self.identity=self.workspace.stat(follow_symlinks=False)
        self.previous_temp=tempfile.tempdir
        tempfile.tempdir=str(self.workspace)

    def tearDown(self):
        update._report_context.clear()
        tempfile.tempdir=self.previous_temp
        current=self.workspace.stat(follow_symlinks=False)
        self.assertEqual((current.st_dev,current.st_ino),(self.identity.st_dev,self.identity.st_ino))
        self.assertTrue(stat.S_ISDIR(current.st_mode))
        # All children here are explicitly created synthetic control data.
        shutil.rmtree(self.workspace)

    def test_success_cleans_default_and_update_namespace(self):
        for preserve in (False,True):
            with install.install_fixture(preserve_on_failure=preserve) as name:
                root=Path(name)
                self.assertTrue(root.name.startswith('ocmi-update-home-' if preserve else 'ocmi-install-home-'))
                (root/'synthetic.txt').write_bytes(b'owned success')
            self.assertFalse(root.exists())

    def test_update_failure_retains_same_root_and_original_exception(self):
        original=AssertionError('original synthetic consumer assertion')
        with self.assertRaises(AssertionError) as caught:
            with install.install_fixture(preserve_on_failure=True) as name:
                root=Path(name)
                identity=root.stat()
                (root/'synthetic.txt').write_bytes(b'original failure evidence')
                raise original
        self.assertIs(caught.exception,original)
        gc.collect()
        self.assertEqual((root.stat().st_dev,root.stat().st_ino),(identity.st_dev,identity.st_ino))
        self.assertEqual((root/'synthetic.txt').read_bytes(),b'original failure evidence')

    def test_default_failure_still_cleans_without_masking(self):
        original=AssertionError('original default consumer assertion')
        with self.assertRaises(AssertionError) as caught:
            with install.install_fixture() as name:
                root=Path(name)
                raise original
        self.assertIs(caught.exception,original)
        self.assertFalse(root.exists())

    def test_replaced_root_has_no_deleting_finalizer(self):
        with self.assertRaisesRegex(RuntimeError,'root changed'):
            with install.install_fixture() as name:
                root=Path(name)
                parked=self.workspace/'parked'
                (root/'original.txt').write_bytes(b'original')
                root.rename(parked)
                root.mkdir()
                (root/'replacement.txt').write_bytes(b'replacement')
        gc.collect()
        self.assertEqual((root/'replacement.txt').read_bytes(),b'replacement')
        self.assertEqual((parked/'original.txt').read_bytes(),b'original')

    def test_replaced_root_preserves_original_body_exception(self):
        for preserve in (False,True):
            original=AssertionError('original replaced-root assertion')
            with self.assertRaises(AssertionError) as caught:
                with install.install_fixture(preserve_on_failure=preserve) as name:
                    root=Path(name)
                    root.rename(self.workspace/('parked-'+str(preserve)))
                    root.mkdir()
                    (root/'replacement.txt').write_bytes(b'replacement')
                    raise original
            self.assertIs(caught.exception,original)
            gc.collect()
            self.assertEqual((root/'replacement.txt').read_bytes(),b'replacement')

    def test_linked_root_preserves_target_after_collection(self):
        target=self.workspace/'link-target'
        target.mkdir()
        (target/'synthetic.txt').write_bytes(b'linked target')
        try:
            probe=self.workspace/'link-probe'
            probe.symlink_to(target,target_is_directory=True)
            probe.unlink()
        except OSError as error:
            self.skipTest('native directory symlink unavailable: '+type(error).__name__)
        try:
            with self.assertRaisesRegex(RuntimeError,'root changed'):
                with install.install_fixture() as name:
                    root=Path(name)
                    root.rename(self.workspace/'parked-link')
                    root.symlink_to(target,target_is_directory=True)
            gc.collect()
            self.assertTrue(root.is_symlink())
            self.assertEqual((target/'synthetic.txt').read_bytes(),b'linked target')
        finally:
            if 'root' in locals() and root.is_symlink():
                root.unlink()

    def test_journal_observation_is_read_only_and_bounded_metadata(self):
        home=self.workspace/'home'
        path=home/'local/bindings/update.json'
        path.parent.mkdir(parents=True)
        raw=json.dumps(dict(phase='applied',rootProgress=3,metadataProgress=2,
            recoveryStep=0,oldCopied=4,newCopied=5,metadata={'private':'PRIVATE-CANARY'},
            stage='PRIVATE-PATH-CANARY')).encode()
        path.write_bytes(raw)
        before=path.stat()
        out=io.StringIO()
        with redirect_stdout(out):
            observation=update.observe_rollback_journal(home)
        self.assertEqual(observation,dict(sha256=hashlib.sha256(raw).hexdigest(),phase='applied',
            rootProgress=3,metadataProgress=2,recoveryStep=0,oldCopied=4,newCopied=5))
        self.assertNotIn('PRIVATE',out.getvalue())
        self.assertEqual(path.read_bytes(),raw)
        self.assertEqual(path.stat().st_mtime_ns,before.st_mtime_ns)
        for raw in (b'{',json.dumps(dict(phase=['PRIVATE-CANARY'],rootProgress=True)).encode()):
            path.write_bytes(raw)
            with redirect_stdout(out):
                observation=update.observe_rollback_journal(home)
            self.assertEqual(observation['phase'],'unknown')
            self.assertNotIn('rootProgress',observation)
        path.unlink()
        with redirect_stdout(out):
            observation=update.observe_rollback_journal(home)
        self.assertEqual(observation['observation'],'io')
        self.assertIsInstance(observation['errno'],int)

    def test_failure_report_strictly_redacts_and_bounds_export(self):
        private='PRIVATE-CANARY/path/member'
        diagnostic=dict(boundary='before_admission',validator='recovery_start.root_placement',
            predicate='system_content',kind='io_error',operation='read',osCode=32,
            phase='applied',recoveryStep=0,rootProgress=7,metadataProgress=3,
            unknown=private,path=private,nativeId=private,member=private,
            observations=[dict(probe=0,predicate='root_content',kind='mismatch',path=private)]*100)
        response=dict(ok=False,code='UPDATE_REFUSED',data=dict(diagnostic=diagnostic,private=private))
        metadata=dict(sha256='a'*64,oldSource='b'*40,newSource='c'*40,phase='applied',
            rootProgress=7,path=private,nativeId=private,rawJournal=private)
        raw=update.safe_failure_report(response,metadata,metadata)
        self.assertLessEqual(len(raw),4096)
        self.assertNotIn(b'PRIVATE',raw)
        value=json.loads(raw)
        self.assertEqual(value['diagnostic']['osCode'],32)
        for predicate in ('activation_phase','recovery_phase','stage_presence'):
            diagnostic.update(predicate=predicate,operation='path_check')
            exported=json.loads(update.safe_failure_report(response,metadata,metadata))['diagnostic']
            self.assertEqual(exported['predicate'],predicate)
            self.assertEqual(exported['operation'],'path_check')
        self.assertEqual(len(value['diagnostic']['observations']),5)
        self.assertEqual(set(value['before']),{'sha256','oldSource','newSource','phase','rootProgress'})
        for key in update.DIAGNOSTIC_ENUMS:
            diagnostic[key]='CANARY'  # Safe-looking tokens are still not admitted enums.
        diagnostic.update(osCode=True,recoveryStep=-1,rootProgress=2**80,memberOrdinal=private)
        metadata.update(sha256=private,oldSource=private,newSource=private,phase=private,rootProgress=True)
        raw=update.safe_failure_report(response,metadata,metadata)
        self.assertNotIn(b'CANARY',raw)
        value=json.loads(raw)
        self.assertNotIn('boundary',value['diagnostic'])
        self.assertNotIn('osCode',value['diagnostic'])
        self.assertEqual(value['before'],{})

    def test_failed_consumer_entrypoint_retains_root_and_only_sanitized_report(self):
        env=dict(os.environ,TEMP=str(self.workspace),TMP=str(self.workspace),TMPDIR=str(self.workspace))
        result=subprocess.run([sys.executable,'-B',str(Path(update.__file__).resolve()),
            '--binary',str(self.workspace/'missing-native-binary')],env=env,
            capture_output=True,text=True,encoding='utf-8',timeout=20)
        self.assertEqual(result.returncode,1)
        self.assertIn('FileNotFoundError',result.stderr)
        self.assertIn('UPDATE_HOME_FIXTURE_PRESERVED',result.stderr)
        roots=list(self.workspace.glob('ocmi-update-home-*'))
        reports=list(self.workspace.glob('ocmi-update-report-*.json'))
        self.assertEqual(len(roots),1)
        self.assertTrue((roots[0]/'candidate/system/bin').is_dir())
        self.assertEqual(len(reports),1)
        raw=reports[0].read_bytes()
        self.assertLessEqual(len(raw),4096)
        self.assertEqual(json.loads(raw),dict(schemaVersion=1,before={},after={}))
        self.assertNotIn(str(self.workspace).encode(),raw)

    def test_failure_report_writer_exports_safe_before_after_only(self):
        base=self.workspace/'fixture'
        home=base/'home'
        journal=home/'local/bindings/update.json'
        journal.parent.mkdir(parents=True)
        before=json.dumps(dict(phase='applied',rootProgress=7,oldSource='a'*40,
            newSource='b'*40,metadata={'private':'PRIVATE-CANARY'})).encode()
        journal.write_bytes(before)
        out=io.StringIO()
        with redirect_stdout(out):
            update.observe_rollback_journal(home)
        after=json.dumps(dict(phase='recovering',recoveryStep=1,metadata={'path':str(home)})).encode()
        journal.write_bytes(after)
        update._report_context['response']=dict(ok=False,code='UPDATE_REFUSED',
            data=dict(diagnostic=dict(boundary='recover_public',validator='recovery_state',
                predicate='system_content',kind='mismatch',phase='recovering',recoveryStep=1,
                rawJournal='PRIVATE-CANARY',path=str(home))))
        with redirect_stdout(out):
            update.write_failure_report(base)
        reports=list(self.workspace.glob('ocmi-update-report-*.json'))
        self.assertEqual(len(reports),1)
        raw=reports[0].read_bytes()
        self.assertNotIn(b'PRIVATE',raw)
        self.assertNotIn(str(home).encode(),raw)
        value=json.loads(raw)
        self.assertEqual(value['before']['sha256'],hashlib.sha256(before).hexdigest())
        self.assertEqual(value['after']['sha256'],hashlib.sha256(after).hexdigest())
        self.assertEqual(value['diagnostic']['boundary'],'recover_public')
        self.assertEqual(journal.read_bytes(),after)

    @unittest.skipUnless(os.name=='nt','Windows junction control')
    def test_windows_junction_preserves_target_after_collection(self):
        import _winapi
        if not hasattr(_winapi,'CreateJunction'):
            self.skipTest('native junction creation unavailable')
        target=self.workspace/'junction-target'
        target.mkdir()
        (target/'synthetic.txt').write_bytes(b'junction target')
        try:
            with self.assertRaisesRegex(RuntimeError,'root changed'):
                with install.install_fixture() as name:
                    root=Path(name)
                    root.rename(self.workspace/'parked-junction')
                    _winapi.CreateJunction(str(target),str(root))
            gc.collect()
            self.assertTrue(root.stat(follow_symlinks=False).st_file_attributes & stat.FILE_ATTRIBUTE_REPARSE_POINT)
            self.assertEqual((target/'synthetic.txt').read_bytes(),b'junction target')
        finally:
            if 'root' in locals() and root.exists():
                root.rmdir()


if __name__=='__main__':
    unittest.main(verbosity=2)
