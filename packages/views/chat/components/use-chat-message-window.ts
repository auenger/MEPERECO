"use client";

import { useMemo } from "react";
import type { InfiniteData, QueryClient } from "@tanstack/react-query";
import { chatKeys } from "@silieco/core/chat/queries";
import type { ChatMessage, ChatMessagesPage } from "@silieco/core/types";

const INITIAL_FIRST_INDEX = 1_000_000;

export function useChatMessageWindow(
  activeSessionId: string | null,
  data: InfiniteData<ChatMessagesPage> | undefined,
) {
  return useMemo(() => {
    const pages = activeSessionId ? data?.pages ?? [] : [];
    const messages = [...pages].reverse().flatMap((page) => page.messages);
    const olderCount = pages.slice(1).reduce((sum, page) => sum + page.messages.length, 0);
    return {
      messages,
      firstItemIndex: messages.length ? INITIAL_FIRST_INDEX - olderCount : 0,
    };
  }, [activeSessionId, data?.pages]);
}

export function appendChatMessageToLatestPageCache(
  qc: QueryClient,
  sessionId: string,
  message: ChatMessage,
) {
  qc.setQueryData<InfiniteData<ChatMessagesPage>>(
    chatKeys.messagesPage(sessionId),
    (old) => {
      if (!old) {
        return {
          pages: [{ messages: [message], limit: 50, has_more: false, next_cursor: null }],
          pageParams: [null],
        };
      }
      if (old.pages.some((page) => page.messages.some((m) => m.id === message.id))) {
        return old;
      }
      return {
        ...old,
        pages: old.pages.map((page, index) =>
          index === 0 ? { ...page, messages: [...page.messages, message] } : page,
        ),
      };
    },
  );
}
