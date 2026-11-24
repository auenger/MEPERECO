"use client";

import { FileText, FolderKanban, FolderOpen, GitBranch, Plus, Route, SquarePen } from "lucide-react";
import type { Project, WorkflowInstance } from "@silieco/core/types";
import { useWorkspacePaths } from "@silieco/core/paths";
import { useModalStore } from "@silieco/core/modals";
import { cn } from "@silieco/ui/lib/utils";
import { SidebarMenu, SidebarMenuButton, SidebarMenuItem } from "@silieco/ui/components/ui/sidebar";
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuSeparator, DropdownMenuTrigger } from "@silieco/ui/components/ui/dropdown-menu";
import { AppLink, useNavigation } from "../navigation";
import { useT } from "../i18n";
import { ProjectIcon } from "../projects/components/project-icon";

const PROJECT_COLORS = [
  "bg-brand/10 text-brand",
  "bg-success/10 text-success",
  "bg-warning/10 text-warning",
  "bg-muted text-muted-foreground",
] as const;

function ProjectMark({ project }: { project: Project }) {
  if (project.icon) return <ProjectIcon project={project} size="md" />;
  const hash = Array.from(project.id).reduce((value, char) => (value * 31 + char.charCodeAt(0)) >>> 0, 0);
  return (
    <span aria-hidden="true" className={cn("flex size-6 shrink-0 items-center justify-center rounded-md text-caption font-semibold", PROJECT_COLORS[hash % PROJECT_COLORS.length])}>
      {Array.from(project.title.trim())[0]?.toUpperCase() || "P"}
    </span>
  );
}

function ProjectMenu({ project, runs }: { project: Project; runs: WorkflowInstance[] }) {
  const { t } = useT("layout");
  const p = useWorkspacePaths();
  const { pathname, searchParams } = useNavigation();
  const href = p.projectDetail(project.id);
  const active = pathname === href || pathname.startsWith(`${href}/`);
  const section = searchParams.get("section");
  const runId = searchParams.get("run");
  const filesActive = pathname === p.projectFiles(project.id);
  const links = [
    { label: t(($) => $.tab.task), href, icon: SquarePen, active: active && !filesActive && !runId && section !== "resources" && section !== "sop" },
    { label: t(($) => $.nav.resources), href: p.projectResources(project.id), icon: FileText, active: active && !filesActive && !runId && section === "resources" },
    { label: t(($) => $.sidebar.project_files), href: p.projectFiles(project.id), icon: FolderOpen, active: filesActive },
    { label: "SOP", href: p.projectSop(project.id), icon: GitBranch, active: active && !filesActive && !runId && section === "sop" },
  ];

  return (
    <SidebarMenuItem>
      <DropdownMenu>
        <SidebarMenuButton tooltip={project.title} aria-label={project.title} isActive={active} render={<DropdownMenuTrigger />} className="!p-1 data-active:ring-1 data-active:ring-brand/40">
          <ProjectMark project={project} />
        </SidebarMenuButton>
        <DropdownMenuContent side="right" align="start" sideOffset={12} className="w-64 max-w-[calc(100vw-5rem)]">
          <div className="flex items-center gap-2 px-2 py-2">
            <ProjectMark project={project} />
            <span className="min-w-0 break-words text-body font-semibold">{project.title}</span>
          </div>
          <DropdownMenuSeparator />
          {links.map((item) => (
            <DropdownMenuItem key={item.href} render={<AppLink href={item.href} aria-current={item.active ? "page" : undefined} />} className={cn("min-h-9", item.active && "bg-accent text-accent-foreground font-medium")}>
              <item.icon className="size-4" />{item.label}
            </DropdownMenuItem>
          ))}
          <DropdownMenuSeparator />
          <div className="flex items-center justify-between px-2 py-1.5 text-caption text-muted-foreground">
            <span>{t(($) => $.sidebar.project_runs)}</span><span>{runs.length}</span>
          </div>
          {runs.length > 0 ? (
            <div className="max-h-48 overflow-y-auto">
              {runs.map((run) => (
                <DropdownMenuItem key={run.id} render={<AppLink href={p.projectWorkflowRun(project.id, run.id)} aria-current={active && runId === run.id ? "page" : undefined} />} className={cn("min-h-9", active && runId === run.id && "bg-accent text-accent-foreground font-medium")}>
                  <Route className="size-4 text-brand" /><span className="min-w-0 truncate" title={run.title}>{run.title}</span>
                </DropdownMenuItem>
              ))}
            </div>
          ) : <p className="px-2 pb-2 text-caption text-muted-foreground">{t(($) => $.sidebar.no_project_runs)}</p>}
        </DropdownMenuContent>
      </DropdownMenu>
    </SidebarMenuItem>
  );
}

export function CollapsedProjects({ projects, runsByProject }: {
  projects: Project[];
  runsByProject: Map<string, WorkflowInstance[]>;
}) {
  const { t } = useT("layout");
  const p = useWorkspacePaths();
  const { pathname } = useNavigation();

  return (
    <>
      <div className="w-6 border-t border-sidebar-border" />
      <SidebarMenu className="items-center gap-1" aria-label={t(($) => $.nav.projects)}>
        <SidebarMenuItem>
          <SidebarMenuButton tooltip={t(($) => $.sidebar.view_projects)} aria-label={t(($) => $.sidebar.view_projects)} isActive={pathname === p.projects()} render={<AppLink href={p.projects()} />}>
            <FolderKanban />
          </SidebarMenuButton>
        </SidebarMenuItem>
        <SidebarMenuItem>
          <SidebarMenuButton tooltip={t(($) => $.sidebar.create_project)} aria-label={t(($) => $.sidebar.create_project)} onClick={() => useModalStore.getState().open("create-project")}>
            <Plus />
          </SidebarMenuButton>
        </SidebarMenuItem>
        {projects.map((project) => <ProjectMenu key={project.id} project={project} runs={runsByProject.get(project.id) ?? []} />)}
      </SidebarMenu>
      <div className="w-6 border-t border-sidebar-border" />
    </>
  );
}
