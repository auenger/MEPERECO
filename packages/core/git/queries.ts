import { queryOptions } from "@tanstack/react-query";
import { api } from "../api";
import type { GitListParams } from "../types/git";

export const gitKeys = {
  workspace: (wsId: string) => ["git", wsId] as const,
  project: (wsId: string, projectId: string) => ["git", wsId, "project", projectId] as const,
};
export function gitOverviewOptions(wsId: string, projectId?: string) {
  return queryOptions({
    queryKey: [...(projectId ? gitKeys.project(wsId, projectId) : gitKeys.workspace(wsId)), "overview"],
    queryFn: () => api.getGitOverview(wsId, projectId),
    enabled: !!wsId,
  });
}
export function gitBindingsOptions(wsId: string, params: GitListParams = {}) {
  return queryOptions({
    queryKey: [...gitKeys.workspace(wsId), "bindings", params],
    queryFn: () => api.listGitBindings(wsId, params),
    enabled: !!wsId,
  });
}
export function gitPullRequestsOptions(wsId: string, projectId: string | undefined, params: GitListParams) {
  return queryOptions({
    queryKey: [...(projectId ? gitKeys.project(wsId, projectId) : gitKeys.workspace(wsId)), "pull-requests", params],
    queryFn: () => api.listGitPullRequests(wsId, projectId, params),
    enabled: !!wsId,
  });
}
