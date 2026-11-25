/**
 * Centralized URL path builder. All navigation in shared packages (packages/views)
 * MUST go through this module — no hardcoded string paths.
 *
 * Two kinds of paths:
 *  - workspace-scoped: paths.workspace(slug).xxx() — carry workspace in URL
 *  - global: paths.login(), paths.newWorkspace(), paths.invite(id) — pre-workspace routes
 *
 * Why pure functions + builder pattern:
 *  - Changing a route shape (e.g. adding workspace slug prefix) becomes a single-file edit
 *  - IDs are always URL-encoded here so callers can't forget
 *  - Zero runtime deps means this module is safe in Node (tests) and browsers
 */

export type WorkResourceTab = "library" | "files" | "git";

export function parseWorkResourceTab(value: string | null | undefined): WorkResourceTab {
  return value === "files" || value === "git" ? value : "library";
}

const encode = (id: string) => encodeURIComponent(id);

function workspaceScoped(slug: string) {
  const ws = `/${encode(slug)}`;
  return {
    root: () => `${ws}/chat`,
    usage: () => `${ws}/usage`,
    tasks: () => `${ws}/tasks`,
    taskDetail: (id: string) => `${ws}/tasks/${encode(id)}`,
    projects: () => `${ws}/projects`,
    resources: (tab: WorkResourceTab = "library") => `${ws}/resources${tab === "library" ? "" : `?tab=${tab}`}`,
    git: () => `${ws}/resources?tab=git`,
    legacyGit: () => `${ws}/git`,
    projectGit: (id: string) => `${ws}/projects/${encode(id)}?section=resources&tab=git`,
    projectDetail: (id: string) => `${ws}/projects/${encode(id)}`,
    projectFiles: (id: string) => `${ws}/projects/${encode(id)}?section=resources&tab=files`,
    projectFilesStandalone: (id: string) => `${ws}/projects/${encode(id)}/files`,
    projectResources: (id: string, tab: WorkResourceTab = "library") =>
      `${ws}/projects/${encode(id)}?section=resources${tab === "library" ? "" : `&tab=${tab}`}`,
    projectSop: (id: string) => `${ws}/projects/${encode(id)}?section=sop`,
    projectWorkflowRun: (projectId: string, runId: string) =>
      `${ws}/projects/${encode(projectId)}?run=${encode(runId)}`,
    workflows: () => `${ws}/workflows`,
    workflowDetail: (id: string) => `${ws}/workflows/${encode(id)}`,
    autopilots: () => `${ws}/autopilots`,
    autopilotDetail: (id: string) => `${ws}/autopilots/${encode(id)}`,
    agents: () => `${ws}/agents`,
    newAgent: () => `${ws}/agents/new`,
    agentDetail: (id: string) => `${ws}/agents/${encode(id)}`,
    memberDetail: (id: string) => `${ws}/members/${encode(id)}`,
    squads: () => `${ws}/squads`,
    squadDetail: (id: string) => `${ws}/squads/${encode(id)}`,
    inbox: () => `${ws}/inbox`,
    chat: () => `${ws}/chat`,
    myTasks: () => `${ws}/my-tasks`,
    runtimes: () => `${ws}/runtimes`,
    runtimeDetail: (id: string) => `${ws}/runtimes/${encode(id)}`,
    runtimeSettings: (machineId: string, runtimeId: string) =>
      `${ws}/runtimes/${encode(machineId)}/runtime/${encode(runtimeId)}`,
    skills: () => `${ws}/skills`,
    skillDetail: (id: string) => `${ws}/skills/${encode(id)}`,
    settings: () => `${ws}/settings`,
    attachmentPreview: (id: string) => `${ws}/attachments/${encode(id)}/preview`,
  };
}

export const paths = {
  workspace: workspaceScoped,

  // Global (pre-workspace) routes
  login: () => "/login",
  workspaces: () => "/workspaces",
  newWorkspace: () => "/workspaces/new",
  invite: (id: string) => `/invite/${encode(id)}`,
  invitations: () => "/invitations",
  onboarding: () => "/onboarding",
  authCallback: () => "/auth/callback",
  root: () => "/",
};

export type WorkspacePaths = ReturnType<typeof workspaceScoped>;

// Prefixes — not slug names — because we match against full URL paths.
// A path is global if it equals or begins with any of these.
// Note: `/workspaces/` (trailing slash) is the prefix — `workspaces` is reserved,
// so any path starting with `/workspaces/...` is system-owned, not user-owned.
const GLOBAL_PREFIXES = ["/login", "/workspaces/", "/invite/", "/invitations", "/onboarding", "/auth/", "/logout", "/signup"];

export function isGlobalPath(path: string): boolean {
  return path === "/workspaces" || path.startsWith("/workspaces?") || GLOBAL_PREFIXES.some((p) => path === p || path.startsWith(p));
}
