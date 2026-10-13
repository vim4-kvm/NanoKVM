#!/usr/bin/python3
"""Wrap a verified Ubuntu application component in the NanoKVM update protocol."""
import argparse
import base64
import hashlib
import json
from pathlib import Path
import re
import shutil
import subprocess
import tarfile
import tempfile


def metadata(path, version):
    return dict(version=version, name=path.name, size=path.stat().st_size,
                sha512=base64.b64encode(hashlib.sha512(path.read_bytes()).digest()).decode())


def build(package, output):
    # Packaging has one owner: the workspace's Ubuntu component builder. In
    # particular, do not rebuild helpers, dependencies or units differently here.
    fields = {}
    for field in ('Package', 'Architecture', 'Version'):
        fields[field] = subprocess.check_output(
            ['dpkg-deb', '-f', str(package), field], text=True).strip()
    if fields['Package'] != 'vim4-nanokvm' or fields['Architecture'] != 'arm64':
        raise ValueError('Expected a vim4-nanokvm arm64 component package')
    version = fields['Version']
    if not re.fullmatch(r'[0-9]+\.[0-9]+\.[0-9]+(?:[+.-][A-Za-z0-9.-]+)?', version):
        raise ValueError('Use a semantic application version')
    output.mkdir(parents=True, exist_ok=True)
    archive = output / f'nanokvm_pro_{version}.tar.gz'
    if archive.exists():
        raise ValueError('Refusing to overwrite an existing release')
    with tempfile.TemporaryDirectory() as temporary:
        bundle = Path(temporary) / f'nanokvm_pro_{version}'
        bundle.mkdir()
        deb = bundle / f'vim4-nanokvm_{version}_arm64.deb'
        shutil.copyfile(package, deb)
        (bundle / f'vim4-nanokvm_{version}.json').write_text(json.dumps(metadata(deb, version)))
        with tarfile.open(archive, 'w:gz') as tar:
            tar.add(bundle, arcname=bundle.name)
    (output / 'nanokvm_pro_latest.json').write_text(json.dumps(metadata(archive, version)))
    print(archive)


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('package', type=Path, help='Verified vim4-nanokvm ARM64 deb from the workspace build')
    parser.add_argument('output', type=Path)
    args = parser.parse_args()
    build(args.package, args.output)
