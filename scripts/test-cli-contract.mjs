import test from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync, mkdtempSync, mkdirSync, rmSync } from 'node:fs';
import { join } from 'node:path';
import { tmpdir } from 'node:os';
import { stateDir } from '../apps/desktop/src/main/state-dir.ts';
import { parseCliManifest, cliArtifactFor } from '../packages/core/release/cli-manifest.ts';

test('Go and Electron use the same state-directory contract', () => {
  const cases = JSON.parse(readFileSync(new URL('../core/internal/branding/testdata/state-dir.json', import.meta.url), 'utf8'));
  for (const fixture of cases) {
    const home = mkdtempSync(join(tmpdir(), 'meperstudio-test-'));
    try {
      for (const dir of fixture.directories) mkdirSync(join(home, dir));
      assert.equal(stateDir(home), join(home, fixture.expected));
    } finally { rmSync(home, { recursive: true, force: true }); }
  }
});
test('TypeScript release client selects and validates manifest artifacts', () => {
  const sha256 = 'a'.repeat(64);
  const artifact = { os: 'windows', arch: 'amd64', path: `releases/1.2.3/${sha256}/bundle.zip`, archive_name: 'bundle.zip', binary: 'meperstudio.exe', sha256, size_bytes: 42 };
  const fixture = { schema_version: 'meperstudio.cli/v1', version: '1.2.3', artifacts: [artifact] };
  assert.deepEqual(cliArtifactFor(parseCliManifest(fixture), 'win32', 'x64'), artifact);
  assert.throws(() => cliArtifactFor(parseCliManifest(fixture), 'linux', 'arm64'));
  for (const patch of [{ path: '../bundle.zip' }, { sha256: 'wrong' }, { size_bytes: 0 }, { binary: '../meperstudio.exe' }]) assert.throws(() => parseCliManifest({ ...fixture, artifacts: [{ ...artifact, ...patch }] }));
  assert.throws(() => parseCliManifest({ ...fixture, artifacts: [artifact, artifact] }));
});
