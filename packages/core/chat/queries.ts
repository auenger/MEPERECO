import { infiniteQueryOptions, queryOptions, type InfiniteData, type QueryClient } from "@tanstack/react-query";
import { api } from "../api";
import type { ChatSession } from "../types/chat";
import type { AgentRunMessagePayload } from "../types/events";
import { getCurrentSlug, getCurrentWsId } from "../platform/workspace-storage";
import { cacheChatMessagePage, mergeLatestChatMessagePage, mergeAgentRunMessagesBySeq, type CachedChatMessagesPage } from "./cache-merge";

export { mergeAgentRunMessagesBySeq } from "./cache-merge";

// NOTE on workspace scoping:
// `wsId` is used only as part of queryKey for cache isolation per workspace.
// The actual workspace context comes from ApiClient's X-Workspace-Slug header,
// which is set by the URL-driven [workspaceSlug] layout. Callers must ensure
// the header is in sync with the wsId they pass here — otherwise cache writes
// will be misattributed during a workspace switch race window.

export const chatKeys = {
  all: (wsId: string) => ["chat", wsId] as const,
  /** Full sessions list (active + archived); the dropdown splits locally. */
  sessions: (wsId: string) => [...chatKeys.all(wsId), "sessions"] as const,
  session: (wsId: string, id: string) => [...chatKeys.all(wsId), "session", id] as const,
  messagesAll: () => ["chat", "messages"] as const,
  messages: (sessionId: string) => [...chatKeys.messagesAll(), sessionId] as const,
  messagesPageAll: () => ["chat", "messages-page"] as const,
  messagesPage: (sessionId: string) => [...chatKeys.messagesPageAll(), sessionId] as const,
  pendingRunAll: () => ["chat", "pending-run"] as const,
  pendingRun: (sessionId: string) => [...chatKeys.pendingRunAll(), sessionId] as const,
  draftRestoresAll: () => ["chat", "draft-restores"] as const,
  /** Durable deferred-cancellation draft restores for a session (#5219). */
  draftRestores: (sessionId: string) => [...chatKeys.draftRestoresAll(), sessionId] as const,
  /** Aggregate of in-flight chat runs for the current user — FAB reads this. */
  pendingRuns: (wsId: string) => [...chatKeys.all(wsId), "pending-runs"] as const,
  /** Per-user pinned agents for the quick-agent bar. */
  pinnedAgents: (wsId: string) => [...chatKeys.all(wsId), "pinned-agents"] as const,
  /**
   * Boolean "does the user have any in-flight chat run" — the FAB's cheap
   * running indicator. Separate cache from the detailed `pendingRuns` list so
   * the FAB (closed-window) and ChatWindow (open) can subscribe independently.
   */
  pendingRunsHasAny: (wsId: string) =>
    [...chatKeys.all(wsId), "pending-runs", "has-any"] as const,
  /** Per-run execution messages — shared with task agent cards. */
  agentRunMessagesAll: () => ["run-messages"] as const,
  agentRunMessages: (runId: string) => [...chatKeys.agentRunMessagesAll(), runId] as const,
};

const UUID_PATTERN = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i;

export function isAgentRunMessageRunId(runId: string | null | undefined): runId is string {
  return typeof runId === "string" && UUID_PATTERN.test(runId);
}

export function chatSessionsOptions(wsId: string) {
  return queryOptions({
    queryKey: chatKeys.sessions(wsId),
    queryFn: () => api.listChatSessions({ status: "all" }),
    staleTime: Infinity,
  });
}

/** Last-activity timestamp used to rank the IM list (newest first). */
function sessionActivityTime(s: ChatSession): number {
  return new Date(s.last_message?.created_at ?? s.updated_at).getTime();
}

/**
 * Orders the chat list the same way the server does: pinned chats first, then
 * everyone else by most-recent activity. Used both to render the list and to
 * re-sort the cache after an optimistic pin/unpin or a WS patch, so a mutated
 * flat cache never renders out of order. Returns a new array; stable for equal
 * keys (Array.prototype.sort is stable), so pinned rows keep their server
 * order when pin timestamps aren't carried in the list payload.
 */
export function sortChatSessions(sessions: ChatSession[]): ChatSession[] {
  return [...sessions].sort((a, b) => {
    const ap = a.pinned ? 1 : 0;
    const bp = b.pinned ? 1 : 0;
    if (ap !== bp) return bp - ap;
    return sessionActivityTime(b) - sessionActivityTime(a);
  });
}

/**
 * Number of sessions that should light up the quick-chat FAB unread badge.
 * `chatSessionsOptions` fetches `status=all` (active + archived) so the thread
 * list can render an Archived view, but archived sessions must NOT contribute
 * to the badge: they are read-only and hidden from the default history list, so
 * a badge sourced from one is uncleared-able — the user can't open it to mark it
 * read. Archiving now also drops the external-channel binding server-side, so no
 * new unread should land on an archived session; this filter is the front-end
 * half of that guarantee (SILI-4372).
 */
export function countUnreadChatSessions(sessions: ChatSession[]): number {
  return sessions.filter((s) => s.has_unread && s.status !== "archived").length;
}

export function chatPinnedAgentsOptions(wsId: string) {
  return queryOptions({
    queryKey: chatKeys.pinnedAgents(wsId),
    queryFn: () => api.listChatPinnedAgents(),
    staleTime: Infinity,
  });
}

export function chatSessionOptions(wsId: string, id: string) {
  return queryOptions({
    queryKey: chatKeys.session(wsId, id),
    queryFn: () => api.getChatSession(id),
    enabled: !!id,
    staleTime: Infinity,
  });
}

export function chatMessagesOptions(sessionId: string) {
  return queryOptions({
    queryKey: chatKeys.messages(sessionId),
    queryFn: () => api.listChatMessages(sessionId),
    enabled: !!sessionId,
    staleTime: Infinity,
  });
}

export function chatMessagesPageOptions(sessionId: string, limit = 50) {
  return infiniteQueryOptions({
    queryKey: chatKeys.messagesPage(sessionId),
    queryFn: async ({ pageParam }) =>
      cacheChatMessagePage(await api.listChatMessagesPage(sessionId, { before: pageParam, limit })),
    initialPageParam: null as { created_at: string; id: string } | null,
    getNextPageParam: (lastPage) =>
      lastPage.has_more ? lastPage.next_cursor ?? undefined : undefined,
    enabled: !!sessionId,
    staleTime: Infinity,
  });
}

type TailRefresh = { dirty: boolean };
const tailRefreshes = new WeakMap<QueryClient, Map<string, TailRefresh>>();

/** Ordinary chat events only refresh the latest page. Keep full invalidation
 * for cancellation/deletion/reconnect. Never race an in-flight older-page
 * fetch, and never write a result into a different workspace or removed cache.
 */
export function syncLatestChatMessageQueries(qc: QueryClient, sessionId: string): void {
  void qc.invalidateQueries({ queryKey: chatKeys.messages(sessionId) });
  const key = chatKeys.messagesPage(sessionId);
  const query = qc.getQueryCache().find({ queryKey: key, exact: true });
  if (!query?.isActive()) {
    void qc.invalidateQueries({ queryKey: key, refetchType: "none" });
    return;
  }
  const workspace = getCurrentWsId();
  const slug = getCurrentSlug();
  let refreshes = tailRefreshes.get(qc);
  if (!refreshes) {
    refreshes = new Map();
    tailRefreshes.set(qc, refreshes);
  }
  const id = `${workspace ?? ""}:${sessionId}`;
  const existing = refreshes.get(id);
  if (existing) {
    existing.dirty = true;
    return;
  }
  const job: TailRefresh = { dirty: true };
  refreshes.set(id, job);
  const sameScope = () => getCurrentWsId() === workspace && getCurrentSlug() === slug
    && qc.getQueryCache().find({ queryKey: key, exact: true }) === query;
  const finish = () => refreshes.delete(id);
  const schedule = (delay = 80) => { setTimeout(() => void refresh(), delay); };
  const refresh = async () => {
    if (!sameScope()) { finish(); return; }
    if (!query.isActive()) {
      void qc.invalidateQueries({ queryKey: key, refetchType: "none" });
      finish();
      return;
    }
    // fetchNextPage/invalidation writes a snapshot of the entire infinite
    // query; wait rather than overwrite that snapshot with a concurrent tail.
    if (query.state.fetchStatus === "paused") {
      void qc.invalidateQueries({ queryKey: key, refetchType: "none" });
      finish();
      return;
    }
    if (query.state.fetchStatus === "fetching") { schedule(250); return; }
    const before = qc.getQueryData<InfiniteData<CachedChatMessagesPage>>(key);
    if (!before?.pages.length) {
      finish();
      void qc.invalidateQueries({ queryKey: key });
      return;
    }
    job.dirty = false;
    const dataVersion = query.state.dataUpdateCount;
    try {
      const fresh = await api.listChatMessagesPage(sessionId, { limit: before.pages[0]?.limit ?? 50 });
      if (!sameScope()) { finish(); return; }
      if (query.state.fetchStatus !== "idle"
        || query.state.dataUpdateCount !== dataVersion
        || qc.getQueryData(key) !== before) {
        job.dirty = true;
      } else {
        const merged = mergeLatestChatMessagePage(before, fresh);
        if (!merged) {
          finish();
          void qc.invalidateQueries({ queryKey: key });
          return;
        }
        qc.setQueryData(key, merged);
      }
    } catch {
      if (sameScope()) void qc.invalidateQueries({ queryKey: key });
      finish();
      return;
    }
    if (job.dirty) schedule();
    else finish();
  };
  schedule();
}

/**
 * Pending run for a chat session — the "is something still running?" signal.
 * Refetched via WS invalidation in useRealtimeSync when chat:message / chat:done
 * / run:completed / run:failed arrive.
 */
export function pendingChatRunOptions(sessionId: string) {
  return queryOptions({
    queryKey: chatKeys.pendingRun(sessionId),
    queryFn: () => api.getPendingChatRun(sessionId),
    enabled: !!sessionId,
    staleTime: Infinity,
  });
}

/**
 * Durable deferred-cancellation draft restores for a session (#5219).
 * staleTime 0 deliberately overrides the app-wide Infinity default: this is
 * the recovery path for a client that MISSED the chat:cancel_finalized
 * broadcast, so it must actually refetch on every composer mount (an
 * Infinity-fresh cache would pin the first result forever). WS reconnects
 * additionally invalidate chatKeys.draftRestoresAll() in useRealtimeSync,
 * and the initiator's realtime handler invalidates this key when the event
 * does arrive. The response is tiny (usually empty), so the extra fetches
 * are negligible.
 */
export function chatDraftRestoresOptions(sessionId: string) {
  return queryOptions({
    queryKey: chatKeys.draftRestores(sessionId),
    queryFn: () => api.listChatDraftRestores(sessionId),
    enabled: !!sessionId,
    staleTime: 0,
  });
}

/**
 * Timeline for a single run — rendered by both the live chat view (while a
 * run is running) and AssistantMessage (for completed runs). WS
 * `run:message` events seed this cache in real time via useRealtimeSync.
 */
export function agentRunMessagesOptions(runId: string) {
  return queryOptions({
    queryKey: chatKeys.agentRunMessages(runId),
    queryFn: async ({ client }) => {
      const rows = await api.listAgentRunMessages(runId);
      // Reopening a slept/evicted live conversation rehydrates from the DB.
      // Reconcile events received during that request instead of overwriting
      // them with an earlier server snapshot.
      const buffered = client.getQueryData<AgentRunMessagePayload[]>(chatKeys.agentRunMessages(runId));
      return mergeAgentRunMessagesBySeq(rows, buffered ?? []);
    },
    enabled: isAgentRunMessageRunId(runId),
    staleTime: Infinity,
  });
}

/**
 * Aggregate of in-flight chat runs for the current user in this workspace.
 * Drives the FAB "running" indicator while the chat window is minimised —
 * no per-session query is active then, so we need this roll-up.
 */
export function pendingChatRunsOptions(wsId: string) {
  return queryOptions({
    queryKey: chatKeys.pendingRuns(wsId),
    queryFn: () => api.listPendingChatRuns(),
    staleTime: Infinity,
  });
}

/**
 * Boolean "is any chat run running for me right now" — the cheap sibling of
 * pendingChatRunsOptions. The FAB uses this (with `enabled: !isOpen`) so the
 * minimised chat button never fetches or holds the full run list; the
 * detailed list is reserved for the open ChatWindow (history + stop flows).
 * Both caches are kept in sync by the run-lifecycle WS handlers.
 */
export function hasPendingChatRunsOptions(wsId: string) {
  return queryOptions({
    queryKey: chatKeys.pendingRunsHasAny(wsId),
    queryFn: () => api.hasAnyPendingChatRuns(),
    staleTime: Infinity,
  });
}
