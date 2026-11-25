import { QueryClient } from "@tanstack/react-query";

export function createQueryClient(): QueryClient {
  const client = new QueryClient({
    defaultOptions: {
      queries: {
        staleTime: Infinity,
        gcTime: 10 * 60 * 1000, // 10 minutes
        refetchOnWindowFocus: false,
        refetchOnReconnect: true,
        retry: 1,
      },
      mutations: {
        retry: false,
      },
    },
  });

  // Register defaults before useQuery OR WS/optimistic setQueryData creates
  // an entry. TanStack retains the longest gcTime ever seen by a query, so
  // setting a shorter observer option afterwards would not reclaim it sooner.
  // Active observers are never GC'd; closing/switching the chat starts the
  // idle clock without clipping the active history or its scroll anchor.
  client.setQueryDefaults(["chat", "messages-page"], { gcTime: 5 * 60 * 1000 });
  client.setQueryDefaults(["chat", "messages"], { gcTime: 5 * 60 * 1000 });
  client.setQueryDefaults(["run-messages"], { gcTime: 3 * 60 * 1000 });
  return client;
}
