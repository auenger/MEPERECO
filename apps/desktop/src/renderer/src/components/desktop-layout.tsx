import { BrowserToolLifecycle } from "@/platform/browser-tool-entry";
import { useEffect, useRef, useSyncExternalStore } from "react";
import { ChevronLeft, ChevronRight, SquarePen } from "lucide-react";
import { motion } from "motion/react";
import { cn } from "@silieco/ui/lib/utils";
import { SiliecoIcon } from "@silieco/ui/components/common/silieco-icon";
import { useTabHistory } from "@/hooks/use-tab-history";
import {
  SidebarProvider,
  SidebarTrigger,
  useSidebar,
} from "@silieco/ui/components/ui/sidebar";
import { ModalRegistry } from "@silieco/views/modals/registry";
import { AppSidebar, GlobalShortcuts } from "@silieco/views/layout";
import { Tooltip, TooltipTrigger, TooltipContent } from "@silieco/ui/components/ui/tooltip";
import { openCreateTaskWithPreference } from "@silieco/core/tasks/stores/create-mode-store";
import { useTaskDraftStore } from "@silieco/core/tasks/stores/draft-store";
import { useShortcut, formatShortcut } from "@silieco/core/shortcuts";
import { useT } from "@silieco/views/i18n";
import { SearchCommand, SearchTrigger } from "@silieco/views/search";
import { FloatingChat, ChatToolbarButton } from "@silieco/views/chat";
import { WorkspaceSlugProvider, paths } from "@silieco/core/paths";
import { useNavigation } from "@silieco/views/navigation";
import { getCurrentSlug, subscribeToCurrentSlug } from "@silieco/core/platform";
import { useDesktopUnreadBadge, useMessageUnreadCounts } from "@silieco/views/platform";
import {
  DesktopNavigationProvider,
  routeContentLinkPath,
} from "@/platform/navigation";
import { TabBar } from "./tab-bar";
import { TabContent } from "./tab-content";
import { WindowOverlay } from "./window-overlay";
import { useDesktopUpdateNotice } from "./use-desktop-update";

const TOP_BAR_HEIGHT_CLASS = "h-12";
const WINDOW_TOOLBAR_CLEARANCE = 184;
// Windows runs titleless with the native Window Controls Overlay at the
// top-RIGHT (see main/index.ts), so unlike macOS there are no traffic
// lights to clear at the top-left — the brand row folds into the nav strip.
const IS_WINDOWS = window.desktopAPI.appInfo.os === "windows";
const toolbarMotion = {
  type: "spring",
  stiffness: 420,
  damping: 38,
  mass: 0.8,
} as const;

function WindowToolbar() {
  const { canGoBack, canGoForward, goBack, goForward } = useTabHistory();
  const navButtonClassName =
    "flex size-7 items-center justify-center rounded-md text-muted-foreground/70 transition-colors hover:bg-sidebar-accent hover:text-sidebar-accent-foreground disabled:pointer-events-none disabled:opacity-30";

  return (
    <div
      className={cn(
        "fixed left-0 top-0 z-30 flex w-[184px] shrink-0 items-center px-3",
        TOP_BAR_HEIGHT_CLASS,
      )}
      style={{ WebkitAppRegion: "drag" } as React.CSSProperties}
    >
      <div
        className="flex items-center gap-1 pl-[70px]"
        style={{ WebkitAppRegion: "no-drag" } as React.CSSProperties}
      >
        <SidebarTrigger
          className="size-7 text-muted-foreground/70 hover:bg-sidebar-accent hover:text-sidebar-accent-foreground"
          style={{ WebkitAppRegion: "no-drag" } as React.CSSProperties}
        />
        <div className="flex items-center gap-1">
          <button
            type="button"
            onClick={goBack}
            disabled={!canGoBack}
            aria-label="Go back"
            title="Go back"
            className={navButtonClassName}
            style={{ WebkitAppRegion: "no-drag" } as React.CSSProperties}
          >
            <ChevronLeft className="size-4" />
          </button>
          <button
            type="button"
            onClick={goForward}
            disabled={!canGoForward}
            aria-label="Go forward"
            title="Go forward"
            className={navButtonClassName}
            style={{ WebkitAppRegion: "no-drag" } as React.CSSProperties}
          >
            <ChevronRight className="size-4" />
          </button>
        </div>
      </div>
    </div>
  );
}

function SidebarTopSpacer() {
  return <div className={cn("shrink-0", TOP_BAR_HEIGHT_CLASS)} />;
}

// Windows top-left strip: brand + sidebar collapse + back/forward in one
// in-flow row inside the sidebar (macOS keeps the fixed 184px overlay
// toolbar with traffic-light clearance instead). The whole strip is a drag
// region; individual controls opt out.
function WindowsSidebarTopBar() {
  const { t } = useT("layout");
  const { canGoBack, canGoForward, goBack, goForward } = useTabHistory();
  const navButtonClassName =
    "flex size-7 items-center justify-center rounded-md text-muted-foreground/70 transition-colors hover:bg-sidebar-accent hover:text-sidebar-accent-foreground disabled:pointer-events-none disabled:opacity-30";

  return (
    <div
      className={cn("flex shrink-0 items-center gap-1 pl-2 pr-3 group-data-[collapsible=icon]:justify-center group-data-[collapsible=icon]:px-0", TOP_BAR_HEIGHT_CLASS)}
      style={{ WebkitAppRegion: "drag" } as React.CSSProperties}
    >
      <div
        className="flex min-w-0 flex-1 items-center gap-2 px-1 group-data-[collapsible=icon]:hidden"
        style={{ WebkitAppRegion: "no-drag" } as React.CSSProperties}
      >
        <SiliecoIcon noSpin variant="small" className="size-7 shrink-0" />
        <p className="min-w-0 flex-1 truncate text-caption font-medium text-muted-foreground group-data-[collapsible=icon]:hidden">
          {t(($) => $.sidebar.brand_tagline)}
        </p>
      </div>
      <SidebarTrigger
        className="size-7 shrink-0 text-muted-foreground/70 hover:bg-sidebar-accent hover:text-sidebar-accent-foreground"
        style={{ WebkitAppRegion: "no-drag" } as React.CSSProperties}
      />
      <div
        className="flex shrink-0 items-center gap-1 group-data-[collapsible=icon]:hidden"
        style={{ WebkitAppRegion: "no-drag" } as React.CSSProperties}
      >
        <button
          type="button"
          onClick={goBack}
          disabled={!canGoBack}
          aria-label="Go back"
          title="Go back"
          className={navButtonClassName}
        >
          <ChevronLeft className="size-4" />
        </button>
        <button
          type="button"
          onClick={goForward}
          disabled={!canGoForward}
          aria-label="Go forward"
          title="Go forward"
          className={navButtonClassName}
        >
          <ChevronRight className="size-4" />
        </button>
      </div>
    </div>
  );
}

function useNativeNavigationGestures() {
  const { goBack, goForward } = useTabHistory();

  useEffect(() => {
    return window.desktopAPI.onNavigationGesture((gesture) => {
      if (gesture === "back") {
        goBack();
      } else {
        goForward();
      }
    });
  }, [goBack, goForward]);
}

function NewTaskToolbarButton() {
  const { t } = useT("layout");
  const shortcut = useShortcut("createTask");
  const hasDraft = useTaskDraftStore((s) => s.hasDraft());
  const label = t(($) => $.sidebar.new_task);

  return (
    <Tooltip>
      <TooltipTrigger
        aria-label={label}
        onClick={() => openCreateTaskWithPreference()}
        className="relative flex size-8 shrink-0 items-center justify-center rounded-md text-muted-foreground transition-colors hover:bg-sidebar-accent hover:text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
        style={{ WebkitAppRegion: "no-drag" } as React.CSSProperties}
      >
        <SquarePen className="size-5" />
        {hasDraft && (
          <span aria-hidden="true" className="absolute right-0.5 top-0.5 size-1.5 rounded-full bg-brand" />
        )}
      </TooltipTrigger>
      <TooltipContent side="bottom" sideOffset={10}>
        {label}{shortcut ? ` · ${formatShortcut(shortcut)}` : ""}
      </TooltipContent>
    </Tooltip>
  );
}

// The main area's top bar doubles as a window drag region. When the sidebar
// is not occupying main-flow width, leave room for the fixed window toolbar
// so tabs do not land beneath the traffic lights / navigation controls.
function MainTopBar({ showChat, settingsNotice }: { showChat: boolean; settingsNotice?: string }) {
  const { state, isMobile } = useSidebar();
  const { chatUnreadCount, inboxUnreadCount } = useMessageUnreadCounts();
  const { t } = useT("chat");
  // Windows keeps its nav strip inside the sidebar flow, so nothing fixed
  // ever overlaps the tab bar and no clearance is needed.
  const toolbarClearance = IS_WINDOWS
    ? 0
    : isMobile
      ? WINDOW_TOOLBAR_CLEARANCE
      : state === "collapsed"
        ? "calc(184px - var(--sidebar-width-icon) - 1rem)"
        : 0;

  return (
    <motion.header
      animate={{ paddingLeft: toolbarClearance }}
      className={cn("relative shrink-0 flex items-center gap-2", TOP_BAR_HEIGHT_CLASS)}
      style={{ paddingRight: IS_WINDOWS ? "max(144px, calc(100vw - env(titlebar-area-x, 0px) - env(titlebar-area-width, 100vw)))" : 8 }}
      initial={false}
      transition={toolbarMotion}
    >
      <motion.div
        aria-hidden
        animate={{ left: toolbarClearance }}
        className="absolute inset-y-0 right-0"
        initial={false}
        transition={toolbarMotion}
        style={{ WebkitAppRegion: "drag" } as React.CSSProperties}
      />
      <div className="relative z-10 flex h-full min-w-0 flex-1 items-center overflow-hidden">
        <TabBar settingsNotice={settingsNotice}
          chatNotice={chatUnreadCount > 0 ? t(($) => $.list.unread_notice, { count: chatUnreadCount }) : undefined}
          inboxNotice={inboxUnreadCount > 0 ? t(($) => $.list.unread_notice, { count: inboxUnreadCount }) : undefined}
        />
      </div>
      <div className="relative z-10 flex h-full shrink-0 items-center justify-center gap-1 pr-1" style={{ WebkitAppRegion: "no-drag" } as React.CSSProperties}>
        {showChat && (
          <>
            <NewTaskToolbarButton />
            <div className="flex size-8 shrink-0 items-center justify-center">
              <ChatToolbarButton />
            </div>
          </>
        )}
      </div>
    </motion.header>
  );
}

// The canvas hugs the expanded sidebar with a hairline gap. When the sidebar
// leaves the main flow, the left margin must grow to mirror the fixed mr-2 so
// the floating canvas sits symmetrically inside the window frame.
function MainCanvas({ children }: { children: React.ReactNode }) {
  const { isMobile } = useSidebar();

  return (
    <motion.div
      animate={{ marginLeft: isMobile ? 8 : 2 }}
      className="relative flex flex-1 min-h-0 flex-col overflow-hidden mr-2 mb-2 rounded-xl bg-page-canvas ring-1 ring-surface-border shadow-[var(--surface-shadow)]"
      initial={false}
      transition={toolbarMotion}
    >
      {children}
    </motion.div>
  );
}

function useInternalLinkHandler() {
  useEffect(() => {
    const handler = (e: Event) => {
      const path = (e as CustomEvent).detail?.path;
      if (!path) return;
      routeContentLinkPath(path);
    };
    window.addEventListener("silieco:navigate", handler);
    return () => window.removeEventListener("silieco:navigate", handler);
  }, []);
}

/**
 * Bridge between the renderer and the Electron main process for inbox-level
 * OS integration. Mounted inside WorkspaceSlugProvider so it can resolve the
 * current workspace's id for the badge hook.
 *
 * Two responsibilities:
 *   1. Mirror the unread inbox count onto the dock/taskbar badge.
 *   2. When the user clicks an OS notification, open the notified
 *      workspace's inbox focused on that item. The route uses the `slug`
 *      that the notification was *emitted* with — not the currently active
 *      workspace — so a notification from workspace A always opens A's
 *      inbox even if the user has since switched to workspace B. Marking
 *      the row read is handled by InboxPage's selected-item effect, which
 *      covers both click-to-select and URL-param-select paths.
 *
 * The click routes through `useNavigation().push` — NOT the
 * `silieco:navigate` event, whose handler `openTab`s into the ACTIVE
 * workspace's tab group. The navigation adapter detects a cross-workspace
 * path and translates it into `switchWorkspace(slug, path)`, so clicking a
 * workspace-A notification while B is active performs a real workspace
 * switch instead of mounting A's inbox inside B's tab group (#3766).
 */
function DesktopInboxBridge() {
  const { chatUnreadCount, inboxUnreadCount, workspaceSlug, unreadChatIds, unreadInboxKeys } = useMessageUnreadCounts();
  useDesktopUnreadBadge(chatUnreadCount + inboxUnreadCount);
  useEffect(() => {
    if (!workspaceSlug) return;
    window.desktopAPI.reconcileNotifications({
      slug: workspaceSlug,
      unreadChatIds,
      unreadInboxKeys,
    });
  }, [workspaceSlug, unreadChatIds, unreadInboxKeys]);
  const { push } = useNavigation();
  // The adapter identity changes with the active tab's location; the ref
  // keeps the main-process subscription stable across navigations.
  const pushRef = useRef(push);
  useEffect(() => {
    pushRef.current = push;
  }, [push]);

  useEffect(() => {
    return window.desktopAPI.onInboxOpen(({ slug, taskKey, kind }) => {
      if (!slug) return;
      const p = paths.workspace(slug);
      pushRef.current(kind === "chat"
        ? `${p.chat()}?session=${encodeURIComponent(taskKey)}`
        : `${p.inbox()}?task=${encodeURIComponent(taskKey)}`);
    });
  }, []);

  return null;
}

export function DesktopShell() {
  useInternalLinkHandler();
  useNativeNavigationGestures();
  const settingsNotice = useDesktopUpdateNotice();

  // Reactive read of current workspace slug from the platform singleton.
  // On first mount, slug is null until WorkspaceRouteLayout (inside the tab
  // router) sets it. Once set, the sidebar and other shell-level components
  // can resolve workspace-scoped paths via useWorkspacePaths().
  const slug = useSyncExternalStore(subscribeToCurrentSlug, getCurrentSlug, () => null);

  return (
    <DesktopNavigationProvider>
      <BrowserToolLifecycle />
      {/* WorkspaceSlugProvider accepts null — components that need slug
          use useWorkspaceSlug() (nullable) or useRequiredWorkspaceSlug()
          (throws). TabContent MUST always render so the tab router can
          mount WorkspaceRouteLayout, which calls setCurrentWorkspace()
          to populate the slug. The sidebar gates on slug being present
          to avoid the useRequiredWorkspaceSlug throw. Zero-workspace
          users see the window-level overlay (new-workspace flow)
          triggered by IndexRedirect, not a route. */}
      <WorkspaceSlugProvider slug={slug}>
        <DesktopInboxBridge />
        <span role="status" className="sr-only">{settingsNotice}</span>
        <div className="flex h-screen bg-app-shell">
          <SidebarProvider className="flex-1 bg-app-shell">
            {slug && <GlobalShortcuts />}
            {slug && !IS_WINDOWS && <WindowToolbar />}
            {slug && (
              <AppSidebar
                topSlot={IS_WINDOWS ? <WindowsSidebarTopBar /> : <SidebarTopSpacer />}
                searchSlot={<SearchTrigger />}
                hideBrand={IS_WINDOWS}
                taskInToolbar
                settingsNotice={settingsNotice}
              />
            )}
            {/* Right side: header + content container */}
            <div className="flex flex-1 min-w-0 flex-col">
              <MainTopBar showChat={Boolean(slug)} settingsNotice={settingsNotice} />
              <MainCanvas>
                <TabContent />
                {slug && <FloatingChat showLauncher={false} />}
              </MainCanvas>
            </div>
          </SidebarProvider>
        </div>
        {slug && <ModalRegistry />}
        {slug && <SearchCommand />}
        <WindowOverlay />
      </WorkspaceSlugProvider>
    </DesktopNavigationProvider>
  );
}
