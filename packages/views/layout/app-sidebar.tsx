"use client";

import React, { useCallback, useEffect, useRef, useState } from "react";
import { cn } from "@silieco/ui/lib/utils";
import { useScrollFade } from "@silieco/ui/hooks/use-scroll-fade";
import { AppLink, useNavigation } from "../navigation";
import {
  DndContext,
  PointerSensor,
  useSensor,
  useSensors,
  closestCenter,
  type DragEndEvent,
} from "@dnd-kit/core";
import { SortableContext, verticalListSortingStrategy, useSortable, arrayMove } from "@dnd-kit/sortable";
import { CSS } from "@dnd-kit/utilities";
import {
  ChevronDown,
  ChevronRight,
  FileText,
  FolderKanban,
  Layers3,
  GitBranch,
  LogOut,
  Users,
  Settings,
  Plus,
  Route,
  SquarePen,
  X,
} from "lucide-react";
import { CollapsedProjects } from "./collapsed-projects";
import { WorkspaceAvatar } from "../workspace/workspace-avatar";
import { ActorAvatar } from "@silieco/ui/components/common/actor-avatar";
import { SiliecoIcon } from "@silieco/ui/components/common/silieco-icon";
import { Tooltip, TooltipTrigger, TooltipContent } from "@silieco/ui/components/ui/tooltip";
import { Collapsible, CollapsibleTrigger, CollapsibleContent } from "@silieco/ui/components/ui/collapsible";
import { CappedNumberFlow } from "@silieco/ui/components/ui/number-flow";
import { StatusIcon } from "../tasks/components/status-icon";
import { useTaskDraftStore } from "@silieco/core/tasks/stores/draft-store";
import { openCreateTaskWithPreference } from "@silieco/core/tasks/stores/create-mode-store";
import {
  Sidebar,
  SidebarContent,
  SidebarGroup,
  SidebarGroupLabel,
  SidebarHeader,
  SidebarFooter,
  SidebarMenu,
  SidebarMenuButton,
  SidebarMenuItem,
  SidebarRail,
  useSidebar,
} from "@silieco/ui/components/ui/sidebar";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@silieco/ui/components/ui/dropdown-menu";
import { useAuthStore } from "@silieco/core/auth";
import { useCurrentWorkspace, useWorkspacePaths, paths } from "@silieco/core/paths";
import { workspaceListOptions, myInvitationListOptions, workspaceKeys } from "@silieco/core/workspace/queries";
import { resolvePublicFileUrl } from "@silieco/core/workspace/avatar-url";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { inboxUnreadSummaryOptions, unreadWorkspaceIds } from "@silieco/core/inbox/queries";
import { api, ApiError } from "@silieco/core/api";
import { useModalStore } from "@silieco/core/modals";
import { useConfigStore } from "@silieco/core/config";
import { pinListOptions } from "@silieco/core/pins/queries";
import { useDeletePin, useReorderPins } from "@silieco/core/pins/mutations";
import { taskDetailOptions } from "@silieco/core/tasks/queries";
import { projectDetailOptions } from "@silieco/core/projects/queries";
import { projectListOptions } from "@silieco/core/projects/queries";
import { workflowInstancesOptions, useSopVisibilityStore } from "@silieco/core/workflows";
import type { PinnedItem } from "@silieco/core/types";
import { useLogout } from "../auth";
import { ProjectIcon } from "../projects/components/project-icon";
import { routeIconForPath } from "./route-icon-components";
import { useT } from "../i18n";
import {
  useShortcut,
} from "@silieco/core/shortcuts";
import { ShortcutKeycaps } from "../common/shortcut-keycaps";
import { useMessageUnreadCounts } from "../platform/use-message-unread-counts";

// Top-level nav items stay active when the user is on a child route
// (e.g. "Projects" stays lit on /:slug/projects/:id). Pinned items keep
// strict equality elsewhere — a pinned project shouldn't highlight on
// sub-pages of itself.
function isNavActive(pathname: string, href: string): boolean {
  return pathname === href || pathname.startsWith(href + "/");
}

// Stable empty arrays for query defaults. Using an inline `= []` default on
// `useQuery` creates a new array reference on every render when `data` is
// undefined (e.g. query disabled or loading) — which in turn breaks any
// `useEffect`/`useMemo` that depends on the value, and can trigger infinite
// re-render loops when the effect itself calls `setState`.
const EMPTY_PINS: PinnedItem[] = [];
const EMPTY_WORKSPACES: Awaited<ReturnType<typeof api.listWorkspaces>> = [];
const EMPTY_INVITATIONS: Awaited<ReturnType<typeof api.listMyInvitations>> = [];
const EMPTY_INBOX_SUMMARY: Awaited<ReturnType<typeof api.getInboxUnreadSummary>> = [];

// Nav items reference WorkspacePaths method names so they can be resolved
// against the current workspace slug at render time (see AppSidebar body).
// Only parameterless paths are valid nav destinations.
type NavKey =
  | "inbox"
  | "chat"
  | "myTasks"
  | "tasks"
  | "projects"
  | "resources"
  | "git"
  | "workflows"
  | "autopilots"
  | "agents"
  | "squads"
  | "usage"
  | "runtimes"
  | "skills"
  | "settings";

// Static schema (key only) — labels resolved at render via useT("layout"),
// icons derived from the destination path via routeIconForPath.
type NavLabelKey =
  | "inbox"
  | "chat"
  | "my_tasks"
  | "tasks"
  | "projects"
  | "resources"
  | "git"
  | "workflows"
  | "autopilots"
  | "agents"
  | "squads"
  | "usage"
  | "runtimes"
  | "skills"
  | "settings";

// Nav icons are NOT declared here: they are derived from each item's
// destination path at render time, so the sidebar and the desktop tab bar
// always agree. See route-icon-components.tsx.
const personalNav: { key: NavKey; labelKey: NavLabelKey }[] = [
  { key: "chat", labelKey: "chat" },
  { key: "inbox", labelKey: "inbox" },
  { key: "myTasks", labelKey: "my_tasks" },
];

const workspaceNav: { key: NavKey; labelKey: NavLabelKey }[] = [
  { key: "tasks", labelKey: "tasks" },
  { key: "resources", labelKey: "resources" },
];

const managementNav: { key: NavKey; labelKey: NavLabelKey }[] = [
  { key: "workflows", labelKey: "workflows" },
  { key: "autopilots", labelKey: "autopilots" },
  { key: "agents", labelKey: "agents" },
  { key: "squads", labelKey: "squads" },
  { key: "usage", labelKey: "usage" },
];

function DraftDot() {
  const hasDraft = useTaskDraftStore((s) => s.hasDraft());
  if (!hasDraft) return null;
  return <span className="absolute top-0 right-0 size-1.5 rounded-full bg-brand" />;
}

/**
 * Presentational pin row. The `label` and `iconNode` are computed by the
 * parent `PinRow` from cached task / project detail queries — keeping
 * this component dumb means the dnd-kit / navigation wiring lives in
 * one place and the data flow is explicit.
 */
function SortablePinItem({
  pin,
  href,
  pathname,
  onUnpin,
  label,
  iconNode,
}: {
  pin: PinnedItem;
  href: string;
  pathname: string;
  onUnpin: () => void;
  label: string;
  iconNode: React.ReactNode;
}) {
  const { t } = useT("layout");
  const { attributes, listeners, setNodeRef, transform, transition, isDragging } = useSortable({ id: pin.id });
  const wasDragged = useRef(false);

  useEffect(() => {
    if (isDragging) wasDragged.current = true;
  }, [isDragging]);

  const style = { transform: CSS.Transform.toString(transform), transition };
  const isActive = pathname === href;

  return (
    <SidebarMenuItem
      ref={setNodeRef}
      style={style}
      className={cn("group/pin", isDragging && "opacity-30")}
      {...attributes}
      {...listeners}
    >
      <SidebarMenuButton
        size="sm"
        tooltip={label}
        aria-label={label}
        isActive={isActive}
        render={<AppLink openInAppTab href={href} draggable={false} />}
        onClick={(event) => {
          if (wasDragged.current) {
            wasDragged.current = false;
            event.preventDefault();
            return;
          }
        }}
        className={cn(
          "text-muted-foreground hover:not-data-active:bg-sidebar-accent/70 data-active:bg-sidebar-accent data-active:text-sidebar-accent-foreground",
          isDragging && "pointer-events-none",
        )}
      >
        {iconNode}
        <span
          className="min-w-0 flex-1 overflow-hidden whitespace-nowrap group-data-[collapsible=icon]:hidden"
          style={{
            maskImage: "linear-gradient(to right, black calc(100% - 12px), transparent)",
            WebkitMaskImage: "linear-gradient(to right, black calc(100% - 12px), transparent)",
          }}
        >{label}</span>
        <Tooltip>
          <TooltipTrigger
            render={<span role="button" />}
            className="hidden size-2.5 shrink-0 items-center justify-center rounded-sm text-muted-foreground group-hover/pin:flex hover:text-foreground group-data-[collapsible=icon]:!hidden"
            onClick={(event) => {
              event.preventDefault();
              event.stopPropagation();
              onUnpin();
            }}
          >
            <X className="size-1" />
          </TooltipTrigger>
          <TooltipContent side="top" sideOffset={4}>{t(($) => $.sidebar.unpin_tooltip)}</TooltipContent>
        </Tooltip>
      </SidebarMenuButton>
    </SidebarMenuItem>
  );
}

/**
 * Smart wrapper that resolves a pin's display data (label + status/icon)
 * from the task / project detail query cache. Both queries are declared
 * unconditionally with `enabled` gates so the hook order stays stable
 * regardless of `pin.item_type`.
 *
 * Loading: render a flat skeleton so the sidebar height doesn't jump.
 * Missing (deleted item / 404): render nothing — the row hides itself
 * until the user unpins manually or a server-side cascade catches up.
 */
function PinRow({
  pin,
  href,
  pathname,
  onUnpin,
  wsId,
}: {
  pin: PinnedItem;
  href: string;
  pathname: string;
  onUnpin: () => void;
  wsId: string;
}) {
  const isTask = pin.item_type === "task";
  const taskQuery = useQuery({
    ...taskDetailOptions(wsId, pin.item_id),
    enabled: isTask,
  });
  const projectQuery = useQuery({
    ...projectDetailOptions(wsId, pin.item_id),
    enabled: !isTask,
  });

  const triggeredRef = useRef(false);
  useEffect(() => {
    const err = isTask ? taskQuery.error : projectQuery.error;
    if (err instanceof ApiError && err.status === 404 && !triggeredRef.current) {
      triggeredRef.current = true;
      onUnpin();
    }
  }, [isTask, taskQuery.error, onUnpin, projectQuery.error]);

  if (isTask) {
    if (taskQuery.isPending) return <PinSkeleton />;
    if (taskQuery.isError || !taskQuery.data) return null;
    const task = taskQuery.data;
    const label = task.title;
    const iconNode = (
      /* Override parent [&_svg]:size-4 — pinned items need smaller icons to match sm size */
      <StatusIcon status={task.status} className="!size-3.5 shrink-0" />
    );
    return (
      <SortablePinItem
        pin={pin}
        href={href}
        pathname={pathname}
        onUnpin={onUnpin}
        label={label}
        iconNode={iconNode}
      />
    );
  }

  if (projectQuery.isPending) return <PinSkeleton />;
  if (projectQuery.isError || !projectQuery.data) return null;
  const project = projectQuery.data;
  const iconNode = <ProjectIcon project={project} size="sm" />;
  return (
    <SortablePinItem
      pin={pin}
      href={href}
      pathname={pathname}
      onUnpin={onUnpin}
      label={project.title}
      iconNode={iconNode}
    />
  );
}

function PinSkeleton() {
  return (
    <SidebarMenuItem>
      <div className="flex h-7 w-full items-center gap-2 px-2">
        <div className="size-3.5 shrink-0 rounded-sm bg-sidebar-accent/40" />
        <div className="h-3 w-24 rounded bg-sidebar-accent/40" />
      </div>
    </SidebarMenuItem>
  );
}

interface AppSidebarProps {
  /** Rendered above SidebarHeader (e.g. desktop traffic light spacer) */
  topSlot?: React.ReactNode;
  /** Rendered in the header between workspace switcher and new-task button (e.g. search trigger) */
  searchSlot?: React.ReactNode;
  /** Extra className for SidebarHeader */
  headerClassName?: string;
  /** Extra style for SidebarHeader */
  headerStyle?: React.CSSProperties;
  /**
   * Desktop Windows only: the brand row is folded into the window's top-left
   * bar (no traffic lights there, unlike macOS), so the in-sidebar copy is
   * redundant and gets hidden.
   */
  hideBrand?: boolean;
  /** Desktop provides the new-task action in its window toolbar. */
  taskInToolbar?: boolean;
  /** Localized attention notice supplied by the desktop updater. */
  settingsNotice?: string;
}

export function AppSidebar({ topSlot, searchSlot, headerClassName, headerStyle, hideBrand, taskInToolbar = false, settingsNotice }: AppSidebarProps = {}) {
  const { t } = useT("layout");
  const sopEnabled = useSopVisibilityStore((state) => state.enabled);
  const visibleManagementNav = managementNav.filter((item) => sopEnabled || item.key !== "workflows");
  const settingsLabel = t(($) => $.nav.settings);
  const settingsAccessibleLabel = settingsNotice ? `${settingsLabel} · ${settingsNotice}` : settingsLabel;
  const { state, isMobile, setOpen } = useSidebar();
  const iconOnly = state === "collapsed" && !isMobile;
  const { pathname, push, openInNewTab, searchParams } = useNavigation();
  const user = useAuthStore((s) => s.user);
  const userId = useAuthStore((s) => s.user?.id);
  const logout = useLogout();
  const workspace = useCurrentWorkspace();
  const p = useWorkspacePaths();
  const { data: workspaces = EMPTY_WORKSPACES } = useQuery(workspaceListOptions());
  const { data: myInvitations = EMPTY_INVITATIONS } = useQuery(myInvitationListOptions());
  const workspaceCreationDisabled = useConfigStore((s) => s.workspaceCreationDisabled);

  const wsId = workspace?.id;
  const { inboxUnreadCount: unreadCount, chatUnreadCount } = useMessageUnreadCounts();
  // Cross-workspace unread summary backs the workspace-switcher dot. One
  // shared cache entry across workspaces; gated on an active workspace since
  // the endpoint resolves through the workspace-member middleware.
  const { data: unreadSummary = EMPTY_INBOX_SUMMARY } = useQuery({
    ...inboxUnreadSummaryOptions(),
    enabled: !!wsId,
  });
  // Which workspaces have unread, so the switcher dropdown can point at the
  // specific one(s) rather than just the aggregate avatar dot.
  const unreadWsIds = React.useMemo(() => unreadWorkspaceIds(unreadSummary), [unreadSummary]);
  const { data: pinnedItems = EMPTY_PINS } = useQuery({
    ...pinListOptions(wsId ?? "", userId ?? ""),
    enabled: !!wsId && !!userId,
  });
  const deletePin = useDeletePin();
  const reorderPins = useReorderPins();
  const sensors = useSensors(useSensor(PointerSensor, { activationConstraint: { distance: 5 } }));
  const sidebarScrollRef = useRef<HTMLDivElement>(null);
  const sidebarFadeStyle = useScrollFade(sidebarScrollRef, 24);
  const getPinHref = useCallback(
    (pin: PinnedItem) => (pin.item_type === "task" ? p.taskDetail(pin.item_id) : p.projectDetail(pin.item_id)),
    [p],
  );

  // Local presentational copy of pinnedItems for drop-animation stability.
  // Follows TQ at rest; frozen during a drag gesture so a mid-drag cache
  // write (our own optimistic update, or a WS refetch) cannot reorder the
  // DOM under dnd-kit while its drop animation is still interpolating.
  const [localPinned, setLocalPinned] = useState<PinnedItem[]>(pinnedItems);
  const [localPinnedWsId, setLocalPinnedWsId] = useState<string | null>(wsId ?? null);
  const isDraggingRef = useRef(false);
  useEffect(() => {
    if (!isDraggingRef.current) {
      setLocalPinned(pinnedItems);
    }
  }, [pinnedItems]);
  useEffect(() => {
    setLocalPinnedWsId(wsId ?? null);
  }, [wsId]);
  const visiblePinned = localPinnedWsId === (wsId ?? null) ? localPinned : EMPTY_PINS;
  const isActivePinnedRoute = visiblePinned.some((pin) => pathname === getPinHref(pin));

  const handleDragStart = useCallback(() => {
    isDraggingRef.current = true;
  }, []);
  const handleDragEnd = useCallback(
    (event: DragEndEvent) => {
      isDraggingRef.current = false;
      const { active, over } = event;
      if (!over || active.id === over.id) return;
      const oldIndex = localPinned.findIndex((p) => p.id === active.id);
      const newIndex = localPinned.findIndex((p) => p.id === over.id);
      if (oldIndex === -1 || newIndex === -1) return;
      const reordered = arrayMove(localPinned, oldIndex, newIndex);
      setLocalPinned(reordered);
      reorderPins.mutate(reordered);
    },
    [localPinned, reorderPins],
  );

  const queryClient = useQueryClient();
  const acceptInvitationMut = useMutation({
    mutationFn: (id: string) => api.acceptInvitation(id),
    // After accepting an invitation, navigate INTO the newly-joined workspace.
    // Otherwise the user stays on their current workspace and just sees the
    // new one appear in the dropdown — silent and confusing (this is SILI-820).
    onSuccess: async (_, invitationId) => {
      const invitation = myInvitations.find((i) => i.id === invitationId);
      queryClient.invalidateQueries({ queryKey: workspaceKeys.myInvitations() });
      // staleTime: 0 forces a real network fetch — we need the joined workspace
      // in the list before we can resolve its slug for navigation.
      const list = await queryClient.fetchQuery({
        ...workspaceListOptions(),
        staleTime: 0,
      });
      const joined = invitation
        ? list.find((w) => w.id === invitation.workspace_id)
        : null;
      if (joined) {
        push(paths.workspace(joined.slug).chat());
      }
    },
  });
  const declineInvitationMut = useMutation({
    mutationFn: (id: string) => api.declineInvitation(id),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: workspaceKeys.myInvitations() });
    },
  });

  const createTaskShortcut = useShortcut("createTask");
  const { data: projects = [] } = useQuery({
    ...projectListOptions(wsId ?? ""),
    enabled: Boolean(wsId),
  });
  const { data: workflowRuns = [] } = useQuery({
    ...workflowInstancesOptions(wsId ?? ""),
    enabled: Boolean(wsId) && sopEnabled,
  });
  const visibleRunsByProject = React.useMemo(() => {
    const grouped = new Map<string, typeof workflowRuns>();
    for (const run of workflowRuns) {
      if (
        !run.project_id ||
        run.archived_at ||
        run.status === "cancelled"
      ) {
        continue;
      }
      const projectRuns = grouped.get(run.project_id) ?? [];
      projectRuns.push(run);
      grouped.set(run.project_id, projectRuns);
    }
    return grouped;
  }, [workflowRuns]);
  const projectsHref = p.projects();
  const [expandedSpaceId, setExpandedSpaceId] = useState<string>();
  const [managementOpen, setManagementOpen] = useState<Record<string, boolean>>({});
  const managementActive = visibleManagementNav.some((item) =>
    isNavActive(pathname, p[item.key]()),
  );
  useEffect(() => {
    if (wsId && managementActive) {
      setManagementOpen((previous) =>
        previous[wsId] ? previous : { ...previous, [wsId]: true },
      );
    }
  }, [wsId, managementActive, pathname]);
  const [expandedProjectIds, setExpandedProjectIds] = useState<Set<string>>(
    () => new Set(),
  );
  useEffect(() => {
    if (wsId) setExpandedSpaceId(wsId);
  }, [wsId]);
  useEffect(() => {
    const projectPrefix = `${projectsHref}/`;
    if (!pathname.startsWith(projectPrefix)) return;
    const projectId = decodeURIComponent(
      pathname.slice(projectPrefix.length).split("/")[0] ?? "",
    );
    if (!projectId) return;
    setExpandedProjectIds((current) => {
      if (current.has(projectId)) return current;
      const next = new Set(current);
      next.add(projectId);
      return next;
    });
  }, [pathname, projectsHref]);

  return (
    <Sidebar variant="inset" collapsible="icon">
      {topSlot}
      <SidebarHeader
        className={cn("gap-2 border-b border-sidebar-border/70 py-3 group-data-[collapsible=icon]:items-center", headerClassName)}
        style={headerStyle}
      >
        <div className={cn("items-center gap-2 px-2 group-data-[collapsible=icon]:justify-center group-data-[collapsible=icon]:px-0", hideBrand ? "hidden" : "flex")}>
          <SiliecoIcon noSpin variant="small" className="size-7 shrink-0" />
          <p className="min-w-0 flex-1 truncate text-caption font-medium text-muted-foreground group-data-[collapsible=icon]:hidden">
            {t(($) => $.sidebar.brand_tagline)}
          </p>
        </div>
        {!workspaceCreationDisabled && (
          <SidebarMenuButton
            tooltip={t(($) => $.sidebar.create_workspace)}
            aria-label={t(($) => $.sidebar.create_workspace)}
            className="border border-sidebar-border bg-sidebar-accent/40 font-medium"
            onClick={() => useModalStore.getState().open("create-workspace")}
          >
            <Plus className="size-4 text-brand" />
            <span className="group-data-[collapsible=icon]:hidden">{t(($) => $.sidebar.create_workspace)}</span>
          </SidebarMenuButton>
        )}
        {searchSlot}
      </SidebarHeader>

      <SidebarContent ref={sidebarScrollRef} style={sidebarFadeStyle} className="group-data-[collapsible=icon]:overflow-y-auto group-data-[collapsible=icon]:overflow-x-hidden">
        {iconOnly ? (
          <div className="flex flex-col items-center gap-2 py-2">
            {myInvitations.length > 0 && (
              <SidebarMenuButton tooltip={t(($) => $.sidebar.pending_invitations_label)} aria-label={t(($) => $.sidebar.pending_invitations_label)} onClick={() => setOpen(true)} className="relative">
                <Users /><span className="absolute right-1 top-1 size-1.5 rounded-full bg-brand" />
              </SidebarMenuButton>
            )}
            {workspaces.map((space) => {
              const isActiveSpace = space.id === wsId;
              const spaceOpen = isActiveSpace && expandedSpaceId === space.id;
              return (
                <div key={space.id} className={cn("flex w-full flex-col items-center gap-1 rounded-xl py-1", spaceOpen && "bg-sidebar-accent/20")}>
                  <SidebarMenuButton
                    tooltip={space.name}
                    aria-label={space.name}
                    aria-expanded={spaceOpen}
                    aria-controls={spaceOpen ? `compact-space-${space.id}` : undefined}
                    isActive={isActiveSpace}
                    render={isActiveSpace ? undefined : <AppLink openInAppTab href={paths.workspace(space.slug).chat()} />}
                    onClick={() => setExpandedSpaceId((current) => isActiveSpace && current === space.id ? undefined : space.id)}
                    className="relative justify-center !p-1"
                  >
                    <WorkspaceAvatar name={space.name} avatarUrl={space.avatar_url} size="sm" />
                    {unreadWsIds.has(space.id) && <span className="absolute right-0.5 top-0.5 size-1.5 rounded-full bg-brand" />}
                    {spaceOpen && <span aria-hidden="true" className="absolute bottom-0.5 left-1/2 h-0.5 w-2 -translate-x-1/2 rounded-full bg-brand" />}
                  </SidebarMenuButton>
                  {spaceOpen && (
                    <div id={`compact-space-${space.id}`} className="flex w-full flex-col items-center gap-2 pb-2 pt-1">
            <SidebarMenu className="items-center gap-1">
              {!taskInToolbar && <SidebarMenuItem>
                <SidebarMenuButton tooltip={t(($) => $.sidebar.new_task)} aria-label={t(($) => $.sidebar.new_task)} onClick={() => openCreateTaskWithPreference()} className="text-brand">
                  <SquarePen /><DraftDot />
                </SidebarMenuButton>
              </SidebarMenuItem>}
              {personalNav.map((item) => {
                const href = p[item.key]();
                const Icon = routeIconForPath(href);
                const label = t(($) => $.nav[item.labelKey]);
                const count = item.key === "inbox" ? unreadCount : item.key === "chat" ? chatUnreadCount : 0;
                return (
                  <SidebarMenuItem key={item.key}>
                    <SidebarMenuButton tooltip={count ? `${label} (${count})` : label} aria-label={count ? `${label} (${count})` : label} isActive={isNavActive(pathname, href)} render={<AppLink openInAppTab href={href} />} className="relative text-muted-foreground data-active:text-sidebar-accent-foreground">
                      <Icon />
                      {count > 0 && <span className="absolute right-1 top-1 size-1.5 rounded-full bg-brand" />}
                    </SidebarMenuButton>
                  </SidebarMenuItem>
                );
              })}
            </SidebarMenu>
              {visiblePinned.length > 0 && (
                <DndContext sensors={sensors} collisionDetection={closestCenter} onDragStart={handleDragStart} onDragEnd={handleDragEnd}>
                  <SortableContext items={visiblePinned.map((pin) => pin.id)} strategy={verticalListSortingStrategy}>
                    <SidebarMenu className="items-center gap-1" aria-label={t(($) => $.sidebar.pinned_label)}>
                      {visiblePinned.map((pin) => <PinRow key={pin.id} pin={pin} href={getPinHref(pin)} pathname={pathname} onUnpin={() => deletePin.mutate({ itemType: pin.item_type, itemId: pin.item_id })} wsId={wsId ?? ""} />)}
                    </SidebarMenu>
                  </SortableContext>
                </DndContext>
              )}
            <SidebarMenu className="items-center gap-1">
              {workspaceNav.map((item) => {
                const href = p[item.key]();
                const Icon = routeIconForPath(href);
                const label = t(($) => $.nav[item.labelKey]);
                const count = item.key === "inbox" ? unreadCount : item.key === "chat" ? chatUnreadCount : 0;
                return (
                  <SidebarMenuItem key={item.key}>
                    <SidebarMenuButton tooltip={count ? `${label} (${count})` : label} aria-label={count ? `${label} (${count})` : label} isActive={isNavActive(pathname, href) || (item.key === "resources" && pathname === p.legacyGit())} render={<AppLink openInAppTab href={href} />} className="relative text-muted-foreground data-active:text-sidebar-accent-foreground">
                      <Icon />
                      {count > 0 && <span className="absolute right-1 top-1 size-1.5 rounded-full bg-brand" />}
                    </SidebarMenuButton>
                  </SidebarMenuItem>
                );
              })}
              <SidebarMenuItem>
                <DropdownMenu>
                  <SidebarMenuButton
                    tooltip={t(($) => $.sidebar.management_group)}
                    aria-label={t(($) => $.sidebar.management_group)}
                    isActive={managementActive}
                    render={<DropdownMenuTrigger />}
                    className="text-muted-foreground data-active:text-sidebar-accent-foreground"
                  >
                    <Layers3 />
                  </SidebarMenuButton>
                  <DropdownMenuContent side="right" align="start" sideOffset={10} className="w-48">
                    {visibleManagementNav.map((item) => {
                      const href = p[item.key]();
                      const Icon = routeIconForPath(href);
                      return (
                        <DropdownMenuItem
                          key={item.key}
                          render={<AppLink openInAppTab href={href} />}
                          className={cn(isNavActive(pathname, href) && "bg-accent font-medium")}
                        >
                          <Icon className="size-4" />
                          {t(($) => $.nav[item.labelKey])}
                        </DropdownMenuItem>
                      );
                    })}
                  </DropdownMenuContent>
                </DropdownMenu>
              </SidebarMenuItem>
            </SidebarMenu>
                      <CollapsedProjects projects={projects} runsByProject={visibleRunsByProject} />
                    </div>
                  )}
                </div>
              );
            })}
          </div>
        ) : (<>

        {myInvitations.length > 0 && (
          <SidebarGroup className="pb-1">
            <SidebarGroupLabel>
              {t(($) => $.sidebar.pending_invitations_label)}
            </SidebarGroupLabel>
            <div className="space-y-1 px-2">
              {myInvitations.map((invitation) => (
                <div
                  key={invitation.id}
                  className="rounded-lg border border-sidebar-border/70 bg-sidebar-accent/30 p-2"
                >
                  <div className="flex items-center gap-2">
                    <WorkspaceAvatar
                      name={invitation.workspace_name ?? "W"}
                      size="sm"
                    />
                    <span className="min-w-0 flex-1 truncate text-caption font-medium">
                      {invitation.workspace_name ??
                        t(($) => $.sidebar.invitation_workspace_fallback)}
                    </span>
                  </div>
                  <div className="mt-2 flex gap-1.5">
                    <button
                      type="button"
                      className="rounded-md bg-primary px-2 py-1 text-micro text-primary-foreground"
                      disabled={acceptInvitationMut.isPending}
                      onClick={() => acceptInvitationMut.mutate(invitation.id)}
                    >
                      {t(($) => $.sidebar.invitation_join)}
                    </button>
                    <button
                      type="button"
                      className="rounded-md px-2 py-1 text-micro text-muted-foreground hover:bg-sidebar-accent"
                      disabled={declineInvitationMut.isPending}
                      onClick={() => declineInvitationMut.mutate(invitation.id)}
                    >
                      {t(($) => $.sidebar.invitation_decline)}
                    </button>
                  </div>
                </div>
              ))}
            </div>
          </SidebarGroup>
        )}

        <div className="space-y-1 px-2 py-2">
          {workspaces.map((space) => {
            const isActiveSpace = space.id === wsId;
            const isOpen =
              isActiveSpace && expandedSpaceId === space.id;
            const spaceHref = paths.workspace(space.slug).chat();
            return (
              <Collapsible key={space.id} open={isOpen}>
                <div
                  className={cn(
                    "group/space rounded-xl border transition-colors",
                    isOpen
                      ? "border-sidebar-border bg-sidebar-accent/20"
                      : "border-transparent hover:bg-sidebar-accent/50",
                  )}
                >
                  {isActiveSpace ? (
                    <button
                      type="button"
                      className="flex h-10 w-full items-center gap-2 rounded-xl px-2 text-left"
                      onClick={() =>
                        setExpandedSpaceId((current) =>
                          current === space.id ? undefined : space.id,
                        )
                      }
                    >
                      <WorkspaceAvatar
                        name={space.name}
                        avatarUrl={space.avatar_url}
                        size="sm"
                      />
                      <span className="min-w-0 flex-1 truncate text-body font-semibold">
                        {space.name}
                      </span>
                      {unreadWsIds.has(space.id) && unreadCount === 0 && (
                        <span className="size-1.5 rounded-full bg-brand" />
                      )}
                      <ChevronRight
                        className={cn(
                          "size-3.5 text-muted-foreground transition-transform",
                          isOpen && "rotate-90",
                        )}
                      />
                    </button>
                  ) : (
                    <AppLink
                      openInAppTab
                      href={spaceHref}
                      className="flex h-10 items-center gap-2 rounded-xl px-2"
                    >
                      <WorkspaceAvatar
                        name={space.name}
                        avatarUrl={space.avatar_url}
                        size="sm"
                      />
                      <span className="min-w-0 flex-1 truncate text-body font-medium">
                        {space.name}
                      </span>
                      {unreadWsIds.has(space.id) && (
                        <span className="size-1.5 rounded-full bg-brand" />
                      )}
                      <ChevronRight className="size-3.5 text-muted-foreground" />
                    </AppLink>
                  )}

                  <CollapsibleContent>
                    <div className="space-y-3 border-t border-sidebar-border/60 px-1.5 pb-2 pt-2">
                      <SidebarMenu>
                        {!taskInToolbar && <SidebarMenuItem>
                          <SidebarMenuButton
                            className="bg-brand/8 font-medium text-brand hover:bg-brand/12"
                            onClick={() => openCreateTaskWithPreference()}
                          >
                            <span className="relative">
                              <SquarePen />
                              <DraftDot />
                            </span>
                            <span>{t(($) => $.sidebar.new_task)}</span>
                            {createTaskShortcut ? (
                              <ShortcutKeycaps
                                shortcut={createTaskShortcut}
                                decorative
                                className="pointer-events-none ml-auto"
                              />
                            ) : null}
                          </SidebarMenuButton>
                        </SidebarMenuItem>}
                        {personalNav.map((item) => {
                          const href = p[item.key]();
                          const Icon = routeIconForPath(href);
                          return (
                            <SidebarMenuItem key={item.key}>
                              <SidebarMenuButton
                                isActive={isNavActive(pathname, href)}
                                render={<AppLink openInAppTab href={href} />}
                                className="text-muted-foreground hover:not-data-active:bg-sidebar-accent/70 data-active:bg-sidebar-accent data-active:text-sidebar-accent-foreground"
                              >
                                <Icon />
                                <span>{t(($) => $.nav[item.labelKey])}</span>
                                {item.key === "inbox" && unreadCount > 0 && (
                                  <CappedNumberFlow
                                    value={unreadCount}
                                    animated={false}
                                    className="ml-auto text-caption"
                                  />
                                )}
                                {item.key === "chat" && chatUnreadCount > 0 && (
                                  <CappedNumberFlow
                                    value={chatUnreadCount}
                                    animated={false}
                                    className="ml-auto text-caption"
                                  />
                                )}
                              </SidebarMenuButton>
                            </SidebarMenuItem>
                          );
                        })}
                      </SidebarMenu>

                      {visiblePinned.length > 0 && (
                        <Collapsible defaultOpen>
                          <SidebarGroup className="group/pinned p-0">
                            <SidebarGroupLabel
                              render={<CollapsibleTrigger />}
                              className="group/trigger cursor-pointer hover:bg-sidebar-accent/70"
                            >
                              <span>{t(($) => $.sidebar.pinned_label)}</span>
                              <ChevronRight className="ml-1 !size-3 transition-transform group-data-[panel-open]/trigger:rotate-90" />
                              <span className="ml-auto text-micro">
                                {visiblePinned.length}
                              </span>
                            </SidebarGroupLabel>
                            <CollapsibleContent>
                              <DndContext
                                sensors={sensors}
                                collisionDetection={closestCenter}
                                onDragStart={handleDragStart}
                                onDragEnd={handleDragEnd}
                              >
                                <SortableContext
                                  items={visiblePinned.map((pin) => pin.id)}
                                  strategy={verticalListSortingStrategy}
                                >
                                  <SidebarMenu className="gap-0.5">
                                    {visiblePinned.map((pin) => (
                                      <PinRow
                                        key={pin.id}
                                        pin={pin}
                                        href={getPinHref(pin)}
                                        pathname={pathname}
                                        onUnpin={() =>
                                          deletePin.mutate({
                                            itemType: pin.item_type,
                                            itemId: pin.item_id,
                                          })
                                        }
                                        wsId={wsId ?? ""}
                                      />
                                    ))}
                                  </SidebarMenu>
                                </SortableContext>
                              </DndContext>
                            </CollapsibleContent>
                          </SidebarGroup>
                        </Collapsible>
                      )}

                      <SidebarMenu>
                        {workspaceNav.map((item) => {
                          const href = p[item.key]();
                          const Icon = routeIconForPath(href);
                          return (
                            <SidebarMenuItem key={item.key}>
                              <SidebarMenuButton
                                isActive={
                                  !isActivePinnedRoute &&
                                  (isNavActive(pathname, href) || (item.key === "resources" && pathname === p.legacyGit()))
                                }
                                render={<AppLink openInAppTab href={href} />}
                                className="text-muted-foreground hover:not-data-active:bg-sidebar-accent/70 data-active:bg-sidebar-accent data-active:text-sidebar-accent-foreground"
                              >
                                <Icon />
                                <span>{t(($) => $.nav[item.labelKey])}</span>
                              </SidebarMenuButton>
                            </SidebarMenuItem>
                          );
                        })}
                        <SidebarMenuItem>
                          <Collapsible
                            open={!!managementOpen[space.id]}
                            onOpenChange={(open) =>
                              setManagementOpen((previous) => ({ ...previous, [space.id]: open }))
                            }
                          >
                            <SidebarMenuButton
                              render={<CollapsibleTrigger />}
                              isActive={managementActive && !managementOpen[space.id]}
                              className="text-muted-foreground hover:bg-sidebar-accent/70 data-active:bg-sidebar-accent data-active:text-sidebar-accent-foreground"
                            >
                              <Layers3 />
                              <span className="flex-1">{t(($) => $.sidebar.management_group)}</span>
                              <ChevronRight className={cn("!size-3 transition-transform", managementOpen[space.id] && "rotate-90")} />
                            </SidebarMenuButton>
                            <CollapsibleContent>
                              <SidebarMenu className="ml-3 w-[calc(100%-0.75rem)] border-l border-sidebar-border pl-2">
                                {visibleManagementNav.map((item) => {
                                  const href = p[item.key]();
                                  const Icon = routeIconForPath(href);
                                  return (
                                    <SidebarMenuItem key={item.key}>
                                      <SidebarMenuButton
                                        isActive={!isActivePinnedRoute && isNavActive(pathname, href)}
                                        render={<AppLink openInAppTab href={href} />}
                                        className="text-muted-foreground hover:bg-sidebar-accent/70 data-active:bg-sidebar-accent data-active:text-sidebar-accent-foreground"
                                      >
                                        <Icon />
                                        <span>{t(($) => $.nav[item.labelKey])}</span>
                                      </SidebarMenuButton>
                                    </SidebarMenuItem>
                                  );
                                })}
                              </SidebarMenu>
                            </CollapsibleContent>
                          </Collapsible>
                        </SidebarMenuItem>
                      </SidebarMenu>

                      <div>
                        <div className="mb-1 flex h-8 items-center gap-2 px-2 text-body font-medium text-muted-foreground">
                          <FolderKanban className="size-3.5" />
                          <span>{t(($) => $.nav.projects)}</span>
                          <button
                            type="button"
                            aria-label={t(($) => $.sidebar.create_project)}
                            title={t(($) => $.sidebar.create_project)}
                            className="ml-auto rounded-md p-1 hover:bg-sidebar-accent hover:text-foreground"
                            onClick={() =>
                              useModalStore.getState().open("create-project")
                            }
                          >
                            <Plus className="size-3.5" />
                          </button>
                          <AppLink
                            openInAppTab
                            href={projectsHref}
                            aria-label={t(($) => $.sidebar.view_projects)}
                            title={t(($) => $.sidebar.view_projects)}
                            className="rounded-md p-1 hover:bg-sidebar-accent hover:text-foreground"
                          >
                            <ChevronRight className="size-3.5" />
                          </AppLink>
                        </div>
                        <SidebarMenu className="gap-0.5">
                          {projects.map((project) => {
                            const projectHref = p.projectDetail(project.id);
                            const resourceHref = p.projectResources(project.id);
                            const sopHref = p.projectSop(project.id);
                            const standaloneFiles = pathname === p.projectFilesStandalone(project.id);
                            const projectRuns = visibleRunsByProject.get(project.id) ?? [];
                            const projectOpen = expandedProjectIds.has(project.id);
                            const projectActive = isNavActive(pathname, projectHref);
                            const activeSection = searchParams.get("section");
                            const activeRunId = searchParams.get("run");
                            const resourceActive = standaloneFiles || activeSection === "resources" || activeSection === "git" || activeSection === "files";
                            return (
                              <Collapsible key={project.id} open={projectOpen}>
                                <SidebarMenuItem>
                                  <SidebarMenuButton
                                    isActive={projectActive}
                                    className="h-8 text-body font-medium text-muted-foreground data-active:bg-sidebar-accent data-active:text-sidebar-accent-foreground"
                                    onClick={() => {
                                      setExpandedProjectIds((current) => {
                                        const next = new Set(current);
                                        if (next.has(project.id)) next.delete(project.id);
                                        else next.add(project.id);
                                        return next;
                                      });
                                      if (!projectActive) {
                                        if (openInNewTab) openInNewTab(projectHref, undefined, { activate: true });
                                        else push(projectHref);
                                      }
                                    }}
                                  >
                                    <ProjectIcon project={project} size="sm" />
                                    <span className="min-w-0 flex-1 truncate">
                                      {project.title}
                                    </span>
                                    <ChevronRight
                                      className={cn(
                                        "size-3 transition-transform",
                                        projectOpen && "rotate-90",
                                      )}
                                    />
                                  </SidebarMenuButton>
                                </SidebarMenuItem>
                                <CollapsibleContent>
                                  <div className="ml-5 border-l border-sidebar-border pl-2">
                                    <AppLink
                                      openInAppTab
                                      href={projectHref}
                                      className={cn(
                                        "flex h-7 items-center gap-2 rounded-md px-2 text-caption text-muted-foreground hover:bg-sidebar-accent hover:text-foreground",
                                        projectActive &&
                                          !activeRunId &&
                                          activeSection !== "sop" &&
                                          !resourceActive &&
                                          "bg-sidebar-accent text-foreground",
                                      )}
                                    >
                                      <SquarePen className="size-3.5" />
                                      {t(($) => $.tab.task)}
                                    </AppLink>
                                    <AppLink
                                      openInAppTab
                                      href={resourceHref}
                                      className={cn(
                                        "flex h-7 items-center gap-2 rounded-md px-2 text-caption text-muted-foreground hover:bg-sidebar-accent hover:text-foreground",
                                        projectActive &&
                                          !activeRunId &&
                                          resourceActive &&
                                          "bg-sidebar-accent text-foreground",
                                      )}
                                    >
                                      <FileText className="size-3.5" />
                                      {t(($) => $.nav.resources)}
                                    </AppLink>
                                    {sopEnabled && <AppLink
                                      openInAppTab
                                      href={sopHref}
                                      className={cn(
                                        "flex h-7 items-center gap-2 rounded-md px-2 text-caption text-muted-foreground hover:bg-sidebar-accent hover:text-foreground",
                                        projectActive &&
                                          !activeRunId &&
                                          activeSection === "sop" &&
                                          "bg-sidebar-accent text-foreground",
                                      )}
                                    >
                                      <GitBranch className="size-3.5" />
                                      SOP
                                    </AppLink>}
                                    {sopEnabled && projectRuns.length > 0 && (
                                      <div className="mt-1 border-t border-sidebar-border/70 pt-1">
                                        {projectRuns.map((run) => (
                                          <AppLink
                                            openInAppTab
                                            key={run.id}
                                            href={p.projectWorkflowRun(project.id, run.id)}
                                            title={run.title}
                                            className={cn(
                                              "flex h-7 items-center gap-2 rounded-md px-2 text-caption text-muted-foreground hover:bg-sidebar-accent hover:text-foreground",
                                              projectActive &&
                                                activeRunId === run.id &&
                                                "bg-sidebar-accent text-foreground",
                                            )}
                                          >
                                            <Route className="size-3.5 shrink-0 text-brand" />
                                            <span className="min-w-0 flex-1 truncate">
                                              {run.title}
                                            </span>
                                          </AppLink>
                                        ))}
                                      </div>
                                    )}
                                  </div>
                                </CollapsibleContent>
                              </Collapsible>
                            );
                          })}
                        </SidebarMenu>
                      </div>
                    </div>
                  </CollapsibleContent>
                </div>
              </Collapsible>
            );
          })}
        </div>
        </>)}
      </SidebarContent>

      <SidebarFooter className="border-t border-sidebar-border/70 p-2">
        <div className="flex items-center gap-1 group-data-[collapsible=icon]:flex-col">
          <DropdownMenu>
            <DropdownMenuTrigger
              render={
                <button
                  type="button"
                  aria-label={user?.name || user?.email || t(($) => $.nav.settings)}
                  title={iconOnly ? (user?.name || user?.email || t(($) => $.nav.settings)) : undefined}
                  className="flex min-w-0 flex-1 items-center gap-2 rounded-lg px-2 py-1.5 text-left hover:bg-sidebar-accent group-data-[collapsible=icon]:size-8 group-data-[collapsible=icon]:flex-none group-data-[collapsible=icon]:justify-center group-data-[collapsible=icon]:p-0"
                />
              }
            >
              <ActorAvatar
                name={user?.name ?? ""}
                initials={(user?.name ?? "U").charAt(0).toUpperCase()}
                avatarUrl={resolvePublicFileUrl(user?.avatar_url)}
                size="sm"
              />
              <span className="min-w-0 flex-1 group-data-[collapsible=icon]:hidden">
                <span className="block truncate text-caption font-medium">
                  {user?.name}
                </span>
                <span className="block truncate text-micro text-muted-foreground">
                  {user?.email}
                </span>
              </span>
              <ChevronDown className="size-3.5 text-muted-foreground group-data-[collapsible=icon]:hidden" />
            </DropdownMenuTrigger>
            <DropdownMenuContent align="start" side="top" className="w-56">
              <DropdownMenuItem
                render={<AppLink openInAppTab href={p.settings()} />}
                aria-label={settingsAccessibleLabel}
              >
                <Settings className="size-3.5" />
                {settingsLabel}
                {settingsNotice && <span aria-hidden="true" className="ml-auto size-1.5 shrink-0 rounded-full bg-destructive" />}
              </DropdownMenuItem>
              <DropdownMenuSeparator />
              <DropdownMenuItem variant="destructive" onClick={logout}>
                <LogOut className="size-3.5" />
                {t(($) => $.sidebar.log_out)}
              </DropdownMenuItem>
            </DropdownMenuContent>
          </DropdownMenu>
          <Tooltip>
            <TooltipTrigger
              render={
                <AppLink
                  openInAppTab
                  href={p.settings()}
                  aria-label={settingsAccessibleLabel}
                  className="relative flex size-8 shrink-0 items-center justify-center rounded-lg text-muted-foreground hover:bg-sidebar-accent hover:text-foreground"
                />
              }
            >
              <Settings className="size-4" />
              {settingsNotice && <span aria-hidden="true" className="pointer-events-none absolute right-1 top-1 size-1.5 rounded-full bg-destructive" />}
            </TooltipTrigger>
            <TooltipContent side="top">
              {settingsAccessibleLabel}
            </TooltipContent>
          </Tooltip>
        </div>
      </SidebarFooter>
      <SidebarRail />
    </Sidebar>
  );
}
