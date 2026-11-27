export interface UpdaterPreferences {
  automaticUpdates: boolean;
}

export type ManualUpdateCheckResult =
  | {
      ok: true;
      currentVersion: string;
      latestVersion: string;
      available: boolean;
    }
  | { ok: false; error: string };

export type DesktopUpdateState =
  | { status: "idle" }
  | { status: "downloading"; version: string; percent: number }
  | { status: "preparing" | "ready" | "installing" | "manual"; version: string }
  | { status: "error"; version: string; reason: "download" | "prepare" | "install"; message: string };

/** A discovered update remains pending until the updater returns to idle. */
export function pendingDesktopUpdateVersion(state: DesktopUpdateState): string | null {
  return state.status === "idle" ? null : state.version;
}

export type UpdateInstallResult = { ok: true } | { ok: false; error: string };
