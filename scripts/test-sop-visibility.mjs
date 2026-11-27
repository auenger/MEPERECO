import assert from "node:assert/strict";
import test from "node:test";

// The shared preference uses the SSR-safe browser storage adapter. Supply
// an isolated device storage so the test never touches an actual profile.
const entries = new Map();
globalThis.window = {};
globalThis.localStorage = {
  getItem: (key) => entries.get(key) ?? null,
  setItem: (key, value) => entries.set(key, value),
  removeItem: (key) => entries.delete(key),
};

const { useSopVisibilityStore, isSopRoute } = await import("../packages/core/workflows/visibility-store.ts");
const key = "silieco_tr1_sop_visibility";

test("SOP is hidden by default and toggling persists the preference", async () => {
  assert.equal(useSopVisibilityStore.getState().enabled, false);
  const changes = [];
  const unsubscribe = useSopVisibilityStore.subscribe((state) => changes.push(state.enabled));
  useSopVisibilityStore.getState().setEnabled(true);
  assert.equal(useSopVisibilityStore.getState().enabled, true);
  const enabledSnapshot = localStorage.getItem(key);
  assert.equal(JSON.parse(enabledSnapshot).state.enabled, true);
  useSopVisibilityStore.getState().setEnabled(false);
  assert.equal(JSON.parse(localStorage.getItem(key)).state.enabled, false);
  localStorage.setItem(key, enabledSnapshot);
  await useSopVisibilityStore.persist.rehydrate();
  assert.equal(useSopVisibilityStore.getState().enabled, true);
  assert.deepEqual(changes, [true, false, true]);
  unsubscribe();
  useSopVisibilityStore.getState().setEnabled(false);
});

test("saved SOP tabs can be hidden without hiding ordinary task/resource tabs", () => {
  for (const url of [
    "/space/workflows",
    "/space/workflows/template-id",
    "/space/projects/project-id?section=sop",
    "/space/projects/project-id?run=run-id",
  ]) assert.equal(isSopRoute(url), true, url);
  for (const url of [
    "/workflows/tasks",
    "/space/tasks/task-id",
    "/space/projects/project-id",
    "/space/projects/project-id?section=resources",
    "/space/projects/project-id/files",
    "/space/settings?tab=preferences",
    "/space/chat?session=workflows",
  ]) assert.equal(isSopRoute(url), false, url);
});
