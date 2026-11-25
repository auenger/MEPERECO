import { useAuthStore } from "@silieco/core/auth";
import { useWelcomeStore } from "@silieco/core/onboarding";
import { useTabStore } from "../stores/tab-store";
import { useWindowOverlayStore } from "../stores/window-overlay-store";

let logoutCleanup: Promise<void> | undefined;

// On logout, wipe desktop-only in-memory state and stop the daemon so that
// a subsequent login as a different user never inherits the previous user's
// tabs, overlay, or credentials. Zustand persist only writes to localStorage;
// useLogout clears the storage key, but the live stores stay populated until
// we explicitly reset them here.
async function cleanupDaemonLogout() {
  // Report synchronously before async daemon cleanup so a rapidly closed main
  // window cannot leave authenticated task renderers behind.
  window.desktopAPI.reportAuthSession?.(null);
  useTabStore.getState().reset();
  useWindowOverlayStore.getState().close();
  // Drop any post-onboarding welcome signal so user B logging in next
  // doesn't inherit user A's pending modal state.
  useWelcomeStore.getState().reset();
  try {
    await window.daemonAPI.clearToken();
  } catch {
    // Best-effort — clearing is followed by stop which also hardens state.
  }
  try {
    await window.daemonAPI.stop();
  } catch {
    // Daemon may already be stopped.
  }
}

export function handleDaemonLogout(): Promise<void> {
  logoutCleanup = cleanupDaemonLogout();
  return logoutCleanup;
}

/** Clear the old account and finish daemon cleanup before restarting. */
export async function restartForServerChange(): Promise<void> {
  logoutCleanup = undefined;
  useAuthStore.getState().logout();
  await (logoutCleanup ?? handleDaemonLogout());
  await window.desktopConfig.restart();
}
