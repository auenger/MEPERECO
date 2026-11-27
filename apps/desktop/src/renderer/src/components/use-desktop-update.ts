import { useEffect, useState } from "react";
import { toast } from "sonner";
import { useT } from "@silieco/views/i18n";
import { pendingDesktopUpdateVersion, type DesktopUpdateState } from "../../../shared/updater-types";
import { DEFAULT_RUNTIME_CONFIG } from "../../../shared/runtime-config";

const selectState = (state: DesktopUpdateState) => state;

function useDesktopUpdateSnapshot<T>(select: (state: DesktopUpdateState) => T): T {
  const [state, setState] = useState(() => select({ status: "idle" }));
  useEffect(() => {
    let mounted = true;
    let receivedEvent = false;
    const stop = window.updater.onState((next) => {
      receivedEvent = true;
      if (mounted) setState(() => select(next));
    });
    // Subscribe first so a window reload or settings mount cannot miss a
    // completed download or overwrite a newer event with an old snapshot.
    void window.updater.getState().then((next) => {
      if (mounted && !receivedEvent) setState(() => select(next));
    }).catch(console.error);
    return () => { mounted = false; stop(); };
  }, [select]);
  return state;
}

export function useDesktopUpdateState(): DesktopUpdateState {
  return useDesktopUpdateSnapshot(selectState);
}

/** Notice observers only re-render when availability/version changes, not on download progress. */
export function useDesktopUpdateNotice(): string | undefined {
  const version = useDesktopUpdateSnapshot(pendingDesktopUpdateVersion);
  const { t } = useT("settings");
  return version === null ? undefined : t(($) => $.desktop.updates.available_notice, { version });
}

export async function requestUpdateInstall(fallbackMessage: string) {
  try {
    const result = await window.updater.installUpdate();
    if (!result.ok) toast.error(fallbackMessage, { description: result.error });
  } catch {
    toast.error(fallbackMessage);
  }
}

export async function openUpdateDownload(fallbackMessage: string) {
  const config = window.desktopAPI.runtimeConfig;
  const appUrl = config.ok ? config.config.appUrl : DEFAULT_RUNTIME_CONFIG.appUrl;
  try {
    await window.desktopAPI.openExternal(`${appUrl.replace(/\/+$/, "")}/download`);
  } catch {
    toast.error(fallbackMessage);
  }
}
