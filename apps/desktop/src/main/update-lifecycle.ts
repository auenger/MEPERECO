import { setTimeout, clearTimeout } from "node:timers";
import type { DesktopUpdateState, UpdateInstallResult } from "../shared/updater-types";

const PREPARE_TIMEOUT_MS = 120_000;
const INSTALL_TIMEOUT_MS = 30_000;

type Timer = number | ReturnType<typeof setTimeout>;
interface LifecycleOptions {
  requiresNativePreparation: boolean;
  manualOnly: boolean;
  publish: (state: DesktopUpdateState) => void;
  install: () => void;
  schedule?: (fn: () => void, delay: number) => Timer;
  cancel?: (timer: Timer) => void;
}

/** Explicitly separate a downloaded archive from a platform-ready installer. */
export function createUpdateLifecycle(options: LifecycleOptions) {
  let state: DesktopUpdateState = { status: "idle" };
  let timer: Timer | undefined;
  const schedule = options.schedule ?? setTimeout;
  const cancel = options.cancel ?? clearTimeout;

  function snapshot(): DesktopUpdateState { return state; }

  function transition(next: DesktopUpdateState) {
    if (timer !== undefined) cancel(timer);
    timer = undefined;
    state = next;
    options.publish(state);
  }

  function fail(reason: "download" | "prepare" | "install", message: string) {
    if (state.status === "idle" || state.status === "manual") return;
    transition({ status: "error", version: state.version, reason, message });
  }

  return {
    getState: snapshot,
    notAvailable() {
      if (state.status === "manual" || state.status === "error") transition({ status: "idle" });
    },
    available(version: string) {
      if (state.status !== "idle" && state.version === version && state.status !== "error") return;
      transition(options.manualOnly
        ? { status: "manual", version }
        : { status: "downloading", version, percent: 0 });
    },
    progress(percent: number) {
      if (state.status === "downloading" && Number.isFinite(percent)) {
        transition({ ...state, percent: Math.max(0, Math.min(100, percent)) });
      }
    },
    downloaded(version: string) {
      if (options.manualOnly || state.status === "installing") return;
      if ((state.status === "ready" || state.status === "preparing") && state.version === version) return;
      if (!options.requiresNativePreparation) {
        transition({ status: "ready", version });
        return;
      }
      transition({ status: "preparing", version });
      timer = schedule(() => fail("prepare", "Native installer did not become ready in time."), PREPARE_TIMEOUT_MS);
    },
    nativeReady() {
      if (state.status === "preparing" || (state.status === "error" && state.reason === "prepare")) {
        transition({ status: "ready", version: state.version });
      }
    },
    error(error: unknown) {
      if (state.status === "error") return;
      const reason = state.status === "installing" || state.status === "ready" ? "install"
        : state.status === "preparing" ? "prepare" : "download";
      fail(reason, error instanceof Error ? error.message : String(error));
    },
    requestInstall(): UpdateInstallResult {
      if (state.status === "installing") return { ok: true };
      if (state.status !== "ready") return { ok: false, error: "The update is not ready to install." };
      const version = state.version;
      transition({ status: "installing", version });
      timer = schedule(() => fail("install", "The application did not restart in time."), INSTALL_TIMEOUT_MS);
      try {
        options.install();
      } catch (error) {
        fail("install", error instanceof Error ? error.message : String(error));
      }
      const result = snapshot();
      return result.status === "error" ? { ok: false, error: result.message } : { ok: true };
    },
  };
}

export function isAdHocMacSignature(output: string): boolean {
  return /(?:^|\n)Signature=adhoc(?:\r?\n|$)/.test(output);
}
