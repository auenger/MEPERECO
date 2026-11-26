#!/usr/bin/env python3
"""Verify the release layout and perform an isolated native shell installation."""
from pathlib import Path
import hashlib
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
if platform.system() not in ('Darwin','Linux'):
    raise SystemExit('Native shell install test requires macOS or Linux.')
class QuietHandler(http.server.SimpleHTTPRequestHandler):
    def log_message(self, *_): pass
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
    server.shutdown()
print('Native installation and configuration succeeded in a temporary home.')
