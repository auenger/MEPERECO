import { useEffect } from "react";

type BadgeCapableAPI = {
  setUnreadBadge?: (count: number) => void;
};

function getDesktopAPI(): BadgeCapableAPI | undefined {
  if (typeof window === "undefined") return undefined;
  return (window as unknown as { desktopAPI?: BadgeCapableAPI }).desktopAPI;
}

/**
 * Mirror the combined chat and inbox unread count onto the OS dock/taskbar badge. No-op on web
 * (no `desktopAPI`) and on the login screen (no workspace ⇒ count defaults
 * to 0, which clears any stale badge from a previous session).
 */
export function useDesktopUnreadBadge(count: number): void {
  useEffect(() => {
    getDesktopAPI()?.setUnreadBadge?.(count);
  }, [count]);
}
