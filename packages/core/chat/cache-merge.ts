import type { InfiniteData } from "@tanstack/react-query";
import type { AgentRunMessagePayload } from "../types/events";
import type { ChatMessagesPage } from "../types/chat";

// Client-only marker: the newest server-confirmed row, not a speculative WS
// append. It proves that a newly fetched tail overlaps the loaded history.
export type CachedChatMessagesPage = ChatMessagesPage & { confirmedTailId?: string | null };

export function cacheChatMessagePage(page: ChatMessagesPage): CachedChatMessagesPage {
  return { ...page, confirmedTailId: page.messages.at(-1)?.id ?? null };
}

/** Merge an authoritative tail window without moving the cursor of loaded history.
 * null means continuity cannot be established; use the normal infinite refetch.
 */
export function mergeLatestChatMessagePage(
  old: InfiniteData<CachedChatMessagesPage>,
  latest: ChatMessagesPage,
): InfiniteData<CachedChatMessagesPage> | null {
  const head = old.pages[0];
  const fresh = cacheChatMessagePage(latest);
  if (!head) return { pages: [fresh], pageParams: [null] };
  if (!fresh.has_more) return { pages: [fresh], pageParams: [null] };
  // The optimistic seed for a brand-new conversation has no server cursor yet.
  if (!head.confirmedTailId) {
    return old.pages.length === 1 && !head.has_more
      ? { pages: [fresh], pageParams: [null] }
      : null;
  }
  const freshIds = new Set(fresh.messages.map((m) => m.id));
  if (!freshIds.has(head.confirmedTailId)) return null;
  const overlap = head.messages.findIndex((m) => freshIds.has(m.id));
  // Both windows are ordered by the same server cursor. If the new window
  // starts before its first shared row, it may have crossed the old page
  // boundary (even when the next page has not been loaded yet). Rebuild
  // cursor boundaries authoritatively instead of creating overlaps.
  if (overlap < 0 || head.messages[overlap]?.id !== fresh.messages[0]?.id) return null;
  // Prefix retains the historical half of the first page. The fresh suffix
  // replaces matching records (e.g. attachment IDs, failures) authoritatively.
  return {
    ...old,
    pages: [{
      ...head,
      messages: [...head.messages.slice(0, overlap), ...fresh.messages],
      confirmedTailId: fresh.confirmedTailId,
    }, ...old.pages.slice(1)],
  };
}

/**
 * Merge run-message batches into one seq-ordered, seq-deduplicated list for
 * the shared `["run-messages", runId]` cache. Existing entries win on
 * conflict, and the original array reference is preserved when nothing new
 * arrives so React Query observers don't re-render on duplicate events.
 *
 * Both the realtime `run:message` handler (a coalesced batch) and the
 * transcript backfill (a full refetch) write this cache. Routing both through
 * one helper keeps a forced backfill from blind-replacing a seq the WebSocket
 * already delivered — and keeps a late WS event from being lost to an
 * in-flight backfill.
 */
export function mergeAgentRunMessagesBySeq(
  existing: readonly AgentRunMessagePayload[],
  incoming: readonly AgentRunMessagePayload[],
): AgentRunMessagePayload[] {
  if (incoming.length === 0) return existing as AgentRunMessagePayload[];
  // Normal WebSocket delivery is monotonic. Avoid a full Set and sort
  // when the batch is strictly increasing after the last cached sequence.
  let last = existing.at(-1)?.seq ?? -Infinity;
  if (incoming.every((message) => {
    if (message.seq <= last) return false;
    last = message.seq;
    return true;
  })) return [...existing, ...incoming];

  const knownSeqs = new Set(existing.map((m) => m.seq));
  const fresh = incoming.filter((m) => {
    if (knownSeqs.has(m.seq)) return false;
    knownSeqs.add(m.seq);
    return true;
  });
  if (fresh.length === 0) return existing as AgentRunMessagePayload[];
  return [...existing, ...fresh].sort((a, b) => a.seq - b.seq);
}
