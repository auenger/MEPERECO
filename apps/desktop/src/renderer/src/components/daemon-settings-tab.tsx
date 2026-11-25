import { useState, useEffect, useCallback, type ReactNode } from "react";
import { restartForServerChange } from "../platform/server-switch";
import { AlertCircle, Info, LogIn } from "lucide-react";
import { Button } from "@silieco/ui/components/ui/button";
import { Input } from "@silieco/ui/components/ui/input";
import { Switch } from "@silieco/ui/components/ui/switch";
import { cn } from "@silieco/ui/lib/utils";
import { toast } from "sonner";
import {
  SettingsCard,
  SettingsRow,
  SettingsSection,
  SettingsTab,
} from "@silieco/views/settings";
import { reauthenticateDaemon } from "../platform/daemon-reauth";
import type { DaemonPrefs, DaemonStatus } from "../../../shared/daemon-types";
import {
  DAEMON_STATE_COLORS,
  DAEMON_STATE_LABELS,
  formatUptime,
} from "../../../shared/daemon-types";

// One row inside the diagnostics block. Values that are likely to be
// long IDs / URLs render as monospaced + truncated with a tooltip.
function DiagnosticsRow({
  label,
  value,
  mono,
}: {
  label: string;
  value: ReactNode;
  mono?: boolean;
}) {
  return (
    <div className="grid grid-cols-[140px_minmax(0,1fr)] items-baseline gap-3 py-1.5">
      <span className="text-caption text-muted-foreground">{label}</span>
      <span
        className={cn(
          "min-w-0 truncate text-body",
          mono && "font-mono text-caption",
        )}
        title={typeof value === "string" ? value : undefined}
      >
        {value}
      </span>
    </div>
  );
}

export function DaemonSettingsTab() {
  const [prefs, setPrefs] = useState<DaemonPrefs>({ autoStart: true, autoStop: false });
  const [cliInstalled, setCliInstalled] = useState<boolean | null>(null);
  const [saving, setSaving] = useState(false);
  const [status, setStatus] = useState<DaemonStatus>({ state: "stopped" });
  const [reauthLoading, setReauthLoading] = useState(false);
  const [serverUrl, setServerUrl] = useState("");
  const [serverSavedUrl, setServerSavedUrl] = useState<string | null>(null);
  const [serverSaving, setServerSaving] = useState(false);

  useEffect(() => {
    window.daemonAPI.getPrefs().then(setPrefs);
    window.daemonAPI.isCliInstalled().then(setCliInstalled);
    window.daemonAPI.getStatus().then(setStatus);
    // Falls back to the DEFAULT_RUNTIME_CONFIG value when nothing is stored —
    // showing it explicitly makes "what will happen after restart" obvious.
    window.desktopConfig
      .read()
      .then((url) => {
        const runtimeConfig = window.desktopAPI.runtimeConfig;
        const effective = url ?? (runtimeConfig.ok ? runtimeConfig.config.apiUrl : "");
        setServerUrl(effective);
        setServerSavedUrl(effective);
      })
      .catch(() => undefined);
    return window.daemonAPI.onStatusChange(setStatus);
  }, []);

  const handleReauth = useCallback(async () => {
    setReauthLoading(true);
    await reauthenticateDaemon();
    setReauthLoading(false);
  }, []);

  const handleSaveServer = useCallback(async () => {
    setServerSaving(true);
    try {
      const result = await window.desktopConfig.write(serverUrl.trim());
      if (result.ok) {
        setServerSavedUrl(result.apiUrl);
        setServerUrl(result.apiUrl);
        await restartForServerChange();
      } else {
        toast.error(result.error);
      }
    } catch (error) {
      toast.error(error instanceof Error ? error.message : String(error));
    } finally {
      setServerSaving(false);
    }
  }, [serverUrl]);

  const updatePref = useCallback(
    async (key: keyof DaemonPrefs, value: boolean) => {
      setSaving(true);
      try {
        const updated = await window.daemonAPI.setPrefs({ [key]: value });
        setPrefs(updated);
        toast.success("Daemon settings saved", { id: "settings-auto-save" });
      } catch (error) {
        toast.error(
          error instanceof Error ? error.message : "Failed to save daemon settings",
        );
      } finally {
        setSaving(false);
      }
    },
    [],
  );

  // The daemon runs somewhere the app can't drive (e.g. inside WSL2 behind a
  // Windows desktop): /health is reachable but the lifecycle CLI can't reach
  // its process. Auto-start/auto-stop can't work, so disable them and say why
  // rather than letting the toggles silently no-op. See #3916.
  const externallyManaged = status.externallyManaged === true;

  return (
    <SettingsTab
      title="Daemon"
      description="Configure how the local agent daemon behaves with the desktop app."
    >
      <SettingsSection
        title="Server"
        description="API, real-time channel, daemon and automatic updates all use this address. Saving signs you out and restarts the app so you can sign in to the new server."
      >
        <SettingsCard>
          <SettingsRow
            label="Server address"
            description={
              serverSavedUrl
                ? `Current: ${serverSavedUrl}`
                : "Using the default deployment address."
            }
          >
            <div className="flex shrink-0 items-center gap-2">
              <Input
                value={serverUrl}
                onChange={(e) => setServerUrl(e.target.value)}
                className="w-64 font-mono text-caption"
                spellCheck={false}
              />
              <Button
                size="sm"
                onClick={handleSaveServer}
                disabled={
                  serverSaving ||
                  serverUrl.trim().length === 0 ||
                  serverUrl.trim() === serverSavedUrl
                }
              >
                Save & restart
              </Button>
            </div>
          </SettingsRow>
        </SettingsCard>
      </SettingsSection>

      {status.state === "auth_expired" && (
        <div className="mt-4 flex items-start gap-3 rounded-lg border border-destructive/40 bg-destructive/5 px-4 py-3">
          <AlertCircle className="mt-0.5 size-4 shrink-0 text-destructive" />
          <div className="min-w-0 flex-1">
            <p className="text-body font-medium text-destructive">
              Sign-in expired
            </p>
            <p className="mt-0.5 text-body text-muted-foreground">
              The local daemon couldn&apos;t authenticate, so this device
              can&apos;t take tasks. Sign in again to restore it.
            </p>
          </div>
          <Button
            size="sm"
            className="shrink-0"
            onClick={handleReauth}
            disabled={reauthLoading}
          >
            <LogIn className="size-3.5 mr-1.5" />
            Sign in again
          </Button>
        </div>
      )}

      {externallyManaged && (
        <div className="mt-4 flex items-start gap-3 rounded-lg border bg-muted/30 px-4 py-3">
          <Info className="mt-0.5 size-4 shrink-0 text-muted-foreground" />
          <p className="min-w-0 text-body text-muted-foreground">
            This device&apos;s daemon runs outside the app — for example inside
            WSL2 — so the app can&apos;t start or stop it. Start or stop it from
            that environment with{" "}
            <code className="font-mono text-caption">silieco daemon start</code> /{" "}
            <code className="font-mono text-caption">silieco daemon stop</code>.
          </p>
        </div>
      )}

      <SettingsCard>
        <SettingsRow
          label="Auto-start on launch"
          description="Automatically start the daemon when the app opens and you are logged in."
        >
          <Switch
            checked={prefs.autoStart}
            onCheckedChange={(checked) => updatePref("autoStart", checked)}
            disabled={saving || externallyManaged}
          />
        </SettingsRow>

        <SettingsRow
          label="Auto-stop on quit"
          description="Stop the daemon when the desktop app is closed. Disable this to keep the daemon running in the background."
        >
          <Switch
            checked={prefs.autoStop}
            onCheckedChange={(checked) => updatePref("autoStop", checked)}
            disabled={saving || externallyManaged}
          />
        </SettingsRow>

        <SettingsRow
          label="CLI Status"
          description={
            cliInstalled === null
              ? "Checking…"
              : cliInstalled
                ? "silieco CLI is installed and available."
                : "silieco CLI not found. It ships bundled with this app — restart the app and the daemon will pick it up."
          }
        >
          {cliInstalled !== false && <span />}
        </SettingsRow>
      </SettingsCard>

      {/* Diagnostics — moved out of the logs panel so the panel can focus
          on logs. These fields matter for support tickets and bug reports,
          not for everyday use. */}
      <SettingsSection
        title="Diagnostics"
        description="Identification and connection details. Useful when filing a bug report or investigating why a runtime isn't showing up."
      >
        <SettingsCard>
          <div className="px-4 py-2">
          <DiagnosticsRow
            label="State"
            value={
              <span className="inline-flex items-center gap-1.5">
                <span
                  className={cn(
                    "size-1.5 rounded-full",
                    DAEMON_STATE_COLORS[status.state],
                  )}
                />
                {DAEMON_STATE_LABELS[status.state]}
              </span>
            }
          />
          <DiagnosticsRow
            label="Uptime"
            value={status.uptime ? formatUptime(status.uptime) : "—"}
          />
          <DiagnosticsRow
            label="PID"
            value={status.pid ?? "—"}
            mono={!!status.pid}
          />
          <DiagnosticsRow
            label="Daemon ID"
            value={status.daemonId ?? "—"}
            mono={!!status.daemonId}
          />
          <DiagnosticsRow
            label="Profile"
            value={status.profile || "default"}
          />
          <DiagnosticsRow
            label="Server URL"
            value={status.serverUrl ?? "—"}
            mono={!!status.serverUrl}
          />
          <DiagnosticsRow
            label="Device name"
            value={status.deviceName ?? "—"}
          />
          <DiagnosticsRow
            label="Workspaces"
            value={
              typeof status.workspaceCount === "number"
                ? status.workspaceCount
                : "—"
            }
          />
          </div>
        </SettingsCard>
      </SettingsSection>
    </SettingsTab>
  );
}
