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
import json
import re
import shutil

ROOT = Path(__file__).resolve().parents[1]

def build(output: Path, version: str, commit: str):
    version = version.removeprefix("v")
    if not re.fullmatch(r"[0-9]+\.[0-9]+\.[0-9]+", version):
        raise ValueError("release version must contain three numeric components")
    output.mkdir(parents=True, exist_ok=True)
    date = datetime.datetime.now(datetime.timezone.utc).strftime('%Y-%m-%dT%H:%M:%SZ')
    with tempfile.TemporaryDirectory(dir=output.parent, prefix='meperstudio-cli-') as temp:
        work = Path(temp)
        archives = []
        artifacts = []
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
                    if prefix == 'meperstudio':
                        digest = hashlib.sha256(archive.read_bytes()).hexdigest()
                        relative = f'releases/{version}/{digest}/{archive.name}'
                        artifacts.append(dict(os=os_name, arch=arch, path=relative, archive_name=archive.name, binary=name, sha256=digest, size_bytes=archive.stat().st_size))
                binary.unlink()
        manifest = work/'checksums.txt'
        manifest.write_text(''.join(f'{hashlib.sha256(p.read_bytes()).hexdigest()}  {p.name}\n' for p in sorted(archives)))
        release = work/'version.txt'
        release.write_text(version+'\n')
        # Publish immutable content-addressed paths before changing the pointer.
        for entry in artifacts:
            target = output/entry['path']
            target.parent.mkdir(parents=True, exist_ok=True)
            if not target.exists():
                staged = work/(entry['sha256']+'.archive')
                shutil.copyfile(work/entry['archive_name'], staged)
                staged.replace(target)
            elif hashlib.sha256(target.read_bytes()).hexdigest() != entry['sha256']:
                raise ValueError(f'Existing immutable artifact differs: {target}')
        # Compatibility assets for releases that predate the manifest.
        for path in archives + [release, manifest]:
            path.replace(output/path.name)
        release_manifest = dict(schema_version='meperstudio.cli/v1', version=version, artifacts=artifacts)
        pointer = work/'manifest.json'
        pointer.write_text(json.dumps(release_manifest, indent=2)+'\n')
        # TSV projection of the exact same manifest allows a POSIX installer to
        # read it without depending on Python, Node or jq on a remote host.
        tsv = work/'manifest.tsv'
        tsv.write_text('meperstudio.cli/v1\t'+version+'\n'+''.join('\t'.join(str(e[k]) for k in ('os','arch','path','archive_name','binary','sha256','size_bytes'))+'\n' for e in artifacts))
        tsv.replace(output/tsv.name)
        pointer.replace(output/pointer.name)

if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--output', type=Path, default=ROOT/'apps/web/public/cli')
    parser.add_argument('--version', default='1.0.0')
    parser.add_argument('--commit', default='source')
    args = parser.parse_args()
    build(args.output.resolve(), args.version, args.commit)
