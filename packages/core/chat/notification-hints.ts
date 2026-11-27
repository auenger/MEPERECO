import type { ChatSession } from "../types/chat";
import type { ChatDonePayload, ChatMessageEventPayload, ChatNotificationPayload } from "../types/events";

/** A workspace-wide event is only a hint: the session list must authorize it. */
export type ChatNoticeHint = Omit<ChatNotificationPayload, "mentioned" | "created_at"> & {
  created_at?: string;
  mentioned?: boolean;
  legacyRole?: "user" | "assistant";
  senderUserId?: string;
  expectedContent?: string;
};

export function legacyChatMessageHint(
  workspaceId: string | null | undefined, userId: string | undefined,
  event: ChatMessageEventPayload, actorId?: string, actorType?: string,
): ChatNoticeHint | null {
  // Assistant streaming fragments are not new persisted replies; chat:done
  // is the authoritative completion event. Never notify the sender.
  const senderUserId = event.sender_user_id || (actorType === "member" ? actorId : undefined);
  if (!workspaceId || !userId || event.role !== "user" || !senderUserId || senderUserId === userId || !event.message_id || !event.chat_session_id) return null;
  return {
    workspace_id: workspaceId, recipient_id: userId, chat_session_id: event.chat_session_id,
    message_id: event.message_id, created_at: event.created_at, legacyRole: "user",
    senderUserId, expectedContent: event.content,
  };
}

export function legacyChatDoneHint(
  workspaceId: string | null | undefined, userId: string | undefined, event: ChatDonePayload,
): ChatNoticeHint | null {
  if (!workspaceId || !userId || !event.message_id || !event.chat_session_id) return null;
  return {
    workspace_id: workspaceId, recipient_id: userId, chat_session_id: event.chat_session_id,
    message_id: event.message_id, created_at: event.created_at, legacyRole: "assistant",
    expectedContent: event.content,
  };
}

export function authorizedUnreadNoticeSession(sessions: readonly ChatSession[], hint: ChatNoticeHint): ChatSession | undefined {
  return sessions.find((s) => s.id === hint.chat_session_id && s.workspace_id === hint.workspace_id && s.status === "active"
    && (s.unread_count ?? (s.has_unread ? 1 : 0)) > 0
    && (hint.legacyRole !== "user" || (s.kind === "group" && hint.senderUserId !== hint.recipient_id)));
}

/** List timestamps are second-precision; reply events may include fractions. */
export function authorizedNoticeContent(session: ChatSession, hint: ChatNoticeHint): string {
  const last = session.last_message;
  if (!last || !hint.created_at) return "";
  const eventTime = Date.parse(hint.created_at);
  const lastTime = Date.parse(last.created_at);
  if (!Number.isFinite(eventTime) || !Number.isFinite(lastTime) || Math.floor(eventTime / 1000) !== Math.floor(lastTime / 1000)) return "";
  if (hint.legacyRole && hint.legacyRole !== last.role) return "";
  // The workspace payload is never the displayed body. Compare it only to
  // avoid quoting another message that landed in the same second.
  if (hint.expectedContent !== undefined && hint.expectedContent !== last.content) return "";
  return last.content;
}

export function chatContentMentionsUser(content: string, userId: string): boolean {
  const pattern = /(?<![\\])\[(?:\\.|[^[\]\\\r\n])+\]\(mention:\/\/(member|all)\/([0-9a-fA-F-]+|all)\)/g;
  for (const match of content.matchAll(pattern)) {
    if (match[1] === "all" && match[2] === "all") return true;
    if (match[1] === "member" && match[2]?.toLowerCase() === userId.toLowerCase()) return true;
  }
  return false;
}

/** The same message can arrive via the legacy and personal event streams. */
export function createChatNoticeDedupe(limit = 1_000) {
  const seen = new Set<string>();
  return (hint: ChatNoticeHint) => {
    const key = `${hint.recipient_id}:${hint.workspace_id}:${hint.message_id}`;
    if (seen.has(key)) return false;
    seen.add(key);
    if (seen.size > limit) {
      const oldest = seen.values().next().value;
      if (oldest !== undefined) seen.delete(oldest);
    }
    return true;
  };
}
