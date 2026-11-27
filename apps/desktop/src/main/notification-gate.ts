/**
 * Main-process authority for native notification eligibility. Renderer focus
 * is window-local, so only main can answer whether any MEPERStudio window is in
 * the foreground and collapse the same realtime event from N renderers.
 */
export class NotificationGate {
  private readonly seenItemIds = new Set<string>();

  private readonly maxRememberedItems: number;

  constructor(maxRememberedItems = 1_000) {
    this.maxRememberedItems = maxRememberedItems;
  }

  shouldShow(itemId: string, anyWindowFocused: boolean): boolean {
    const normalizedItemId = itemId.trim();
    if (!normalizedItemId) return false;
    if (this.seenItemIds.has(normalizedItemId)) return false;

    this.seenItemIds.add(normalizedItemId);
    if (this.seenItemIds.size > this.maxRememberedItems) {
      const oldest = this.seenItemIds.values().next().value;
      if (typeof oldest === "string") this.seenItemIds.delete(oldest);
    }
    return !anyWindowFocused;
  }
}

export interface NativeNotificationPayload {
  kind?: "inbox" | "chat";
  slug: string;
  itemId: string;
  taskKey: string;
  title: string;
  body: string;
}

export function parseNativeNotificationPayload(
  value: unknown,
): NativeNotificationPayload | null {
  if (!value || typeof value !== "object") return null;
  const candidate = value as Record<string, unknown>;
  if (candidate.kind !== undefined && candidate.kind !== "inbox" && candidate.kind !== "chat") return null;
  const limits: Record<Exclude<keyof NativeNotificationPayload, "kind">, number> = {
    slug: 256,
    itemId: 256,
    taskKey: 256,
    title: 512,
    body: 2_000,
  };
  const result = {} as NativeNotificationPayload;
  for (const key of Object.keys(limits) as (Exclude<keyof NativeNotificationPayload, "kind">)[]) {
    const field = candidate[key];
    const mayBeEmpty = key === "slug" || key === "body";
    if (
      typeof field !== "string" ||
      (!mayBeEmpty && !field.trim()) ||
      field.length > limits[key]
    ) {
      return null;
    }
    result[key] = field;
  }
  if (candidate.kind === "chat") result.kind = "chat";
  return result;
}

export interface NativeNotificationReadState {
  slug: string;
  unreadChatIds?: string[];
  unreadInboxKeys?: string[];
}

export function parseNativeNotificationReadState(value: unknown): NativeNotificationReadState | null {
  if (!value || typeof value !== "object") return null;
  const candidate = value as Record<string, unknown>;
  if (typeof candidate.slug !== "string" || !candidate.slug.trim() || candidate.slug.length > 256) return null;
  const result: NativeNotificationReadState = { slug: candidate.slug };
  for (const field of ["unreadChatIds", "unreadInboxKeys"] as const) {
    const ids = candidate[field];
    if (ids === undefined) continue;
    if (!Array.isArray(ids) || !ids.every((id) => typeof id === "string" && id.length > 0 && id.length <= 256)) return null;
    result[field] = ids;
  }
  return result;
}

/** A missing list means the renderer has not loaded that surface yet. */
export function shouldDismissReadNotification(payload: NativeNotificationPayload, state: NativeNotificationReadState): boolean {
  if (payload.slug !== state.slug) return false;
  const unread = payload.kind === "chat" ? state.unreadChatIds : state.unreadInboxKeys;
  return unread !== undefined && !unread.includes(payload.taskKey);
}
