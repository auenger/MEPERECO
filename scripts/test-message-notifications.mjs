import assert from "node:assert/strict";
import test from "node:test";
import { countUnreadChatMessages } from "../packages/core/chat/unread.ts";
import { stripMentionMarkdown } from "../packages/core/markdown/strip-mention-markdown.ts";
import { authorizedNoticeContent, authorizedUnreadNoticeSession, chatContentMentionsUser, createChatNoticeDedupe, legacyChatDoneHint, legacyChatMessageHint } from "../packages/core/chat/notification-hints.ts";
import { NotificationGate, parseNativeNotificationPayload, parseNativeNotificationReadState, shouldDismissReadNotification } from "../apps/desktop/src/main/notification-gate.ts";

test("chat aggregate clears only read sessions and excludes archived sessions", () => {
  const sessions = [
    { id: "a", status: "active", unread_count: 2, has_unread_mention: true },
    { id: "b", status: "active", unread_count: 3 },
    { id: "archive", status: "archived", unread_count: 8 },
  ];
  assert.equal(countUnreadChatMessages(sessions), 5);
  assert.equal(countUnreadChatMessages(sessions, "a"), 3);
  sessions[0] = { ...sessions[0], unread_count: 0, has_unread_mention: false };
  assert.equal(countUnreadChatMessages(sessions), 3);
  sessions[1].unread_count = 0;
  assert.equal(countUnreadChatMessages(sessions), 0);
});

test("native banners remain for unread targets and close when their target is read", () => {
  const chat = { kind: "chat", slug: "space", itemId: "message", taskKey: "chat-1", title: "Group", body: "hello" };
  const inbox = { slug: "space", itemId: "item", taskKey: "task-1", title: "Task", body: "new comment" };
  assert.deepEqual(parseNativeNotificationPayload(chat), chat);
  assert.deepEqual(parseNativeNotificationPayload(inbox), inbox);
  assert.equal(parseNativeNotificationPayload({ ...chat, kind: "other" }), null);
  assert.equal(shouldDismissReadNotification(chat, { slug: "space", unreadChatIds: ["chat-1"] }), false);
  assert.equal(shouldDismissReadNotification(chat, { slug: "space", unreadChatIds: [] }), true);
  assert.equal(shouldDismissReadNotification(inbox, { slug: "space", unreadChatIds: [] }), false);
  assert.equal(shouldDismissReadNotification(inbox, { slug: "space", unreadInboxKeys: [] }), true);
  assert.equal(shouldDismissReadNotification(chat, { slug: "another", unreadChatIds: [] }), false);
  assert.equal(parseNativeNotificationReadState({ slug: "space", unreadChatIds: [42] }), null);
  assert.deepEqual(parseNativeNotificationReadState({ slug: "space", unreadChatIds: [], unreadInboxKeys: [] }), { slug: "space", unreadChatIds: [], unreadInboxKeys: [] });
});

test("background banners deduplicate across renderer windows; foreground events do not pop up", () => {
  const gate = new NotificationGate();
  assert.equal(gate.shouldShow("chat-message", false), true);
  assert.equal(gate.shouldShow("chat-message", false), false);
  assert.equal(gate.shouldShow("inbox-item", true), false);
  assert.equal(gate.shouldShow("inbox-item", false), false);
  assert.equal(stripMentionMarkdown("[@someone](mention://member/123) hello"), "@someone hello");
});

test("existing chat:message events notify only the other member of an accessible unread group", () => {
  const me = "10000000-0000-0000-0000-000000000001";
  const sender = "10000000-0000-0000-0000-000000000002";
  const event = { role: "user", chat_session_id: "group", message_id: "msg", sender_user_id: sender, created_at: "2026-11-27T02:00:00Z", content: `[@me](mention://member/${me}) hello` };
  const hint = legacyChatMessageHint("ws", me, event);
  assert.ok(hint);
  assert.equal(legacyChatMessageHint("ws", sender, event), null);
  assert.equal(legacyChatMessageHint("ws", me, { ...event, role: "assistant" }), null);
  assert.equal(legacyChatMessageHint("ws", me, { ...event, sender_user_id: undefined }), null);
  assert.ok(legacyChatMessageHint("ws", me, { ...event, sender_user_id: undefined }, sender, "member"));
  const group = { id: "group", workspace_id: "ws", kind: "group", status: "active", unread_count: 1, last_message: event };
  assert.equal(authorizedUnreadNoticeSession([], hint), undefined);
  assert.equal(authorizedUnreadNoticeSession([{ ...group, kind: "direct" }], hint), undefined);
  assert.equal(authorizedUnreadNoticeSession([{ ...group, status: "archived" }], hint), undefined);
  assert.equal(authorizedUnreadNoticeSession([{ ...group, unread_count: 0 }], hint), undefined);
  assert.equal(authorizedUnreadNoticeSession([{ ...group, workspace_id: "other" }], hint), undefined);
  assert.equal(authorizedUnreadNoticeSession([group], hint), group);
  assert.equal(authorizedNoticeContent(group, hint), event.content);
  assert.equal(chatContentMentionsUser(event.content, me), true);
  assert.equal(chatContentMentionsUser(event.content, sender), false);
  assert.equal(chatContentMentionsUser("[@all](mention://all/all)", me), true);
});

test("existing chat:done uses persisted reply data and dedupes against newer personal events", () => {
  const event = { chat_session_id: "direct", run_id: "run", message_id: "reply", content: "hello", created_at: "2026-11-27T02:00:00.123Z" };
  const hint = legacyChatDoneHint("ws", "me", event);
  assert.ok(hint);
  assert.equal(legacyChatDoneHint("ws", "me", { ...event, message_id: undefined }), null);
  const direct = { id: "direct", workspace_id: "ws", kind: "direct", status: "active", unread_count: 1, last_message: { role: "assistant", content: "hello", created_at: "2026-11-27T02:00:00Z" } };
  assert.equal(authorizedUnreadNoticeSession([direct], hint), direct);
  assert.equal(authorizedNoticeContent(direct, hint), "hello");
  assert.equal(authorizedNoticeContent({ ...direct, last_message: { ...direct.last_message, content: "different message" } }, hint), "");
  const claim = createChatNoticeDedupe();
  assert.equal(claim(hint), true);
  assert.equal(claim({ ...hint, legacyRole: undefined, mentioned: false }), false);
  assert.equal(claim({ ...hint, message_id: "another" }), true);
});
