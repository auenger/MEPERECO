import assert from 'node:assert/strict';
import test from 'node:test';
import { createUpdateLifecycle, isAdHocMacSignature } from '../apps/desktop/src/main/update-lifecycle.ts';

function fixture(options = {}) {
  let calls = 0;
  let sequence = 0;
  const timers = new Map();
  const events = [];
  const lifecycle = createUpdateLifecycle({
    requiresNativePreparation: true,
    manualOnly: false,
    publish: state => events.push(state),
    install: () => { calls++; },
    schedule: (fn, delay) => { const id = ++sequence; timers.set(id, { fn, delay }); return id; },
    cancel: id => timers.delete(id),
    ...options,
  });
  return {
    lifecycle, events, calls: () => calls,
    advance(delay) {
      for (const [id, timer] of [...timers]) if (timer.delay <= delay) { timers.delete(id); timer.fn(); }
    },
    timers,
  };
}

test('macOS ZIP download does not imply native installer readiness', () => {
  const { lifecycle: u, calls, timers } = fixture();
  u.available('1.0.2'); u.progress(100); u.downloaded('1.0.2');
  assert.equal(u.getState().status, 'preparing');
  assert.equal(u.requestInstall().ok, false);
  assert.equal(calls(), 0);
  u.nativeReady();
  assert.equal(u.getState().status, 'ready');
  assert.equal(timers.size, 0);
  assert.equal(u.requestInstall().ok, true);
  assert.equal(u.getState().status, 'installing');
  assert.equal(u.requestInstall().ok, true);
  assert.equal(calls(), 1);
});

test('ad-hoc macOS installation advertises manual update, never starts an installer', () => {
  assert.equal(isAdHocMacSignature('Executable=/App\nSignature=adhoc\nTeamIdentifier=not set'), true);
  assert.equal(isAdHocMacSignature('Authority=Developer ID Application: Example\nTeamIdentifier=EXAMPLE123'), false);
  const { lifecycle: u, calls } = fixture({ manualOnly: true });
  u.available('1.0.2'); u.downloaded('1.0.2'); u.nativeReady();
  assert.deepEqual(u.getState(), { status: 'manual', version: '1.0.2' });
  assert.equal(u.requestInstall().ok, false);
  assert.equal(calls(), 0);
  u.available('1.0.3'); assert.equal(u.getState().version, '1.0.3');
  u.notAvailable(); assert.equal(u.getState().status, 'idle');
});

test('native preparation fails visibly instead of showing a restart button', () => {
  const { lifecycle: u, calls } = fixture();
  u.available('1.0.2'); u.downloaded('1.0.2');
  u.error(new Error('signature requirement not met'));
  assert.deepEqual(u.getState(), { status: 'error', version: '1.0.2', reason: 'prepare', message: 'signature requirement not met' });
  u.error(new Error('duplicate failure'));
  assert.equal(u.getState().reason, 'prepare');
  assert.equal(u.requestInstall().ok, false);
  assert.equal(calls(), 0);
});

test('native preparation timeout is recoverable if the installer finishes later', () => {
  const { lifecycle: u, advance } = fixture();
  u.available('1.0.2'); u.downloaded('1.0.2'); advance(120_000);
  assert.equal(u.getState().reason, 'prepare');
  u.nativeReady(); assert.equal(u.getState().status, 'ready');
});

test('non-Mac installers can restart when the package is downloaded, and timeout is visible', () => {
  const { lifecycle: u, calls, advance } = fixture({ requiresNativePreparation: false });
  u.available('1.0.2'); u.downloaded('1.0.2');
  assert.equal(u.getState().status, 'ready');
  assert.equal(u.requestInstall().ok, true);
  assert.equal(calls(), 1);
  advance(30_000); assert.equal(u.getState().reason, 'install');
});

test('synchronous installer failures are returned to IPC caller', () => {
  const { lifecycle: u, timers } = fixture({ requiresNativePreparation: false, install: () => { throw new Error('cannot start installer'); } });
  u.available('1.0.2'); u.downloaded('1.0.2');
  assert.deepEqual(u.requestInstall(), { ok: false, error: 'cannot start installer' });
  assert.equal(u.getState().reason, 'install');
  assert.equal(timers.size, 0);
});
