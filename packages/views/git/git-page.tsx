"use client";

import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { GitBranch, GitPullRequest, FolderKanban, Plug, RefreshCw, Link2, FileDiff, X, ListFilter } from "lucide-react";
import { Button } from "@silieco/ui/components/ui/button";
import { Checkbox } from "@silieco/ui/components/ui/checkbox";
import { GitSelect } from "./git-select";
import { GitChangesDialog } from "./git-changes-dialog";
import { Input } from "@silieco/ui/components/ui/input";
import { useCurrentWorkspace, useWorkspacePaths } from "@silieco/core/paths";
import { useWorkspaceId } from "@silieco/core/hooks";
import { gitOverviewOptions, gitBindingsOptions, gitPullRequestsOptions } from "@silieco/core/git";
import { useCreateProjectResource, useUpdateProjectResource } from "@silieco/core/projects";
import { vcsConnectionsOptions } from "@silieco/core/vcs";
import { githubInstallationsOptions } from "@silieco/core/github";
import { api } from "@silieco/core/api";
import type { GitBinding, GitOverview, GitRepositoryIntegration, GitPullRequestItem, GitTaskReference } from "@silieco/core/types";
import { toast } from "sonner";
import { AppLink } from "../navigation";
import { useT } from "../i18n";
import { ProjectResourcesSection } from "../projects/components/project-resources-section";
import { PullRequestRow } from "../tasks/components/pull-request-list";

const sectionClass = "rounded-lg border border-border bg-card p-4";

function LoadError({ retry }: { retry: () => void }) {
  const { t } = useT("git");
  return <div role="alert" className="flex flex-wrap items-center justify-between gap-3 rounded-lg border border-border p-4 text-caption text-muted-foreground">
    <span>{t($ => $.error)}</span><Button variant="outline" size="sm" onClick={retry}><RefreshCw className="size-3.5" />{t($ => $.retry)}</Button>
  </div>;
}
function BindingState({ state }: { state: GitBinding["sync_state"] }) {
  const { t } = useT("git");
  const labels = {
    git_only: t($ => $.git_only), disconnected: t($ => $.disconnected),
    pending_projection: t($ => $.pending_projection), waiting_events: t($ => $.waiting_events), received: t($ => $.received),
  };
  return <span className="inline-flex items-center gap-1.5 rounded-md bg-muted px-2 py-1 text-micro text-muted-foreground"><span className={state === "received" ? "size-1.5 rounded-full bg-emerald-500" : "size-1.5 rounded-full bg-muted-foreground/50"} />{labels[state] ?? t($ => $.unknown)}</span>;
}
function useGitConnections(wsId: string) {
  const vcs = useQuery(vcsConnectionsOptions(wsId));
  const github = useQuery(githubInstallationsOptions(wsId));
  return {
    entries: [
      ...(vcs.data?.connections ?? []).map(c => ({ kind: "vcs" as const, id: c.id, label: `${c.provider} · ${c.instance_url}` })),
      ...(github.data?.installations ?? []).map(c => ({ kind: "github_app" as const, id: c.id, label: `GitHub · ${c.account_login}` })),
    ],
    loading: vcs.isPending || github.isPending,
    error: vcs.isError || github.isError,
  };
}
function ConnectionSummary({ wsId }: { wsId: string }) {
  const { t } = useT("git");
  const p = useWorkspacePaths();
  const { entries, loading, error } = useGitConnections(wsId);
  return <section className={sectionClass}>
    <div className="flex flex-wrap items-center justify-between gap-2"><h2 className="flex items-center gap-2 text-body font-medium"><Plug className="size-4 text-muted-foreground" />{t($ => $.connections)}</h2><AppLink className="text-caption text-primary underline-offset-4 hover:underline" href={p.settings() + "?tab=repositories"}>{t($ => $.settings)}</AppLink></div>
    {loading ? <p className="mt-3 text-caption text-muted-foreground">{t($ => $.loading)}</p> : error ? <p className="mt-3 text-caption text-muted-foreground">{t($ => $.error)}</p> : entries.length ? <div className="mt-3 flex flex-wrap gap-2">{entries.map(e => <span key={e.id} className="rounded-md border px-2.5 py-1.5 text-caption">{e.label}</span>)}</div> : <p className="mt-3 text-caption text-muted-foreground">{t($ => $.no_connections)}</p>}
  </section>;
}
function RepositoryIntegrationEditor({ binding, wsId }: { binding: GitBinding; wsId: string }) {
  const { t } = useT("git");
  const ref = binding.resource.resource_ref;
  const inRef = ref.integration;
  const [open, setOpen] = useState(false);
  const [choice, setChoice] = useState(inRef ? `${inRef.kind}:${inRef.connection_id}` : "");
  const [path, setPath] = useState(inRef?.repository_path ?? binding.repository_path ?? "");
  const { entries, loading, error } = useGitConnections(wsId);
  const update = useUpdateProjectResource(wsId, binding.resource.project_id);
  const save = async () => {
    const option = entries.find(e => `${e.kind}:${e.id}` === choice);
    if (choice && (!option || !path.trim())) { toast.error(t($ => $.path_required)); return; }
    let integration: GitRepositoryIntegration | null = null;
    if (option) {
      integration = { kind: option.kind, connection_id: option.id, repository_path: path.trim() };
      if (inRef?.kind === option.kind && inRef.connection_id === option.id && inRef.repository_path === integration.repository_path && inRef.external_repo_id) integration.external_repo_id = inRef.external_repo_id;
    }
    try {
      await update.mutateAsync({ resourceId: binding.resource.id, data: { resource_ref: { ...ref, integration } } });
      toast.success(t($ => $.saved)); setOpen(false);
    } catch (e) { toast.error(e instanceof Error ? e.message : t($ => $.save_error)); }
  };
  if (!open) return <Button variant="ghost" size="sm" onClick={() => setOpen(true)}><Link2 className="size-3.5" />{t($ => $.configure)}</Button>;
  return <div className="mt-2 grid gap-2 rounded-md border border-border p-3">
    <label className="grid gap-1 text-caption">{t($ => $.connection)}
      <GitSelect label={t($ => $.connection)} className="w-full" value={choice} onChange={setChoice} disabled={loading || error || update.isPending} options={[
        { value: "", label: t($ => $.none) },
        ...(choice && !entries.some(e => `${e.kind}:${e.id}` === choice) ? [{ value: choice, label: t($ => $.disconnected) }] : []),
        ...entries.map(e => ({ value: `${e.kind}:${e.id}`, label: e.label })),
      ]} />
    </label>
    {choice && <label className="grid gap-1 text-caption">{t($ => $.path)}<Input value={path} onChange={e => setPath(e.target.value)} placeholder={t($ => $.path_placeholder)} disabled={update.isPending} /></label>}
    <div className="flex justify-end gap-2"><Button variant="ghost" size="sm" onClick={() => setOpen(false)} disabled={update.isPending}>{t($ => $.cancel)}</Button><Button size="sm" onClick={save} disabled={loading || error || update.isPending}>{t($ => $.save)}</Button></div>
  </div>;
}
function BindingCard({ binding, wsId, manage, workspaceView }: { binding: GitBinding; wsId: string; manage?: boolean; workspaceView?: boolean }) {
  const { t } = useT("git");
  const p = useWorkspacePaths();
  const ref = binding.resource.resource_ref;
  return <div className="rounded-lg border border-border p-3">
    <div className="flex flex-wrap items-start justify-between gap-2">
      <div className="min-w-0 flex-1">
        {workspaceView && <AppLink href={p.projectGit(binding.resource.project_id)} className="block truncate text-caption text-primary hover:underline">{binding.project_title}</AppLink>}
        <div className="flex items-center gap-2"><GitBranch className="size-4 shrink-0 text-muted-foreground" /><span className="truncate font-medium" title={ref.url}>{binding.resource.label || binding.repository_path || ref.url}</span>{ref.primary && <span className="text-micro text-muted-foreground">{t($ => $.primary)}</span>}</div>
        <p className="mt-1 break-all text-caption text-muted-foreground">{ref.url}{ref.ref ? ` · ${ref.ref}` : ""}</p>
        {binding.instance_url && <p className="mt-1 truncate text-caption text-muted-foreground">{binding.provider} · {binding.instance_url}</p>}
      </div>
      <BindingState state={binding.sync_state} />
    </div>
    {manage && <RepositoryIntegrationEditor key={JSON.stringify(ref.integration)} binding={binding} wsId={wsId} />}
  </div>;
}
function Pagination({ cursor, next, back, forward }: { cursor: string | undefined; next: string | null | undefined; back: () => void; forward: (cursor: string) => void }) {
  const { t } = useT("git");
  return <div className="flex items-center justify-end gap-2"><Button size="sm" variant="outline" disabled={!cursor} onClick={back}>{t($ => $.previous)}</Button><Button size="sm" variant="outline" disabled={!next} onClick={() => next && forward(next)}>{t($ => $.next)}</Button></div>;
}
function PullRequests({ wsId, projectId, projectFilter }: { wsId: string; projectId?: string; projectFilter?: string }) {
  const { t } = useT("git");
  const p = useWorkspacePaths();
  const [state, setState] = useState(""); const [repository, setRepository] = useState(""); const [linked, setLinked] = useState(false);
  const [history, setHistory] = useState<string[]>([]);
  const [task, setTask] = useState<GitTaskReference | null>(null);
  const [taskQuery, setTaskQuery] = useState("");
  const prefix = useCurrentWorkspace()?.task_prefix;
  const queryText = taskQuery.trim();
  const taskNumber = prefix && queryText.toUpperCase().startsWith(`${prefix.toUpperCase()}-`) ? queryText.slice(prefix.length + 1) : "";
  const normalizedTaskQuery = /^\d+$/.test(taskNumber) ? taskNumber : queryText;
  const [changePR, setChangePR] = useState<GitPullRequestItem | null>(null);
  const cursor = history.at(-1);
  const projects = useQuery({queryKey:["git",wsId,"projects-for-copy"],queryFn:()=>api.listProjects()});
  const q = useQuery(gitPullRequestsOptions(wsId, projectId, { cursor, state, repository:repository.trim(), linked_only: linked, project_id: projectFilter, task_id: task?.id, task_query: task ? undefined : normalizedTaskQuery }));
  const reset = () => setHistory([]);
  return <section className={sectionClass}>
    <h2 className="flex items-center gap-2 text-body font-medium"><GitPullRequest className="size-4 text-muted-foreground" />{t($ => $.recent)}</h2>
    <div className="my-3 flex flex-wrap items-center gap-2">
      <GitSelect label={t($ => $.all_states)} value={state} onChange={value => {setState(value);reset();}} options={[
        { value: "", label: t($ => $.all_states) }, { value: "open", label: t($ => $.open) }, { value: "draft", label: t($ => $.draft) }, { value: "merged", label: t($ => $.merged) }, { value: "closed", label: t($ => $.closed) },
      ]} />
      <Input aria-label={t($ => $.repository_filter)} placeholder={t($ => $.repository_filter)} className="w-full sm:w-60" value={repository} onChange={e => {setRepository(e.target.value);reset();}} />
      <Input aria-label={t($ => $.task_filter)} placeholder={t($ => $.task_filter)} className="w-full sm:w-52" value={taskQuery} disabled={!!task} onChange={e => {setTaskQuery(e.target.value);reset();}} />
      <label className="flex items-center gap-2 text-caption text-muted-foreground"><Checkbox aria-label={t($ => $.linked)} checked={linked} onCheckedChange={value => {setLinked(value);reset();}} />{t($ => $.linked)}</label>
    </div>
    {task && <div className="mb-3 flex items-center gap-2"><span className="text-caption text-muted-foreground">{t($ => $.task_filter_active)}</span><Button variant="outline" size="sm" className="max-w-full" onClick={() => {setTask(null);setTaskQuery("");reset();}}><span className="truncate">{prefix ? `${prefix}-${task.number}` : `#${task.number}`} {task.title}</span><X className="size-3.5" /></Button></div>}
    {q.isError ? <LoadError retry={() => void q.refetch()} /> : q.isPending ? <p className="p-4 text-caption text-muted-foreground">{t($ => $.loading)}</p> : !q.data.pull_requests.length ? <p className="rounded-md border border-dashed p-5 text-caption text-muted-foreground">{state || repository.trim() || linked || task || taskQuery.trim() || projectFilter ? t($ => $.no_matching_pr) : t($ => $.empty_pr)}</p> : <div className="divide-y divide-border">
      {q.data.pull_requests.map(pr => <article key={pr.id} className="py-3">
        <div className="flex items-start gap-3"><div className="min-w-0 flex-1"><PullRequestRow pr={pr} /></div><Button variant="outline" size="sm" onClick={() => setChangePR(pr)}><FileDiff className="size-3.5" />{t($ => $.view_changes)}</Button></div>
        <div className="ml-4 flex flex-wrap items-center gap-2 text-micro text-muted-foreground">
          <span>{pr.instance_url}</span><span>{pr.checks_conclusion === "passed" ? t($ => $.passed) : pr.checks_conclusion === "failed" ? t($ => $.failed) : pr.checks_conclusion === "pending" ? t($ => $.pending) : t($ => $.unknown_ci)}</span>
        </div>
        {!projectId && pr.project_ids.length > 0 && <div className="ml-4 mt-2 flex flex-wrap gap-2"><span className="text-caption text-muted-foreground">{t($=>$.projects)}:</span>{pr.project_ids.map(id=><AppLink key={id} href={p.projectGit(id)} className="rounded-md bg-muted px-2 py-1 text-caption hover:underline">{projects.data?.projects.find(project=>project.id===id)?.title ?? id.slice(0,8)}</AppLink>)}</div>}
        {pr.tasks.length > 0 && <div className="ml-4 mt-2 flex flex-wrap gap-2"><span className="text-caption text-muted-foreground">{t($ => $.tasks)}:</span>{pr.tasks.map(task => <div key={task.id} className="flex max-w-full items-center rounded-md border"><AppLink className="min-w-0 truncate px-2 py-1 text-caption hover:bg-muted" href={p.taskDetail(task.id)}>{prefix ? `${prefix}-${task.number}` : `#${task.number}`} {task.title}{task.reference_only ? ` · ${t($ => $.reference)}` : ""}</AppLink><Button variant="ghost" size="icon-sm" title={t($ => $.filter_by_task)} aria-label={t($ => $.filter_by_task)} onClick={() => {setTask(task);setTaskQuery("");reset();}}><ListFilter className="size-3.5" /></Button></div>)}</div>}
      </article>)}
    </div>}
    <div className="mt-3"><Pagination cursor={cursor} next={q.data?.next_cursor} back={() => setHistory(h => h.slice(0,-1))} forward={next => setHistory(h => [...h,next])} /></div>
    {changePR && <GitChangesDialog key={changePR.id} wsId={wsId} projectId={projectId} projectFilter={projectFilter} pr={changePR} onClose={() => setChangePR(null)} />}
  </section>;
}
function Stats({ data }: { data: GitOverview }) {
  const { t } = useT("git");
  return <div className="grid grid-cols-2 gap-2 lg:grid-cols-4">
    {[ [t($ => $.repositories),data.binding_count,GitBranch], [t($ => $.projects),data.project_count,FolderKanban], [t($ => $.connected),data.connected_binding_count,Plug], [t($ => $.pull_requests),data.pull_request_count,GitPullRequest] ].map(([label,value,Icon]) => {
      const StatIcon = Icon as typeof GitBranch;
      return <div key={String(label)} className={sectionClass}><div className="flex items-center gap-2 text-caption text-muted-foreground"><StatIcon className="size-4" />{String(label)}</div><div className="mt-2 text-2xl font-semibold tabular-nums">{String(value)}</div></div>;
    })}
  </div>;
}
function DefaultRepositoryCopy({ wsId, defaults }: { wsId: string; defaults: GitOverview["default_repos"] }) {
  const { t } = useT("git");
  const [projectId,setProjectId] = useState(""); const [repo,setRepo] = useState("");
  const q = useQuery({queryKey:["git",wsId,"projects-for-copy"],queryFn:()=>api.listProjects()});
  const create = useCreateProjectResource(wsId,projectId);
  const copy = async () => {
    if (!projectId || !repo) return;
    try { await create.mutateAsync({resource_type:"github_repo",resource_ref:{url:repo}}); toast.success(t($ => $.copied)); }
    catch(e){toast.error(e instanceof Error ? e.message : t($ => $.copy_error));}
  };
  return <section className={sectionClass}><h2 className="font-medium">{t($ => $.default_repos)}</h2><p className="mt-1 text-caption text-muted-foreground">{t($ => $.default_description)}</p>
    {!!defaults?.length && <ul className="mt-3 space-y-1 text-caption text-muted-foreground">{defaults.map(r=><li key={r.url} className="break-all">{/^https?:\/\//i.test(r.url) ? <a href={r.url} target="_blank" rel="noopener noreferrer" className="hover:text-foreground hover:underline">{r.url}</a> : r.url}</li>)}</ul>}
    {!!defaults?.length && <div className="mt-3 flex flex-wrap gap-2"><GitSelect label={t($ => $.select_project)} className="w-48" value={projectId} onChange={setProjectId} options={[{value:"",label:t($=>$.select_project)},...(q.data?.projects ?? []).map(p=>({value:p.id,label:p.title}))]} /><GitSelect label={t($=>$.select_repo)} className="w-72" value={repo} onChange={setRepo} options={[{value:"",label:t($=>$.select_repo)},...defaults.map(r=>({value:r.url,label:r.url}))]} /><Button size="sm" disabled={!projectId||!repo||create.isPending} onClick={copy}>{t($=>$.copy)}</Button></div>}
  </section>;
}
export function WorkspaceGitPage({ embedded = false }: { embedded?: boolean } = {}) {
  const { t } = useT("git"); const wsId = useWorkspaceId();
  const overview = useQuery(gitOverviewOptions(wsId));
  const [history,setHistory] = useState<string[]>([]); const [project,setProject] = useState("");
  const cursor = history.at(-1);
  const bindings = useQuery(gitBindingsOptions(wsId,{cursor,project_id:project}));
  const projects = useQuery({queryKey:["git",wsId,"projects-for-copy"],queryFn:()=>api.listProjects()});
  return <div className="flex h-full min-h-0 flex-col">{!embedded && <div className="shrink-0 border-b px-5 py-4"><h1 className="flex items-center gap-2 text-title font-semibold"><GitBranch className="size-5" />{t($=>$.title)}</h1><p className="text-caption text-muted-foreground">{t($=>$.subtitle)}</p></div>}
    <div className="min-h-0 flex-1 overflow-auto"><div className="mx-auto max-w-6xl space-y-4 p-5">
      {overview.isError ? <LoadError retry={()=>void overview.refetch()} /> : overview.data ? <Stats data={overview.data} /> : <p className="text-caption text-muted-foreground">{t($=>$.loading)}</p>}
      <ConnectionSummary wsId={wsId} />
      <section className={sectionClass}><div className="flex flex-wrap items-center justify-between gap-2"><h2 className="font-medium">{t($=>$.bindings)}</h2><GitSelect label={t($=>$.all_projects)} className="w-48" value={project} onChange={value=>{setProject(value);setHistory([]);}} options={[{value:"",label:t($=>$.all_projects)},...(projects.data?.projects ?? []).map(p=>({value:p.id,label:p.title}))]} /></div>
        {bindings.isError ? <LoadError retry={()=>void bindings.refetch()} /> : bindings.isPending ? <p className="mt-3 text-caption text-muted-foreground">{t($=>$.loading)}</p> : !bindings.data.bindings.length ? <p className="mt-3 text-caption text-muted-foreground">{t($=>$.empty_bindings)}</p> : <div className="mt-3 grid gap-2 md:grid-cols-2">{bindings.data.bindings.map(b=><BindingCard key={b.resource.id} binding={b} wsId={wsId} workspaceView />)}</div>}
        <div className="mt-3"><Pagination cursor={cursor} next={bindings.data?.next_cursor} back={()=>setHistory(h=>h.slice(0,-1))} forward={c=>setHistory(h=>[...h,c])} /></div>
      </section>
      <PullRequests key={project} wsId={wsId} projectFilter={project} />
      <DefaultRepositoryCopy wsId={wsId} defaults={overview.data?.default_repos} />
    </div></div></div>;
}
function RepoModeControl({ projectId, wsId, mode, canManage }: { projectId: string; wsId: string; mode: GitOverview["repo_mode"]; canManage: boolean }) {
  const { t } = useT("git");
  const qc=useQueryClient();
  const [selected,setSelected]=useState(mode);
  const update=useMutation({mutationFn:()=>api.setProjectGitRepoMode(projectId,selected),onSuccess:()=>{void qc.invalidateQueries({queryKey:["git",wsId]});toast.success(t($=>$.mode_saved));},onError:e=>toast.error(e instanceof Error?e.message:t($=>$.mode_error))});
  return <section className={sectionClass}><h2 className="font-medium">{t($=>$.repo_mode)}</h2>
    <div className="mt-3 flex flex-wrap items-center gap-2"><GitSelect label={t($=>$.repo_mode)} className="w-full sm:w-80" value={selected} disabled={!canManage||update.isPending} onChange={value=>setSelected(value as GitOverview["repo_mode"])} options={[
      {value:"inherit",label:t($=>$.mode_inherit)}, {value:"project_only",label:t($=>$.mode_project_only)}, {value:"none",label:t($=>$.mode_none)},
    ]} />{canManage && <Button size="sm" disabled={selected===mode||update.isPending} onClick={()=>update.mutate()}>{t($=>$.save)}</Button>}</div>
    <p className="mt-2 text-caption text-muted-foreground">{t($=>$.mode_warning)}</p>
  </section>;
}
export function ProjectGitTab({ projectId,canManage }: { projectId:string;canManage:boolean }) {
  const { t } = useT("git");const wsId=useWorkspaceId();
  const overview=useQuery(gitOverviewOptions(wsId,projectId));
  const [history,setHistory]=useState<string[]>([]);const cursor=history.at(-1);
  const bindings=useQuery(gitBindingsOptions(wsId,{project_id:projectId,cursor}));
  return <div className="flex-1 min-h-0 overflow-auto"><div className="mx-auto max-w-5xl space-y-4 p-5">
    <p className="text-caption text-muted-foreground">{t($=>$.project_description)}</p>
    {overview.isError ? <LoadError retry={()=>void overview.refetch()} /> : overview.data ? <Stats data={overview.data} /> : null}
    <PullRequests key={projectId} wsId={wsId} projectId={projectId} />
    {overview.data && <RepoModeControl key={overview.data.repo_mode} projectId={projectId} wsId={wsId} mode={overview.data.repo_mode} canManage={canManage} />}
    <section className={sectionClass}><ProjectResourcesSection projectId={projectId} canManage={canManage} mode="repositories" /></section>
    <ConnectionSummary wsId={wsId} />
    <section className={sectionClass}><h2 className="font-medium">{t($=>$.bindings)}</h2>
      {bindings.isError ? <LoadError retry={()=>void bindings.refetch()} /> : bindings.isPending ? <p className="mt-3 text-caption text-muted-foreground">{t($=>$.loading)}</p> : !bindings.data.bindings.length ? <p className="mt-3 text-caption text-muted-foreground">{overview.data?.repo_mode === "inherit" ? t($=>$.effective_default) : t($=>$.empty_bindings)}</p> : <div className="mt-3 grid gap-2 md:grid-cols-2">{bindings.data.bindings.map(b=><BindingCard key={b.resource.id} binding={b} wsId={wsId} manage={canManage} />)}</div>}
      <div className="mt-3"><Pagination cursor={cursor} next={bindings.data?.next_cursor} back={()=>setHistory(h=>h.slice(0,-1))} forward={c=>setHistory(h=>[...h,c])} /></div>
    </section>
  </div></div>;
}
