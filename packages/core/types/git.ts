import type { ProjectResource, GithubRepoResourceRef } from "./project";
import type { GitHubPullRequest } from "./github";
import type { WorkspaceRepo } from "./workspace";

export interface GitRepositoryIntegration {
  kind: "vcs" | "github_app";
  connection_id: string;
  repository_path: string;
  external_repo_id?: string;
}
export type GitBindingState = "git_only" | "pending_projection" | "disconnected" | "waiting_events" | "received";
export interface GitBinding {
  resource: ProjectResource & { resource_ref: GithubRepoResourceRef };
  project_title: string;
  project_status: string;
  repository_path: string | null;
  provider: string | null;
  instance_url: string | null;
  sync_state: GitBindingState;
}
export interface GitOverview {
  binding_count: number;
  project_count: number;
  connected_binding_count: number;
  pull_request_count: number;
  repo_mode: "inherit" | "project_only" | "none";
  bindings?: GitBinding[];
  default_repos?: WorkspaceRepo[];
}
export interface GitTaskReference {
  id: string;
  project_id: string | null;
  title: string;
  number: number;
  reference_only: boolean;
  close_intent: boolean;
}
export interface GitPullRequestItem extends GitHubPullRequest {
  instance_url: string;
  external_repo_id: string | null;
  project_ids: string[];
  tasks: GitTaskReference[];
}
export interface GitBindingPage { bindings: GitBinding[]; next_cursor: string | null }
export interface GitPullRequestPage { pull_requests: GitPullRequestItem[]; next_cursor: string | null }
export interface GitListParams { cursor?: string; project_id?: string; repository?: string; state?: string; linked_only?: boolean; task_id?: string; task_query?: string; limit?: number }

export interface GitChangedFile {
  path: string;
  previous_path?: string;
  status: string;
  additions?: number;
  deletions?: number;
  patch?: string;
  patch_truncated?: boolean;
}
export interface GitFilesPage { files: GitChangedFile[]; next_page: number | null }
