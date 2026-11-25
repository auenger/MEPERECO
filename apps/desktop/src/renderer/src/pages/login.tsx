import { useState } from "react";
import { restartForServerChange } from "../platform/server-switch";
import { LoginPage } from "@silieco/views/auth";
import { DragStrip } from "@silieco/views/platform";
import { SiliecoIcon } from "@silieco/ui/components/common/silieco-icon";
import { Button } from "@silieco/ui/components/ui/button";
import { Input } from "@silieco/ui/components/ui/input";

function requireRuntimeAppUrl(): string {
  const runtimeConfig = window.desktopAPI.runtimeConfig;
  if (!runtimeConfig.ok) {
    throw new Error(
      "Invariant violated: DesktopLoginPage rendered before App accepted runtime config",
    );
  }
  return runtimeConfig.config.appUrl;
}

export function DesktopLoginPage() {
  const appUrl = requireRuntimeAppUrl();
  const [serverUrl, setServerUrl] = useState(() => {
    const runtimeConfig = window.desktopAPI.runtimeConfig;
    return runtimeConfig.ok ? runtimeConfig.config.apiUrl : "";
  });
  const [editingServer, setEditingServer] = useState(false);
  const [serverSaving, setServerSaving] = useState(false);
  const [serverError, setServerError] = useState<string | null>(null);

  const handleGoogleLogin = () => {
    // Open web login page in the default browser with platform=desktop flag.
    // The web callback will redirect back via silieco:// deep link with the token.
    window.desktopAPI.openExternal(
      `${appUrl}/login?platform=desktop`,
    );
  };

  const handleSaveServer = async () => {
    setServerSaving(true);
    setServerError(null);
    try {
      const result = await window.desktopConfig.write(serverUrl.trim());
      if (result.ok) {
        setServerUrl(result.apiUrl);
        // CoreProvider initializes the API and auth singletons once per
        // process. Restart the app so API, WebSocket and update feed agree.
        await restartForServerChange();
      } else {
        setServerError(result.error);
      }
    } catch (error) {
      setServerError(error instanceof Error ? error.message : String(error));
    } finally {
      setServerSaving(false);
    }
  };

  return (
    <div className="flex h-screen flex-col">
      <DragStrip />
      <LoginPage
        logo={<SiliecoIcon bordered size="lg" />}
        passwordAuth
        onSuccess={() => {
          // Auth store update triggers AppContent re-render → shows DesktopShell.
          // Initial workspace navigation happens in routes.tsx via IndexRedirect.
        }}
        onGoogleLogin={handleGoogleLogin}
      />
      <div className="flex flex-col items-center gap-1.5 pb-3">
        {editingServer ? (
          <div className="flex items-center gap-1.5">
            <Input
              value={serverUrl}
              onChange={(e) => setServerUrl(e.target.value)}
              className="h-7 w-72 font-mono text-caption"
              spellCheck={false}
              autoFocus
              onKeyDown={(e) => {
                if (e.key === "Enter" && !serverSaving && serverUrl.trim()) void handleSaveServer();
              }}
            />
            <Button
              size="sm"
              variant="outline"
              onClick={() => void handleSaveServer()}
              disabled={serverSaving || serverUrl.trim().length === 0}
            >
              Save & restart
            </Button>
            <Button
              size="sm"
              variant="ghost"
              onClick={() => {
                setEditingServer(false);
                setServerError(null);
                const runtimeConfig = window.desktopAPI.runtimeConfig;
                setServerUrl(
                  runtimeConfig.ok ? runtimeConfig.config.apiUrl : serverUrl,
                );
              }}
              disabled={serverSaving}
            >
              Cancel
            </Button>
          </div>
        ) : (
          <div className="flex items-center gap-2 text-caption text-muted-foreground">
            <span className="font-mono">{serverUrl || "—"}</span>
            <button
              type="button"
              onClick={() => setEditingServer(true)}
              className="underline underline-offset-2 transition-colors hover:text-foreground"
            >
              Change server
            </button>
          </div>
        )}
        {serverError && (
          <p className="max-w-md text-caption text-destructive">{serverError}</p>
        )}
        <p className="text-caption text-muted-foreground/70">
          Signing in connects this app to the server above.
        </p>
      </div>
    </div>
  );
}
