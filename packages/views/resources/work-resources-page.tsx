"use client";

import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { ExternalLink, FileText, FolderOpen, GitBranch } from "lucide-react";
import { useWorkspaceId } from "@silieco/core/hooks";
import { parseWorkResourceTab, useWorkspacePaths, type WorkResourceTab } from "@silieco/core/paths";
import { projectListOptions } from "@silieco/core/projects";
import { Button } from "@silieco/ui/components/ui/button";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@silieco/ui/components/ui/select";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@silieco/ui/components/ui/tabs";
import { AppLink, useNavigation } from "../navigation";
import { useT } from "../i18n";
import { WorkspaceGitPage, ProjectGitTab } from "../git";
import { ProjectDirectoryBrowser } from "../projects/components/project-directory-browser";
import { ProjectResourcesSection } from "../projects/components/project-resources-section";
import { ProjectFilesPage, ResourcesPage } from "./resources-page";

function ResourceTabs({ tab, onChange, library, files, git }: { tab: WorkResourceTab; onChange: (tab: WorkResourceTab) => void; library: React.ReactNode; files: React.ReactNode; git: React.ReactNode }) {
  const { t } = useT("resources");
  const panelClass = "flex min-h-0 flex-1 flex-col overflow-hidden";
  return <Tabs value={tab} onValueChange={value => onChange(parseWorkResourceTab(value as string))} className="min-h-0 flex-1 gap-0 overflow-hidden">
    <div className="shrink-0 border-b px-5"><TabsList variant="line" className="h-10" aria-label={t($ => $.work_resources)}>
      <TabsTrigger value="library" className="px-3"><FileText className="size-4" />{t($ => $.library_tab)}</TabsTrigger>
      <TabsTrigger value="files" className="px-3"><FolderOpen className="size-4" />{t($ => $.files_tab)}</TabsTrigger>
      <TabsTrigger value="git" className="px-3"><GitBranch className="size-4" />Git</TabsTrigger>
    </TabsList></div>
    <TabsContent value="library" className={panelClass}>{library}</TabsContent>
    <TabsContent value="files" className={panelClass}>{files}</TabsContent>
    <TabsContent value="git" className={panelClass}>{git}</TabsContent>
  </Tabs>;
}

function OpenDirectoryTab({ projectId }: { projectId: string }) {
  const { t } = useT("resources");
  const p = useWorkspacePaths();
  return <AppLink href={p.projectFilesStandalone(projectId)} target="_blank" newTabTitle={t($ => $.files_tab)} className="inline-flex shrink-0 items-center gap-1.5 rounded-lg border px-2.5 py-1.5 text-caption hover:bg-accent focus-visible:ring-2 focus-visible:ring-ring"><ExternalLink className="size-3.5" />{t($ => $.directory_open)}</AppLink>;
}

function WorkspaceWorkingFiles() {
  const { t } = useT("resources");
  const p = useWorkspacePaths();
  const wsId = useWorkspaceId();
  const q = useQuery(projectListOptions(wsId));
  const projects = q.data ?? [];
  const [chosen, setChosen] = useState("");
  const selected = projects.find(project => project.id === chosen) ?? projects[0];
  return <div className="flex min-h-0 flex-1 flex-col">
    <div className="shrink-0 space-y-3 border-b px-5 py-4">
      <div><h2 className="text-body font-medium">{t($ => $.files_tab)}</h2><p className="text-caption text-muted-foreground">{t($ => $.workspace_files_description)}</p></div>
      {selected && <div className="flex flex-wrap items-center gap-2">
        <Select items={projects.map(project => ({ value: project.id, label: project.title }))} value={selected.id} onValueChange={value => { if (value) setChosen(value); }}><SelectTrigger aria-label={t($ => $.select_project_files)} className="w-64 max-w-full"><SelectValue /></SelectTrigger><SelectContent>{projects.map(project => <SelectItem key={project.id} value={project.id}>{project.title}</SelectItem>)}</SelectContent></Select>
        <AppLink href={p.projectFiles(selected.id)} className="text-caption text-primary hover:underline">{t($ => $.manage_project_files)}</AppLink><div className="ml-auto"><OpenDirectoryTab projectId={selected.id} /></div>
      </div>}
    </div>
    {q.isError ? <div role="alert" className="space-y-2 p-5 text-caption text-muted-foreground"><p>{t($ => $.load_failed)}</p><Button variant="outline" size="sm" onClick={() => void q.refetch()}>{t($ => $.retry)}</Button></div> : q.isPending ? <p className="p-5 text-caption text-muted-foreground">{t($ => $.directory_loading)}</p> : selected ? <div className="flex min-h-0 flex-1 flex-col p-4"><ProjectDirectoryBrowser key={selected.id} projectId={selected.id} fullPage /></div> : <p className="p-5 text-caption text-muted-foreground">{t($ => $.no_projects_files)}</p>}
  </div>;
}

export function WorkResourcesPage({ legacyTab }: { legacyTab?: WorkResourceTab } = {}) {
  const { t } = useT("resources");
  const p = useWorkspacePaths();
  const nav = useNavigation();
  const tab = legacyTab ?? parseWorkResourceTab(nav.searchParams.get("tab"));
  return <div className="flex h-full min-h-0 flex-col">
    <header className="shrink-0 px-5 pt-4 pb-2"><h1 className="flex items-center gap-2 text-title font-semibold"><FileText className="size-5" />{t($ => $.work_resources)}</h1><p className="mt-1 text-caption text-muted-foreground">{t($ => $.work_resources_description)}</p></header>
    <ResourceTabs tab={tab} onChange={next => nav.replace(p.resources(next))} library={<ResourcesPage embedded />} files={<WorkspaceWorkingFiles />} git={<WorkspaceGitPage embedded />} />
  </div>;
}

function ProjectWorkingFiles({ projectId, canManage }: { projectId: string; canManage: boolean }) {
  return <div className="flex min-h-0 flex-1 flex-col">
    <div className="flex shrink-0 items-center justify-end border-b px-5 py-2"><OpenDirectoryTab projectId={projectId} /></div>
    <div className="shrink-0 max-h-[35vh] overflow-auto border-b px-5 py-3"><ProjectResourcesSection projectId={projectId} canManage={canManage} mode="directories" /></div>
    <div className="flex min-h-0 flex-1 flex-col p-4"><ProjectDirectoryBrowser key={projectId} projectId={projectId} fullPage /></div>
  </div>;
}

export function ProjectWorkResources({ projectId, canManage, tab }: { projectId: string; canManage: boolean; tab: WorkResourceTab }) {
  const p = useWorkspacePaths();
  const nav = useNavigation();
  return <ResourceTabs tab={tab} onChange={next => nav.replace(p.projectResources(projectId, next))} library={<ProjectFilesPage projectId={projectId} canManage={canManage} />} files={<ProjectWorkingFiles projectId={projectId} canManage={canManage} />} git={<ProjectGitTab projectId={projectId} canManage={canManage} />} />;
}
