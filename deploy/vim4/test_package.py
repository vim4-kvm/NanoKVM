import base64
import hashlib
import importlib.util
import json
from pathlib import Path
import shutil
import subprocess
import tarfile
import tempfile
import unittest

spec = importlib.util.spec_from_file_location('bundle', Path(__file__).with_name('package.py'))
bundle = importlib.util.module_from_spec(spec)
spec.loader.exec_module(bundle)


@unittest.skipUnless(shutil.which('dpkg-deb'), 'Requires Debian packaging tools')
class PackageTest(unittest.TestCase):
    def test_bundle_preserves_exact_component_bytes_and_hashes(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            control = root / 'stage/DEBIAN'
            control.mkdir(parents=True)
            (control / 'control').write_text('Package: vim4-nanokvm\nVersion: 1.0.3+test.1\nArchitecture: arm64\nMaintainer: Test\nDescription: test fixture\n')
            deb = root / 'application.deb'
            subprocess.run(['dpkg-deb', '--build', str(control.parent), str(deb)], check=True, stdout=subprocess.DEVNULL)
            output = root / 'out'
            bundle.build(deb, output)
            manifest = json.loads((output / 'nanokvm_pro_latest.json').read_text())
            archive = output / manifest['name']
            self.assertEqual(base64.b64decode(manifest['sha512']), hashlib.sha512(archive.read_bytes()).digest())
            with tarfile.open(archive) as tar:
                member = next(m for m in tar.getmembers() if m.name.endswith('.deb'))
                self.assertEqual(tar.extractfile(member).read(), deb.read_bytes())
                metadata = json.load(tar.extractfile(next(m for m in tar.getmembers() if m.name.endswith('.json'))))
                self.assertEqual(base64.b64decode(metadata['sha512']), hashlib.sha512(deb.read_bytes()).digest())
            with self.assertRaisesRegex(ValueError, 'overwrite'):
                bundle.build(deb, output)


if __name__ == '__main__':
    unittest.main()
