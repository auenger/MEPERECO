import { useState } from "react";
import { AlertCircle, ArrowDownToLine, Loader2, RefreshCw, X } from "lucide-react";
import { useT } from "@silieco/views/i18n";
import { Button } from "@silieco/ui/components/ui/button";
import { openUpdateDownload, requestUpdateInstall, useDesktopUpdateState } from "./use-desktop-update";

export function UpdateNotification() {
  const { t } = useT("settings");
  const state = useDesktopUpdateState();
  const [dismissedKey, setDismissedKey] = useState<string | null>(null);

  if (state.status === "idle" || state.status === "downloading" || state.status === "preparing") return null;
  const key = `${state.version}:${state.status}`;
  if (dismissedKey === key) return null;
  const installing = state.status === "installing";
  const ready = state.status === "ready";
  const failed = state.status === "error";
  const Icon = installing ? Loader2 : failed ? AlertCircle : ready ? RefreshCw : ArrowDownToLine;
  const title = installing ? t(($) => $.desktop.updates.installing_title)
    : failed ? t(($) => $.desktop.updates.failed_title)
    : ready ? t(($) => $.desktop.updates.ready_title)
    : t(($) => $.desktop.updates.manual_title);
  const description = installing ? t(($) => $.desktop.updates.installing, { version: state.version })
    : failed ? t(($) => $.desktop.updates.failed, { version: state.version })
    : ready ? t(($) => $.desktop.updates.ready, { version: state.version })
    : t(($) => $.desktop.updates.manual, { version: state.version });

  return (
    <div role="status" className="fixed bottom-4 right-4 z-50 w-80 max-w-[calc(100vw-2rem)] rounded-lg border border-border bg-background p-4 shadow-lg animate-in slide-in-from-bottom-2 fade-in duration-300">
      {!installing && (
        <button type="button" aria-label={t(($) => $.desktop.updates.later)} onClick={() => setDismissedKey(key)}
          className="absolute top-2 right-2 rounded-md p-1 text-muted-foreground hover:text-foreground transition-colors">
          <X className="size-3.5" />
        </button>
      )}
      <div className="flex items-start gap-3">
        <div className={failed ? "mt-0.5 rounded-md bg-destructive/10 p-1.5" : "mt-0.5 rounded-md bg-success/10 p-1.5"}>
          <Icon className={`size-4 ${failed ? "text-destructive" : "text-success"} ${installing ? "animate-spin" : ""}`} />
        </div>
        <div className="flex-1 min-w-0 pr-2">
          <p className="text-body font-medium">{title}</p>
          <p className="text-caption text-muted-foreground mt-0.5">{description}</p>
          <div className="mt-2 flex flex-wrap items-center gap-1.5">
            {ready && (
              <Button size="sm" onClick={() => void requestUpdateInstall(t(($) => $.desktop.updates.failed_title))}>
                {t(($) => $.desktop.updates.restart_now)}
              </Button>
            )}
            {!installing && (
              <>
                <Button size="sm" variant={ready ? "outline" : "default"} onClick={() => void openUpdateDownload(t(($) => $.desktop.updates.download_open_failed))}>
                  {t(($) => $.desktop.updates.manual_download)}
                </Button>
                <Button size="sm" variant="ghost" onClick={() => setDismissedKey(key)}>
                  {t(($) => $.desktop.updates.later)}
                </Button>
              </>
            )}
          </div>
        </div>
      </div>
    </div>
  );
}
