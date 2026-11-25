import { app, ipcMain } from "electron";
import { is } from "@electron-toolkit/utils";
import { mkdir, readFile, writeFile } from "node:fs/promises";
import { dirname } from "node:path";
import { desktopConfigPath } from "./runtime-config-loader";

import { parseRuntimeConfig } from "../shared/runtime-config";

// The desktop runtime config file (~/.silieco/desktop.json) is the single
// persistence point for "which server is this app pointed at". It already
// feeds loadRuntimeConfig on every packaged launch; these handlers expose it
// to the Settings UI so the user can switch deployments in-app. API / WS /
// daemon profile / update feed all derive from the stored apiUrl, so a
// restart after a write repoints the whole app.

/**
 * Returns the stored apiUrl, or null when the file is missing or invalid
 * (the app then falls back to DEFAULT_RUNTIME_CONFIG on next launch).
 */
export async function readDesktopApiUrl(): Promise<string | null> {
  if (is.dev) return null;
  try {
    const raw = await readFile(desktopConfigPath(), "utf-8");
    return parseRuntimeConfig(raw).apiUrl;
  } catch {
    return null;
  }
}

export type WriteDesktopConfigResult =
  | { ok: true; apiUrl: string }
  | { ok: false; error: string };

export async function writeDesktopApiUrl(
  apiUrl: unknown,
): Promise<WriteDesktopConfigResult> {
  try {
    if (is.dev) {
      return {
        ok: false,
        error: "Development server addresses are configured by VITE_API_URL, VITE_WS_URL and VITE_APP_URL.",
      };
    }
    if (typeof apiUrl !== "string" || apiUrl.trim().length === 0) {
      return { ok: false, error: "Server address must be a non-empty URL." };
    }
    // Validate + normalize through the same parser the loader uses, so a
    // value accepted here is guaranteed to load on the next launch.
    const normalized = parseRuntimeConfig(
      JSON.stringify({ schemaVersion: 1, apiUrl }),
    );
    const path = desktopConfigPath();
    await mkdir(dirname(path), { recursive: true });
    await writeFile(
      path,
      `${JSON.stringify({ schemaVersion: 1, apiUrl: normalized.apiUrl }, null, 2)}\n`,
      "utf-8",
    );
    return { ok: true, apiUrl: normalized.apiUrl };
  } catch (err) {
    return {
      ok: false,
      error: err instanceof Error ? err.message : String(err),
    };
  }
}

export function registerDesktopConfigIpc(): void {
  ipcMain.handle("desktop-config:read", () => readDesktopApiUrl());
  ipcMain.handle("desktop-config:write", (_event, apiUrl: unknown) =>
    writeDesktopApiUrl(apiUrl),
  );
  ipcMain.handle("desktop-config:restart", () => {
    if (is.dev) return;
    app.relaunch();
    app.quit();
  });
}
