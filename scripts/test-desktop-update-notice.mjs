import assert from "node:assert/strict";
import test from "node:test";
import { createUpdateLifecycle } from "../apps/desktop/src/main/update-lifecycle.ts";
import { pendingDesktopUpdateVersion } from "../apps/desktop/src/shared/updater-types.ts";

test("update notice appears as soon as availability is detected and survives download and restart preparation", () => {
  const events = [];
  const lifecycle = createUpdateLifecycle({
    requiresNativePreparation: true,
    manualOnly: false,
    publish: (state) => events.push(pendingDesktopUpdateVersion(state)),
    install: () => {},
    schedule: () => 0,
    cancel: () => {},
  });
  assert.equal(pendingDesktopUpdateVersion(lifecycle.getState()), null);
  lifecycle.available("1.0.2");
  lifecycle.progress(45);
  lifecycle.downloaded("1.0.2");
  lifecycle.nativeReady();
  assert.equal(lifecycle.requestInstall().ok, true);
  lifecycle.error(new Error("Install failed"));
  assert.deepEqual(events, Array(6).fill("1.0.2"));
  lifecycle.notAvailable();
  assert.equal(pendingDesktopUpdateVersion(lifecycle.getState()), null);
  assert.equal(events.at(-1), null);
});

test("manual updates show the notice; an idle check error or no available update does not", () => {
  const lifecycle = createUpdateLifecycle({
    requiresNativePreparation: true,
    manualOnly: true,
    publish: () => {},
    install: () => {},
  });
  lifecycle.error(new Error("Check failed"));
  assert.equal(pendingDesktopUpdateVersion(lifecycle.getState()), null);
  lifecycle.available("1.0.2");
  assert.equal(pendingDesktopUpdateVersion(lifecycle.getState()), "1.0.2");
  lifecycle.available("1.0.3");
  assert.equal(pendingDesktopUpdateVersion(lifecycle.getState()), "1.0.3");
  lifecycle.notAvailable();
  assert.equal(pendingDesktopUpdateVersion(lifecycle.getState()), null);
});
