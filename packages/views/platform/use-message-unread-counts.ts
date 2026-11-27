import { useEffect, useMemo } from "react";
import { useQuery } from "@tanstack/react-query";
import { chatSessionsOptions } from "@silieco/core/chat/queries";
import { countUnreadChatMessages } from "@silieco/core/chat/unread";
import { useChatStore } from "@silieco/core/chat";
import { deduplicateInboxItems, inboxListOptions } from "@silieco/core/inbox/queries";
import { paths, useCurrentWorkspace } from "@silieco/core/paths";
import { reconcileWebNotifications } from "@silieco/core/platform";
import { useNavigation } from "../navigation";
import { useAppForeground } from "../common/use-app-foreground";

/** One definition for sidebar, top tabs, and the desktop Dock. */
export function useMessageUnreadCounts() {
  const workspace = useCurrentWorkspace();
  const wsId = workspace?.id ?? "";
  const { pathname } = useNavigation();
  const { data: inboxItems } = useQuery({ ...inboxListOptions(wsId), enabled: !!wsId });
  const { data: chatSessions } = useQuery({ ...chatSessionsOptions(wsId, workspace?.slug), enabled: !!wsId });
  const activeSessionId = useChatStore((s) => s.activeSessionId);
  const floatingOpen = useChatStore((s) => s.isOpen);
  const foreground = useAppForeground();
  const chatPath = workspace ? paths.workspace(workspace.slug).chat() : null;
  const viewed = foreground && (floatingOpen || (chatPath && (pathname === chatPath || pathname.startsWith(chatPath + "/"))))
    ? activeSessionId : null;
  const inboxUnread = useMemo(() => inboxItems ? deduplicateInboxItems(inboxItems).filter((i) => !i.read) : undefined, [inboxItems]);
  const unreadChatIds = useMemo(() => chatSessions?.filter((s) => s.status !== "archived" && (s.unread_count ?? 0) > 0).map((s) => s.id), [chatSessions]);
  const unreadInboxKeys = useMemo(() => inboxUnread?.map((i) => i.task_id ?? i.id), [inboxUnread]);
  useEffect(() => {
    if (workspace?.slug) reconcileWebNotifications({ slug: workspace.slug, unreadChatIds, unreadInboxKeys });
  }, [workspace?.slug, unreadChatIds, unreadInboxKeys]);
  return {
    workspaceSlug: workspace?.slug,
    inboxUnreadCount: inboxUnread?.length ?? 0,
    chatUnreadCount: countUnreadChatMessages(chatSessions, viewed),
    unreadChatIds,
    unreadInboxKeys,
  };
}
