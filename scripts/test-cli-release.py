#!/usr/bin/env python3
"""Verify the release layout and perform an isolated native shell installation."""
from pathlib import Path
import hashlib
import json
import http.server
import os
import platform
import subprocess
import tarfile
import tempfile
import threading
import zipfile
from functools import partial

ROOT = Path(__file__).resolve().parents[1]
DIST = ROOT/'apps/web/public/cli'
manifest = dict((name, digest) for digest, name in (line.split() for line in (DIST/'checksums.txt').read_text().splitlines()))
for prefix in ('meperstudio','silieco'):
    for os_name in ('darwin','linux','windows'):
        for arch in ('amd64','arm64'):
            name = f'{prefix}-{os_name}-{arch}.'+('zip' if os_name=='windows' else 'tar.gz')
            archive = DIST/name
            assert hashlib.sha256(archive.read_bytes()).hexdigest() == manifest[name], name
            if os_name=='windows':
                with zipfile.ZipFile(archive) as z:
                    assert z.namelist() == [prefix+'.exe'], name
                    assert z.testzip() is None, name
            else:
                with tarfile.open(archive) as t:
                    assert t.getnames() == [prefix], name
                    assert t.getmember(prefix).mode & 0o111, name
print('Verified six branded and six compatibility archives with matching SHA-256 checksums.')
release = json.loads((DIST/'manifest.json').read_text())
assert release['schema_version'] == 'meperstudio.cli/v1' and len(release['artifacts']) == 6
lines = (DIST/'manifest.tsv').read_text().splitlines()
assert lines[0] == 'meperstudio.cli/v1\t'+release['version']
assert len(lines) == 7
for entry,line in zip(release['artifacts'],lines[1:]):
    assert line.split('\t') == [str(entry[k]) for k in ('os','arch','path','archive_name','binary','sha256','size_bytes')]
    artifact = DIST/entry['path']
    assert artifact.stat().st_size == entry['size_bytes']
    assert hashlib.sha256(artifact.read_bytes()).hexdigest() == entry['sha256']
print('Verified content-addressed JSON/TSV manifest and immutable release files.')
if platform.system() not in ('Darwin','Linux'):
    raise SystemExit('Native shell install test requires macOS or Linux.')
class QuietHandler(http.server.SimpleHTTPRequestHandler):
    mode = 'current'
    def log_message(self, *_): pass
    def do_GET(self):
        if self.mode == 'legacy' and self.path in ('/manifest.tsv', '/manifest.json'):
            self.send_error(404)
            return
        if self.mode == 'damaged' and self.path.startswith('/releases/'):
            self.send_response(200)
            self.end_headers()
            self.wfile.write(b'damaged archive')
            return
        super().do_GET()
with http.server.ThreadingHTTPServer(('127.0.0.1',0), partial(QuietHandler,directory=str(DIST))) as server:
    threading.Thread(target=server.serve_forever,daemon=True).start()
    base = f'http://127.0.0.1:{server.server_port}'
    with tempfile.TemporaryDirectory(prefix='meperstudio-install-test-') as tmp:
        home = Path(tmp)
        env = {k:v for k,v in os.environ.items() if not k.startswith(('SILIECO_','MEPERSTUDIO_'))}
        env.update(HOME=tmp, MEPERSTUDIO_DOWNLOAD_BASE_URL=base, MEPERSTUDIO_BIN_DIR=str(home/'bin'))
        subprocess.run(['bash',str(DIST/'install.sh')],env=env,check=True,cwd=tmp)
        binary = home/'bin/meperstudio'
        help_text = subprocess.check_output([str(binary),'--help'],env=env,cwd=tmp,text=True)
        assert 'MEPERSTUDIO_SERVER_URL' in help_text and 'SILIECO_SERVER_URL' not in help_text
        subprocess.run([str(binary),'config','set','server_url',base],env=env,cwd=tmp,check=True)
        assert (home/'.meperstudio/config.json').exists(), 'expected branded state directory'
        expected_binary = hashlib.sha256(binary.read_bytes()).hexdigest()
        QuietHandler.mode = 'legacy'
        subprocess.run(['bash', str(DIST/'install.sh')], env=env, check=True, cwd=tmp)
        assert hashlib.sha256(binary.read_bytes()).hexdigest() == expected_binary
        QuietHandler.mode = 'damaged'
        failed = subprocess.run(['bash', str(DIST/'install.sh')], env=env, cwd=tmp)
        assert failed.returncode != 0, 'corrupt artifact unexpectedly installed'
        assert hashlib.sha256(binary.read_bytes()).hexdigest() == expected_binary, 'working binary was overwritten'
        subprocess.run([str(binary), '--version'], env=env, check=True, cwd=tmp)
    server.shutdown()
print('Native installation, legacy fallback and damaged-release recovery succeeded in a temporary home.')
