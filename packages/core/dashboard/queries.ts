import { keepPreviousData, queryOptions } from "@tanstack/react-query";
import { api } from "../api";
import type { DashboardRunKind } from "../types/agent";

export const dashboardKeys = {
  all: (wsId: string) => ["dashboard", wsId] as const,
  daily: (
    wsId: string,
    days: number,
    projectId: string | null,
    tz: string,
    runKind: DashboardRunKind = "all",
  ) => [...dashboardKeys.all(wsId), "daily", days, projectId, tz, runKind] as const,
  byAgent: (
    wsId: string,
    days: number,
    projectId: string | null,
    tz: string,
    runKind: DashboardRunKind = "all",
  ) => [...dashboardKeys.all(wsId), "by-agent", days, projectId, tz, runKind] as const,
  agentRuntime: (
    wsId: string,
    days: number,
    projectId: string | null,
    tz: string,
    runKind: DashboardRunKind = "all",
  ) => [...dashboardKeys.all(wsId), "agent-runtime", days, projectId, tz, runKind] as const,
  runTimeDaily: (
    wsId: string,
    days: number,
    projectId: string | null,
    tz: string,
    runKind: DashboardRunKind = "all",
  ) => [...dashboardKeys.all(wsId), "runtime-daily", days, projectId, tz, runKind] as const,
  failuresDaily: (
    wsId: string,
    days: number,
    projectId: string | null,
    tz: string,
    runKind: DashboardRunKind = "all",
  ) => [...dashboardKeys.all(wsId), "failures-daily", days, projectId, tz, runKind] as const,
  failuresByAgent: (
    wsId: string,
    days: number,
    projectId: string | null,
    tz: string,
    runKind: DashboardRunKind = "all",
  ) =>
    [...dashboardKeys.all(wsId), "failures-by-agent", days, projectId, tz, runKind] as const,
};

// Read the raw ledger; refresh active dashboards without requiring navigation.
const STALE_TIME = 60 * 1000;

// Range changes should keep the previous result mounted so KPI cards and
// charts transition in place instead of falling back to a full-page skeleton.
// Scope changes are deliberately excluded: carrying data across workspaces,
// projects, report kinds, or timezones would briefly display the wrong data.
function isSameDashboardScope(
  previousKey: readonly unknown[] | undefined,
  nextKey: readonly unknown[],
): boolean {
  if (!previousKey || previousKey.length !== nextKey.length) return false;
  return previousKey.every(
    (part, index) => index === 3 || Object.is(part, nextKey[index]),
  );
}

// `tz` participates in every dashboard key so a Preferences change
// repoints the cache. Every series — token rollups and the
// atq.completed_at-based run-time / failure series — slices its day boundary
// in the viewer's tz, so all the dashboard tabs always agree.
export function dashboardUsageDailyOptions(
  wsId: string,
  days: number,
  projectId: string | null,
  tz: string,
  runKind: DashboardRunKind = "all",
) {
  const queryKey = dashboardKeys.daily(wsId, days, projectId, tz, runKind);
  return queryOptions({
    queryKey,
    queryFn: () =>
      api.getDashboardUsageDaily({
        days,
        project_id: projectId ?? undefined,
        tz,
        run_kind: runKind === "all" ? undefined : runKind,
      }),
    enabled: !!wsId,
    staleTime: STALE_TIME,
    refetchInterval: STALE_TIME,
    placeholderData: (previousData, previousQuery) =>
      isSameDashboardScope(previousQuery?.queryKey, queryKey)
        ? keepPreviousData(previousData)
        : undefined,
  });
}

export function dashboardUsageByAgentOptions(
  wsId: string,
  days: number,
  projectId: string | null,
  tz: string,
  runKind: DashboardRunKind = "all",
) {
  const queryKey = dashboardKeys.byAgent(wsId, days, projectId, tz, runKind);
  return queryOptions({
    queryKey,
    queryFn: () =>
      api.getDashboardUsageByAgent({
        days,
        project_id: projectId ?? undefined,
        tz,
        run_kind: runKind === "all" ? undefined : runKind,
      }),
    enabled: !!wsId,
    staleTime: STALE_TIME,
    refetchInterval: STALE_TIME,
    placeholderData: (previousData, previousQuery) =>
      isSameDashboardScope(previousQuery?.queryKey, queryKey)
        ? keepPreviousData(previousData)
        : undefined,
  });
}

export function dashboardAgentRunTimeOptions(
  wsId: string,
  days: number,
  projectId: string | null,
  tz: string,
  runKind: DashboardRunKind = "all",
) {
  const queryKey = dashboardKeys.agentRuntime(wsId, days, projectId, tz, runKind);
  return queryOptions({
    queryKey,
    queryFn: () =>
      api.getDashboardAgentRunTime({
        days,
        project_id: projectId ?? undefined,
        tz,
        run_kind: runKind === "all" ? undefined : runKind,
      }),
    enabled: !!wsId,
    staleTime: STALE_TIME,
    refetchInterval: STALE_TIME,
    placeholderData: (previousData, previousQuery) =>
      isSameDashboardScope(previousQuery?.queryKey, queryKey)
        ? keepPreviousData(previousData)
        : undefined,
  });
}

export function dashboardRunTimeDailyOptions(
  wsId: string,
  days: number,
  projectId: string | null,
  tz: string,
  runKind: DashboardRunKind = "all",
) {
  const queryKey = dashboardKeys.runTimeDaily(wsId, days, projectId, tz, runKind);
  return queryOptions({
    queryKey,
    queryFn: () =>
      api.getDashboardRunTimeDaily({
        days,
        project_id: projectId ?? undefined,
        tz,
        run_kind: runKind === "all" ? undefined : runKind,
      }),
    enabled: !!wsId,
    staleTime: STALE_TIME,
    refetchInterval: STALE_TIME,
    placeholderData: (previousData, previousQuery) =>
      isSameDashboardScope(previousQuery?.queryKey, queryKey)
        ? keepPreviousData(previousData)
        : undefined,
  });
}

export function dashboardFailuresDailyOptions(
  wsId: string,
  days: number,
  projectId: string | null,
  tz: string,
  runKind: DashboardRunKind = "all",
) {
  const queryKey = dashboardKeys.failuresDaily(wsId, days, projectId, tz, runKind);
  return queryOptions({
    queryKey,
    queryFn: () =>
      api.getDashboardFailuresDaily({
        days,
        project_id: projectId ?? undefined,
        tz,
        run_kind: runKind === "all" ? undefined : runKind,
      }),
    enabled: !!wsId,
    staleTime: STALE_TIME,
    refetchInterval: STALE_TIME,
    placeholderData: (previousData, previousQuery) =>
      isSameDashboardScope(previousQuery?.queryKey, queryKey)
        ? keepPreviousData(previousData)
        : undefined,
  });
}

export function dashboardFailuresByAgentOptions(
  wsId: string,
  days: number,
  projectId: string | null,
  tz: string,
  runKind: DashboardRunKind = "all",
) {
  const queryKey = dashboardKeys.failuresByAgent(wsId, days, projectId, tz, runKind);
  return queryOptions({
    queryKey,
    queryFn: () =>
      api.getDashboardFailuresByAgent({
        days,
        project_id: projectId ?? undefined,
        tz,
        run_kind: runKind === "all" ? undefined : runKind,
      }),
    enabled: !!wsId,
    staleTime: STALE_TIME,
    refetchInterval: STALE_TIME,
    placeholderData: (previousData, previousQuery) =>
      isSameDashboardScope(previousQuery?.queryKey, queryKey)
        ? keepPreviousData(previousData)
        : undefined,
  });
}
