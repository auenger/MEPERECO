import test from "node:test";
import assert from "node:assert/strict";
import {
  cacheChatMessagePage,
  mergeLatestChatMessagePage,
  mergeAgentRunMessagesBySeq,
} from "./cache-merge.ts";

const message = (id, content = id) => ({ id, content });
const page = (ids, has_more = true) => ({
  messages: ids.map((id) => message(id)),
  has_more,
  limit: 3,
  next_cursor: has_more ? { id: ids[0], created_at: "2026-01-01T00:00:00Z" } : null,
});
const cache = (...pages) => ({
  pages: pages.map(cacheChatMessagePage),
  pageParams: pages.map((_, i) => i === 0 ? null : pages[i - 1].next_cursor),
});
const ids = (result) => result.pages.map((p) => p.messages.map((m) => m.id));

test("append latest rows and preserve older cursor and page identity", () => {
  const old = cache(page(["d", "e", "f"]), page(["a", "b", "c"], false));
  const fresh = page(["e", "f", "g"]);
  fresh.messages[1] = message("f", "authoritative");
  const merged = mergeLatestChatMessagePage(old, fresh);
  assert.deepEqual(ids(merged), [["d", "e", "f", "g"], ["a", "b", "c"]]);
  assert.equal(merged.pages[0].messages[2].content, "authoritative");
  assert.equal(merged.pages[0].confirmedTailId, "g");
  assert.equal(merged.pages[0].next_cursor, old.pages[0].next_cursor);
  assert.equal(merged.pages[1], old.pages[1]);
  assert.equal(merged.pageParams, old.pageParams);
});

test("optimistic append does not falsely prove continuity after a gap", () => {
  const old = cache(page(["a", "b", "c"]));
  old.pages[0].messages.push(message("z"));
  assert.equal(mergeLatestChatMessagePage(old, page(["x", "y", "z"])), null);
});

test("server overlap with an older page falls back rather than duplicating rows", () => {
  const old = cache(page(["d", "e", "f"]), page(["a", "b", "c"], false));
  assert.equal(mergeLatestChatMessagePage(old, page(["c", "d", "f"])), null);
});

test("a window extending before the head cursor must rebuild even if older pages were not loaded", () => {
  const old = cache(page(["d", "e", "f"]));
  assert.equal(mergeLatestChatMessagePage(old, page(["c", "d", "f"])), null);
});

test("seeded first message is replaced by authoritative page", () => {
  const old = { pages: [page(["new"], false)], pageParams: [null] };
  const merged = mergeLatestChatMessagePage(old, page(["a", "b", "new"]));
  assert.deepEqual(ids(merged), [["a", "b", "new"]]);
  assert.equal(merged.pages[0].has_more, true);
});

test("complete latest response reconciles deleted rows", () => {
  const old = cache(page(["a", "b", "c"], false));
  const merged = mergeLatestChatMessagePage(old, page(["a", "c"], false));
  assert.deepEqual(ids(merged), [["a", "c"]]);
  assert.equal(merged.pages[0].has_more, false);
});

test("history that was complete stays complete when new messages cross page size", () => {
  const old = cache(page(["a", "b", "c"], false));
  const merged = mergeLatestChatMessagePage(old, page(["b", "c", "d"]));
  assert.deepEqual(ids(merged), [["a", "b", "c", "d"]]);
  assert.equal(merged.pages[0].has_more, false);
});

test("monotonic run batches append and duplicate/out-of-order batches merge", () => {
  const initial = [{ seq: 1, content: "first" }, { seq: 3 }];
  const next = mergeAgentRunMessagesBySeq(initial, [{ seq: 4 }, { seq: 5 }]);
  assert.deepEqual(next.map((m) => m.seq), [1, 3, 4, 5]);
  assert.equal(next[0], initial[0]);
  const merged = mergeAgentRunMessagesBySeq(next, [{ seq: 3, content: "duplicate" }, { seq: 2 }, { seq: 2 }, { seq: 6 }]);
  assert.deepEqual(merged.map((m) => m.seq), [1, 2, 3, 4, 5, 6]);
  assert.equal(merged[2], initial[1]);
  assert.equal(mergeAgentRunMessagesBySeq(merged, [{ seq: 2 }, { seq: 6 }]), merged);
});
