#!/usr/bin/env python3
"""Build the six Studio CLI archives and their checksum manifest atomically."""
import argparse
import hashlib
import os
from pathlib import Path
import subprocess
import tarfile
import tempfile
import zipfile
import datetime

ROOT = Path(__file__).resolve().parents[1]

def build(output: Path, version: str, commit: str):
    output.mkdir(parents=True, exist_ok=True)
    date = datetime.datetime.now(datetime.timezone.utc).strftime('%Y-%m-%dT%H:%M:%SZ')
    with tempfile.TemporaryDirectory(dir=output.parent, prefix='meperstudio-cli-') as temp:
        work = Path(temp)
        archives = []
        for os_name in ('darwin', 'linux', 'windows'):
            for arch in ('amd64', 'arm64'):
                binary = work / ('meperstudio.exe' if os_name == 'windows' else 'meperstudio')
                env = dict(os.environ, GOOS=os_name, GOARCH=arch, CGO_ENABLED='0')
                print(f'Building {os_name}/{arch}', flush=True)
                subprocess.run(['go', 'build', '-trimpath', '-ldflags', f'-s -w -X main.version={version} -X main.commit={commit} -X main.date={date}', '-o', str(binary), './cmd/silieco'], cwd=ROOT/'core', env=env, check=True)
                # Legacy filenames/binaries are published as well, so already
                # installed Desktop releases can still repair themselves.
                for prefix in ('meperstudio', 'silieco'):
                    name = prefix + ('.exe' if os_name == 'windows' else '')
                    if os_name == 'windows':
                        archive = work / f'{prefix}-{os_name}-{arch}.zip'
                        with zipfile.ZipFile(archive, 'w', zipfile.ZIP_DEFLATED, compresslevel=6) as z:
                            z.write(binary, name)
                    else:
                        archive = work / f'{prefix}-{os_name}-{arch}.tar.gz'
                        with tarfile.open(archive, 'w:gz', compresslevel=6) as t:
                            t.add(binary, arcname=name)
                    archives.append(archive)
                binary.unlink()
        manifest = work/'checksums.txt'
        manifest.write_text(''.join(f'{hashlib.sha256(p.read_bytes()).hexdigest()}  {p.name}\n' for p in sorted(archives)))
        release = work/'version.txt'
        release.write_text(version+'\n')
        for path in archives + [release, manifest]:
            path.replace(output/path.name)

if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--output', type=Path, default=ROOT/'apps/web/public/cli')
    parser.add_argument('--version', default='1.0.0')
    parser.add_argument('--commit', default='source')
    args = parser.parse_args()
    build(args.output.resolve(), args.version, args.commit)
