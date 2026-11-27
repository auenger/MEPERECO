import { useCallback, useEffect, useState } from "react";
import { AlertCircle, ArrowDownToLine, Check, ExternalLink, Loader2 } from "lucide-react";
import { Button } from "@silieco/ui/components/ui/button";
import { Switch } from "@silieco/ui/components/ui/switch";
import { useT } from "@silieco/views/i18n";
import { SettingsCard, SettingsRow, SettingsTab } from "@silieco/views/settings";
import { toast } from "sonner";
import { openUpdateDownload, requestUpdateInstall, useDesktopUpdateState } from "./use-desktop-update";

type CheckState =
  | { status: "idle" }
  | { status: "checking" }
  | { status: "up-to-date" }
  | { status: "available"; latestVersion: string }
  | { status: "error"; message: string };

export function UpdatesSettingsTab() {
  const { t } = useT("settings");
  const update = useDesktopUpdateState();
  const [state, setState] = useState<CheckState>({ status: "idle" });
  const [automaticUpdates, setAutomaticUpdates] = useState(true);
  const [preferencesReady, setPreferencesReady] = useState(false);
  const [savingPreference, setSavingPreference] = useState(false);
  const currentVersion = window.desktopAPI.appInfo.version;

  useEffect(() => {
    let mounted = true;
    void window.updater
      .getPreferences()
      .then((preferences) => {
        if (mounted) setAutomaticUpdates(preferences.automaticUpdates);
      })
      .catch(() => {
        // The main process falls back to enabled when preferences cannot be
        // read. Keep the same safe default if IPC itself becomes unavailable.
      })
      .finally(() => {
        if (mounted) setPreferencesReady(true);
      });

    return () => {
      mounted = false;
    };
  }, []);

  const handleAutomaticUpdatesChange = useCallback(
    async (enabled: boolean) => {
      setSavingPreference(true);
      try {
        const preferences = await window.updater.setAutomaticUpdates(enabled);
        setAutomaticUpdates(preferences.automaticUpdates);
        toast.success(t(($) => $.auto_save.toast_saved), {
          id: "settings-auto-save",
        });
      } catch {
        toast.error(t(($) => $.desktop.updates.automatic_updates_save_failed));
      } finally {
        setSavingPreference(false);
      }
    },
    [t],
  );

  const handleCheck = useCallback(async () => {
    setState({ status: "checking" });
    try {
      const result = await window.updater.checkForUpdates();
      if (!result.ok) {
        setState({ status: "error", message: result.error });
        return;
      }
      setState(result.available ? { status: "available", latestVersion: result.latestVersion } : { status: "up-to-date" });
    } catch (error) {
      setState({ status: "error", message: String(error) });
    }
  }, []);

  return (
    <SettingsTab
      title={t(($) => $.desktop.updates.title)}
      description={t(($) => $.desktop.updates.description)}
    >
      <SettingsCard>
        <SettingsRow label={t(($) => $.desktop.updates.current_version)}>
          <span className="font-mono text-caption text-muted-foreground">
            v{currentVersion}
          </span>
        </SettingsRow>

        <SettingsRow
          label={t(($) => $.desktop.updates.automatic_updates_title)}
          description={t(($) => $.desktop.updates.automatic_updates_description)}
        >
          <Switch
            checked={automaticUpdates}
            onCheckedChange={handleAutomaticUpdatesChange}
            disabled={!preferencesReady || savingPreference}
            aria-label={t(($) => $.desktop.updates.automatic_updates_title)}
          />
        </SettingsRow>

        <SettingsRow
          label={t(($) => $.desktop.updates.check_section_title)}
          align="start"
          description={
            <>
              <p>{t(($) => $.desktop.updates.check_section_description)}</p>
              {state.status === "up-to-date" && update.status === "idle" && (
                <p className="mt-2 inline-flex items-center gap-1.5">
                  <Check className="size-3.5 text-success" />
                  {t(($) => $.desktop.updates.up_to_date)}
                </p>
              )}
              {state.status === "available" && update.status === "idle" && (
                <p className="mt-2 inline-flex items-center gap-1.5">
                  <ArrowDownToLine className="size-3.5 text-primary" />
                  {t(($) => $.desktop.updates.downloading, {
                    version: state.latestVersion,
                  })}
                </p>
              )}
              {state.status === "error" && (
                <p className="mt-2 inline-flex items-center gap-1.5 text-destructive">
                  <AlertCircle className="size-3.5" />
                  {state.message}
                </p>
              )}
            </>
          }
        >
          <Button
            variant="outline"
            size="sm"
            onClick={handleCheck}
            disabled={state.status === "checking" || ["downloading", "preparing", "installing"].includes(update.status)}
          >
            {state.status === "checking" ? (
              <>
                <Loader2 className="size-3.5 animate-spin" />
                {t(($) => $.desktop.updates.checking)}
              </>
            ) : (
              t(($) => $.desktop.updates.check_now)
            )}
          </Button>
        </SettingsRow>
        {update.status !== "idle" && (
          <SettingsRow label={t(($) => $.desktop.updates.status_title)} align="start" description={
            <div className="space-y-1.5">
              <p>{update.status === "downloading" ? t(($) => $.desktop.updates.download_progress, { version: update.version, percent: Math.round(update.percent) })
                : update.status === "preparing" ? t(($) => $.desktop.updates.preparing, { version: update.version })
                : update.status === "ready" ? t(($) => $.desktop.updates.ready, { version: update.version })
                : update.status === "installing" ? t(($) => $.desktop.updates.installing, { version: update.version })
                : update.status === "manual" ? t(($) => $.desktop.updates.manual, { version: update.version })
                : t(($) => $.desktop.updates.failed, { version: update.version })}</p>
              {update.status === "downloading" && <progress className="w-full h-1.5 accent-primary" value={update.percent} max={100} />}
              {update.status === "error" && <p className="break-all text-destructive">{update.message}</p>}
            </div>
          }>
            {update.status === "ready" && <Button size="sm" onClick={() => void requestUpdateInstall(t(($) => $.desktop.updates.failed_title))}>{t(($) => $.desktop.updates.restart_now)}</Button>}
            {update.status === "installing" && <Loader2 className="size-4 animate-spin text-muted-foreground" />}
          </SettingsRow>
        )}
        <SettingsRow
          label={t(($) => $.desktop.updates.manual_download_title)}
          description={t(($) => $.desktop.updates.manual_download_description)}
        >
          <Button size="sm" variant="outline" onClick={() => void openUpdateDownload(t(($) => $.desktop.updates.download_open_failed))}>
            <ExternalLink className="size-3.5" />
            {t(($) => $.desktop.updates.manual_download)}
          </Button>
        </SettingsRow>
      </SettingsCard>
    </SettingsTab>
  );
}
