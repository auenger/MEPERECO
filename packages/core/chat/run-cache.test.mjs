import test from "node:test";
import assert from "node:assert/strict";
import { QueryObserver } from "@tanstack/react-query";
import { createQueryClient } from "../query-client.ts";
import { applyObservedRunMessageBatch } from "./run-cache.ts";

test("chat idle GC defaults apply before optimistic or websocket cache entries", () => {
  const client = createQueryClient();
  try {
    for (const [key, timeout] of [
      [["chat", "messages-page", "session"], 5 * 60_000],
      [["chat", "messages", "session"], 5 * 60_000],
      [["run-messages", "run"], 3 * 60_000],
      [["tasks", "list"], 10 * 60_000],
    ]) {
      client.setQueryData(key, []);
      assert.equal(client.getQueryCache().find({ queryKey: key, exact: true }).gcTime, timeout);
    }
  } finally {
    client.clear();
  }
});

test("unobserved run never accumulates a partial cache, idle warm run becomes stale", () => {
  const client = createQueryClient();
  const key = ["run-messages", "idle"];
  try {
    applyObservedRunMessageBatch(client, key, [{ seq: 1 }]);
    assert.equal(client.getQueryState(key), undefined);
    client.setQueryData(key, [{ seq: 1 }]);
    applyObservedRunMessageBatch(client, key, [{ seq: 2 }]);
    assert.deepEqual(client.getQueryData(key).map((m) => m.seq), [1]);
    assert.equal(client.getQueryState(key).isInvalidated, true);
  } finally {
    client.clear();
  }
});

test("read-only live transcript observer still receives WS messages", () => {
  const client = createQueryClient();
  const key = ["run-messages", "observed"];
  const observer = new QueryObserver(client, { queryKey: key, enabled: false });
  const unsubscribe = observer.subscribe(() => {});
  try {
    applyObservedRunMessageBatch(client, key, [{ seq: 2 }]);
    assert.deepEqual(client.getQueryData(key).map((m) => m.seq), [2]);
    assert.equal(client.getQueryState(key).isInvalidated, true);
    // Authoritative backfill from the open transcript resets the stale marker.
    client.setQueryData(key, [{ seq: 1 }, { seq: 2 }]);
    applyObservedRunMessageBatch(client, key, [{ seq: 3 }, { seq: 2 }]);
    assert.deepEqual(client.getQueryData(key).map((m) => m.seq), [1, 2, 3]);
    assert.equal(client.getQueryState(key).isInvalidated, false);
  } finally {
    unsubscribe();
    client.clear();
  }
});

test("terminal event invalidates an idle previously viewed run without adding data", () => {
  const client = createQueryClient();
  const key = ["run-messages", "finished"];
  try {
    client.setQueryData(key, [{ seq: 1 }]);
    applyObservedRunMessageBatch(client, key);
    assert.equal(client.getQueryState(key).isInvalidated, true);
  } finally {
    client.clear();
  }
});

test("an evicted/unseen run reloads authoritative rows on wake and resumes streaming", async () => {
  const client = createQueryClient();
  const key = ["run-messages", "wake"];
  // While no UI observes the run, the server keeps rows but the client
  // does not allocate a partial transcript.
  applyObservedRunMessageBatch(client, key, [{ seq: 1 }, { seq: 2 }]);
  const observer = new QueryObserver(client, {
    queryKey: key,
    queryFn: async () => [{ seq: 1 }, { seq: 2 }],
    enabled: true,
  });
  const unsubscribe = observer.subscribe(() => {});
  try {
    await observer.refetch();
    applyObservedRunMessageBatch(client, key, [{ seq: 3 }]);
    assert.deepEqual(client.getQueryData(key).map((m) => m.seq), [1, 2, 3]);
  } finally {
    unsubscribe();
    client.clear();
  }
});
