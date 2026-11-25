import type { QueryClient } from "@tanstack/react-query";
import type { AgentRunMessagePayload } from "../types/events";
import { mergeAgentRunMessagesBySeq } from "./cache-merge";

/**
 * Only retain realtime trace data for a mounted observer. Read-only observers
 * (e.g. the open task transcript dialog, enabled:false) still need WS updates.
 * Unobserved runs are persisted on the server, not accumulated in a fresh
 * renderer cache; an existing idle entry is stale until it backfills.
 */
export function applyObservedRunMessageBatch(
  client: QueryClient,
  key: readonly ["run-messages", string],
  batch?: readonly AgentRunMessagePayload[],
): void {
  const query = client.getQueryCache().find({ queryKey: key, exact: true });
  const invalidate = () => {
    void client.invalidateQueries({ queryKey: key, exact: true, refetchType: "none" });
  };
  if (!query || query.getObserversCount() === 0) {
    if (query && !query.state.isInvalidated) invalidate();
    return;
  }
  if (!batch?.length) return;
  // A disabled historical observer may receive an isolated late event.
  // Preserve its incomplete marker; explicit expansion can backfill it.
  // An open read-only live transcript still receives the streamed rows.
  const incomplete = !query.isActive() && (
    query.state.data === undefined || query.state.isInvalidated
  );
  client.setQueryData<AgentRunMessagePayload[]>(
    key,
    (old = []) => mergeAgentRunMessagesBySeq(old, batch),
  );
  if (incomplete) invalidate();
}
