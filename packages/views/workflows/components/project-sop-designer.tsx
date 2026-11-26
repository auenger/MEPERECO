"use client";

import { WorkflowMarkdownEditor } from "./workflow-markdown-editor";

import { WorkflowEditorNavigation, WorkflowEditorTabs } from "./workflow-editor-navigation";
import { workflowOperationDialogClassName } from "./workflow-dialog";
import { StageExecutionFields } from "./stage-execution-fields";
import { lazy, Suspense, useEffect, useMemo, useRef, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import {
  ArrowDown,
  ArrowLeft,
  ArrowUp,
  Archive,
  Bot,
  CalendarClock,
  Check,
  ChevronDown,
  ChevronRight,
  CircleDot,
  Clock3,
  Copy,
  FileInput,
  FileOutput,
  GitBranch,
  History,
  LayoutTemplate,
  LoaderCircle,
  Play,
  Plus,
  ShieldCheck,
  SlidersHorizontal,
  Sparkles,
  Trash2,
  UserCheck,
  UserRound,
  Wrench,
} from "lucide-react";
import { toast } from "sonner";
import { useWorkspaceId } from "@silieco/core/hooks";
import { workspaceMCPServersOptions } from "@silieco/core/mcp";
import { useWorkspacePaths } from "@silieco/core/paths";
import type {
  Workflow,
  WorkflowInstance,
} from "@silieco/core/types";
import {
  agentListOptions,
  memberListOptions,
  skillListOptions,
} from "@silieco/core/workspace/queries";
import {
  useCreateWorkflowInstance,
  useArchiveWorkflowInstance,
  useArchiveWorkflow,
  workflowInstancesOptions,
  workflowListOptions,
} from "@silieco/core/workflows";
import { Badge } from "@silieco/ui/components/ui/badge";
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from "@silieco/ui/components/ui/alert-dialog";
import { Button } from "@silieco/ui/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@silieco/ui/components/ui/dialog";
import { Input } from "@silieco/ui/components/ui/input";
import { Textarea } from "@silieco/ui/components/ui/textarea";
import { cn } from "@silieco/ui/lib/utils";
import { ReadonlyContent } from "../../editor";
import { useT } from "../../i18n";
import { useNavigation } from "../../navigation";
import { WorkflowRunPlanDialog } from "./workflow-run-plan-dialog";
import { StageMCPSelector } from "./stage-mcp-selector";

const WorkflowChatBridge = lazy(() => import("./workflow-chat-bridge"));

import { SOP_TEMPLATES, blankSopTemplate, cloneStages, objectString } from "./sop-designer-model";
import { ArtifactMarkdownEditor, StageSkillSelector } from "./sop-designer-fields";
import { SopCompletionFields, SopGateFields } from "./sop-policy-fields";
import { useSopDesigner } from "./use-sop-designer";
export { SOP_TEMPLATES } from "./sop-designer-model";

export function ProjectSopDesigner({
  projectId,
  workflowId,
}: {
  projectId?: string;
  workflowId?: string;
}) {
  const { t } = useT("workflows");
  const workspaceId = useWorkspaceId();
  const paths = useWorkspacePaths();
  const navigation = useNavigation();
  const { data: availableWorkflows = [], isLoading } = useQuery(
    workflowListOptions(workspaceId, projectId),
  );
  const workflows = useMemo(
    () =>
      projectId
        ? availableWorkflows
        : availableWorkflows.filter((workflow) => !workflow.project_id),
    [availableWorkflows, projectId],
  );
  const { data: runs = [] } = useQuery(
    workflowInstancesOptions(workspaceId, undefined, projectId),
  );
  const { data: workspaceSkills = [], isLoading: skillsLoading } = useQuery(
    skillListOptions(workspaceId),
  );
  const { data: workspaceMCP } = useQuery(workspaceMCPServersOptions(workspaceId));
  const { data: members = [] } = useQuery(memberListOptions(workspaceId));
  const { data: agents = [] } = useQuery(agentListOptions(workspaceId));
  const createInstance = useCreateWorkflowInstance();
  const archiveInstance = useArchiveWorkflowInstance();
  const deleteWorkflow = useArchiveWorkflow(true);
  const [workflowToDelete, setWorkflowToDelete] = useState<Workflow>();
  const { data: allRuns, isError: runsCheckFailed } = useQuery(
    workflowInstancesOptions(workspaceId),
  );
  const [startingWorkflowId, setStartingWorkflowId] = useState<string>();
  const [workflowToStart, setWorkflowToStart] = useState<{
    id: string;
    name: string;
  }>();
  const [runName, setRunName] = useState("");
  const [runInputsText, setRunInputsText] = useState("{}");
  const startingRun = useRef(false);
  const runInputs = useMemo(() => {
    try {
      const parsed: unknown = JSON.parse(runInputsText);
      return parsed !== null && typeof parsed === "object" && !Array.isArray(parsed) ? parsed as Record<string, unknown> : null;
    } catch { return null; }
  }, [runInputsText]);
  const [runToArchive, setRunToArchive] = useState<WorkflowInstance>();
  const [runToAdjust, setRunToAdjust] = useState<WorkflowInstance>();
  const [selectedWorkflowId, setSelectedWorkflowId] = useState<string>();
  const [workflowPreviewOpen, setWorkflowPreviewOpen] = useState(false);
  const [previewStageId, setPreviewStageId] = useState<string>();
  const { editing, setEditing, editingScope, sourceWorkflowName, templateName, templateSource, name, setName, description, setDescription, stages, setStages, assistantOpen, setAssistantOpen, assistantVisited, setAssistantVisited, editorTab, setEditorTab, expanded, setExpanded, invalidRollback, invalidGateCount, begin, customize, updateStage, moveStage, addStage, addCustomStatus, publish, createWorkflow } = useSopDesigner(projectId, members, agents, workspaceSkills);


  const selectedWorkflow = workflows.find(
    (workflow) => workflow.id === selectedWorkflowId,
  );
  const selectedHasRuns = allRuns?.some((run) => run.workflow_id === selectedWorkflow?.id) === true;
  const canManageSelected = selectedWorkflow && (projectId ? selectedWorkflow.project_id === projectId : !selectedWorkflow.project_id);
  const spaceWorkflows = workflows.filter((workflow) => !workflow.project_id);
  const projectWorkflows = workflows.filter(
    (workflow) => workflow.project_id === projectId,
  );
  const activeRuns = runs.filter((run) => !run.archived_at);
  const archivedRuns = runs.filter((run) => Boolean(run.archived_at));


  useEffect(() => {
    const preferred = workflowId ?? selectedWorkflowId;
    if (preferred && workflows.some((workflow) => workflow.id === preferred)) {
      if (preferred !== selectedWorkflowId) setSelectedWorkflowId(preferred);
      return;
    }
    setSelectedWorkflowId(workflows[0]?.id);
  }, [selectedWorkflowId, workflowId, workflows]);

  const prepareStartWorkflow = (workflowId: string, workflowName: string) => {
    if (!projectId) return;
    const sequence =
      runs.filter((run) => run.workflow_id === workflowId).length + 1;
    setRunInputsText("{}");
    setWorkflowToStart({ id: workflowId, name: workflowName });
    setRunName(
      t(($) => $.designer.run_default_name, {
        name: workflowName,
        sequence,
      }),
    );
  };
  const startWorkflow = () => {
    const title = runName.trim();
    if (!workflowToStart || !title || !projectId || !runInputs || startingRun.current || createInstance.isPending) return;
    startingRun.current = true;
    setStartingWorkflowId(workflowToStart.id);
    createInstance.mutate(
      {
        workflowId: workflowToStart.id,
        data: {
          title,
          project_id: projectId,
          inputs: runInputs,
          start: true,
        },
      },
      {
        onSuccess: () => {
          toast.success(t(($) => $.designer.run_started));
          setWorkflowToStart(undefined);
          setRunName("");
        },
        onError: (error) =>
          toast.error(
            error instanceof Error
              ? error.message
              : t(($) => $.designer.run_failed),
          ),
        onSettled: () => { startingRun.current = false; setStartingWorkflowId(undefined); },
      },
    );
  };

  const archiveRun = () => {
    if (!runToArchive) return;
    archiveInstance.mutate(runToArchive.id, {
      onSuccess: () => {
        toast.success(t(($) => $.designer.run_archived));
        setRunToArchive(undefined);
      },
      onError: (error) =>
        toast.error(
          error instanceof Error
            ? error.message
            : t(($) => $.designer.archive_failed),
        ),
    });
  };

  if (!editing) {
    return (
      <>
        <div className="min-h-0 flex-1 overflow-y-auto bg-background">
          <div className="mx-auto w-full max-w-5xl px-5 py-5">
          <header className="flex items-start justify-between gap-4">
            <div>
              <div className="flex items-center gap-2">
                <GitBranch className="size-4 text-brand" />
                <h2 className="text-title-sm font-semibold">{t(($) => $.designer.library_title)}</h2>
                <Badge variant="secondary">{workflows.length}</Badge>
              </div>
              <p className="mt-1 max-w-2xl text-body text-muted-foreground">
                {projectId
                  ? t(($) => $.designer.library_description)
                  : t(($) => $.page.description)}
              </p>
            </div>
            <Button size="sm" onClick={() => begin(blankSopTemplate())}>
              <Plus className="size-3.5" />
              {t(($) => $.page.create)}
            </Button>
          </header>

          {projectId && (
            <section className="mt-6 rounded-xl border border-border/80 bg-muted/10 p-3.5">
              <div className="flex items-center justify-between gap-3">
                <div className="flex items-center gap-2">
                  <Play className="size-4 text-brand" />
                  <h3 className="text-body font-semibold">
                    {t(($) => $.designer.project_runs)}
                  </h3>
                </div>
                <Badge variant="secondary">{activeRuns.length}</Badge>
              </div>
              <p className="mt-1 text-caption text-muted-foreground">
                {t(($) => $.designer.project_runs_hint)}
              </p>
              {activeRuns.length === 0 ? (
                <div className="mt-3 rounded-lg border border-dashed border-border px-3 py-5 text-center text-caption text-muted-foreground">
                  {t(($) => $.designer.no_project_runs)}
                </div>
              ) : (
                <div className="mt-3 grid gap-2 sm:grid-cols-2">
                  {activeRuns.map((run: WorkflowInstance) => {
                    const workflow = workflows.find(
                      (item) => item.id === run.workflow_id,
                    );
                    const runProjectId = run.project_id ?? projectId;
                    const canOpenRun =
                      Boolean(runProjectId) &&
                      (run.status === "active" || run.status === "waiting" || run.status === "completed");
                    const currentStageName =
                      run.current_stage_name ??
                      workflow?.current_version?.stages.find(
                        (stage) => stage.id === run.current_stage_id,
                      )?.name;
                    return (
                      <article
                        key={run.id}
                        className="flex min-h-20 items-start gap-3 rounded-lg border border-border/80 bg-background p-3"
                      >
                        <span className="flex size-8 shrink-0 items-center justify-center rounded-lg bg-brand/8 text-brand">
                          {run.status === "completed" ? (
                            <Check className="size-4" />
                          ) : (
                            <Clock3 className="size-4" />
                          )}
                        </span>
                        <div className="min-w-0 flex-1">
                          <div className="flex items-start justify-between gap-2">
                            <div className="min-w-0">
                              <p className="truncate text-body font-semibold">
                                {run.title}
                              </p>
                              <p className="truncate text-caption text-muted-foreground">
                                {workflow?.name ?? t(($) => $.designer.unknown_sop)}
                              </p>
                            </div>
                            <div className="flex shrink-0 items-center gap-1">
                              <Badge variant="outline">
                                {t(($) => $.designer.run_status[run.status])}
                              </Badge>
                              {run.can_edit && (
                                <Button
                                  type="button"
                                  variant="ghost"
                                  size="icon-sm"
                                  title={t(($) => $.designer.run_plan_action)}
                                  aria-label={t(($) => $.designer.run_plan_action)}
                                  onClick={() => setRunToAdjust(run)}
                                >
                                  <SlidersHorizontal className="size-3.5" />
                                </Button>
                              )}
                              <Button
                                type="button"
                                variant="ghost"
                                size="icon-xs"
                                disabled={run.task_count > 0}
                                title={
                                  run.task_count > 0
                                    ? t(($) => $.designer.archive_blocked, {
                                        count: run.task_count,
                                      })
                                    : t(($) => $.designer.archive_run)
                                }
                                aria-label={t(($) => $.designer.archive_run)}
                                onClick={() => setRunToArchive(run)}
                              >
                                <Archive className="size-3.5" />
                              </Button>
                            </div>
                          </div>
                          <p className="mt-1.5 flex items-center gap-1.5 text-caption text-muted-foreground">
                            <CircleDot className="size-3" />
                            {currentStageName ?? t(($) => $.designer.no_current_stage)}
                          </p>
                          <div className="mt-1 flex flex-wrap items-center gap-x-2 gap-y-1 text-micro text-muted-foreground">
                            <span>
                              {t(($) => $.designer.run_task_count, {
                                count: run.task_count,
                              })}
                            </span>
                            {run.stage_count > 0 && run.current_stage_index !== null && (
                              <span>
                                {t(($) => $.designer.run_plan_progress, {
                                  current: run.current_stage_index + 1,
                                  total: run.stage_count,
                                })}
                              </span>
                            )}
                            {run.source_version > 0 && (
                              <span>
                                {t(($) => $.designer.run_plan_source_version, {
                                  version: run.source_version,
                                })}
                              </span>
                            )}
                            {run.revision > 1 && (
                              <span>
                                {t(($) => $.designer.run_plan_revision, {
                                  revision: run.revision,
                                })}
                              </span>
                            )}
                          </div>
                          {canOpenRun && runProjectId && (
                            <Button
                              type="button"
                              variant="link"
                              size="sm"
                              className="mt-1 h-auto px-0 text-caption"
                              onClick={() =>
                                navigation.push(
                                  paths.projectWorkflowRun(runProjectId, run.id),
                                )
                              }
                            >
                              {t(($) => $.designer.open_run_board)}
                              <ChevronRight className="size-3.5" />
                            </Button>
                          )}
                        </div>
                      </article>
                    );
                  })}
                </div>
              )}
              {archivedRuns.length > 0 && (
                <details className="mt-3 rounded-lg border border-border/70 bg-background">
                  <summary className="flex cursor-pointer list-none items-center gap-2 px-3 py-2.5 text-caption font-medium text-muted-foreground hover:text-foreground">
                    <History className="size-3.5" />
                    <span>{t(($) => $.designer.archived_runs)}</span>
                    <Badge variant="secondary" className="ml-auto">
                      {archivedRuns.length}
                    </Badge>
                    <ChevronDown className="size-3.5" />
                  </summary>
                  <div className="grid gap-2 border-t border-border/70 p-2 sm:grid-cols-2">
                    {archivedRuns.map((run) => {
                      const workflow = workflows.find(
                        (item) => item.id === run.workflow_id,
                      );
                      const archivedBy = members.find(
                        (member) => member.user_id === run.archived_by,
                      );
                      return (
                        <article
                          key={run.id}
                          className="rounded-lg bg-muted/25 px-3 py-2.5"
                        >
                          <div className="flex items-start gap-2">
                            <Archive className="mt-0.5 size-3.5 shrink-0 text-muted-foreground" />
                            <div className="min-w-0 flex-1">
                              <p className="truncate text-caption font-semibold">
                                {run.title}
                              </p>
                              <p className="truncate text-micro text-muted-foreground">
                                {workflow?.name ??
                                  t(($) => $.designer.unknown_sop)}
                              </p>
                              <p className="mt-1 text-micro text-muted-foreground">
                                {t(($) => $.designer.archived_record, {
                                  name:
                                    archivedBy?.name ??
                                    t(($) => $.designer.unknown_member),
                                  time: run.archived_at
                                    ? new Date(run.archived_at).toLocaleString()
                                    : "—",
                                })}
                              </p>
                            </div>
                          </div>
                        </article>
                      );
                    })}
                  </div>
                </details>
              )}
            </section>
          )}

          <section className="mt-6">
            <div className="mb-2 flex items-center gap-2">
              <LayoutTemplate className="size-4 text-muted-foreground" />
              <h3 className="text-body font-semibold">{t(($) => $.designer.templates)}</h3>
            </div>
            <div className="grid gap-2 md:grid-cols-2">
              {SOP_TEMPLATES.map((template) => (
                <button
                  key={template.id}
                  type="button"
                  className="group flex min-h-24 items-start gap-3 rounded-xl border border-border/80 p-3 text-left hover:border-brand/35 hover:bg-brand/5"
                  onClick={() => begin(template)}
                >
                  <span className="flex size-9 shrink-0 items-center justify-center rounded-lg bg-muted text-muted-foreground group-hover:bg-brand/10 group-hover:text-brand">
                    {template.source === "blank" ? <Plus className="size-4" /> : <GitBranch className="size-4" />}
                  </span>
                  <span className="min-w-0 flex-1">
                    <span className="flex items-center gap-2">
                      <span className="text-body font-semibold">{template.name}</span>
                      <span className="text-micro text-muted-foreground">{t(($) => $.designer.stage_count, { count: template.stages.length })}</span>
                    </span>
                    <span className="mt-1 block text-caption leading-5 text-muted-foreground">{template.description}</span>
                  </span>
                </button>
              ))}
            </div>
          </section>

          <section className="mt-7 pb-8">
            <div className="mb-3 flex items-center justify-between gap-3">
              <div className="flex items-center gap-2">
                <GitBranch className="size-4 text-muted-foreground" />
                <h3 className="text-body font-semibold">
                  {projectId
                    ? t(($) => $.designer.existing)
                    : t(($) => $.designer.space_assets)}
                </h3>
              </div>
              <span className="text-caption text-muted-foreground">
                {projectId
                  ? t(($) => $.designer.project_available_hint)
                  : t(($) => $.designer.space_asset_hint)}
              </span>
            </div>
            {isLoading ? (
              <div className="grid gap-3 lg:grid-cols-[minmax(220px,0.8fr)_minmax(0,1.7fr)]">
                <div className="h-64 animate-pulse rounded-xl bg-muted/50" />
                <div className="h-64 animate-pulse rounded-xl bg-muted/50" />
              </div>
            ) : workflows.length === 0 ? (
              <div className="rounded-xl border border-dashed border-border px-4 py-7 text-center">
                <p className="text-body font-medium">{t(($) => $.page.empty_title)}</p>
                <p className="mt-1 text-caption text-muted-foreground">{t(($) => $.designer.empty_existing)}</p>
              </div>
            ) : (
              <div className="space-y-4">
                {[
                  { label: t(($) => $.designer.space_assets), items: spaceWorkflows },
                  { label: t(($) => $.designer.project_variants), items: projectId ? projectWorkflows : [] },
                ].filter((group) => group.items.length > 0).map((group) => (
                  <div key={group.label}>
                    <div className="mb-2 flex items-center gap-2 text-caption font-medium text-muted-foreground">
                      <span>{group.label}</span><Badge variant="secondary">{group.items.length}</Badge>
                    </div>
                    <div className="grid gap-2 sm:grid-cols-2">
                      {group.items.map((workflow) => (
                        <button key={workflow.id} type="button" aria-haspopup="dialog" onClick={() => { setSelectedWorkflowId(workflow.id); setPreviewStageId(undefined); setWorkflowPreviewOpen(true); }} className="flex min-h-24 min-w-0 items-start gap-3 rounded-lg border border-border/80 bg-background p-3 text-left transition-colors hover:border-brand/40 hover:bg-muted/20 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring">
                          <span className="flex size-8 shrink-0 items-center justify-center rounded-lg bg-brand/8 text-brand"><GitBranch className="size-4" /></span>
                          <span className="min-w-0 flex-1">
                            <span className="flex items-start justify-between gap-2">
                              <span className="min-w-0 truncate text-body font-semibold">{workflow.name}</span>
                              <ChevronRight className="mt-0.5 size-4 shrink-0 text-muted-foreground" />
                            </span>
                            <span className="mt-1 line-clamp-2 break-words text-caption text-muted-foreground">{workflow.description || t(($) => $.designer.no_description)}</span>
                            <span className="mt-2 flex flex-wrap items-center gap-2 text-micro text-muted-foreground">
                              <span>{t(($) => $.designer.stage_count, { count: workflow.current_version?.stages.length ?? 0 })}</span>
                              <Badge variant="outline">{t(($) => $.detail.version, { version: workflow.current_version?.version ?? 1 })}</Badge>
                            </span>
                          </span>
                        </button>
                      ))}
                    </div>
                  </div>
                ))}
              </div>
            )}
          </section>
          </div>
        </div>
        <Dialog open={workflowPreviewOpen && Boolean(selectedWorkflow)} onOpenChange={setWorkflowPreviewOpen}>
          <DialogContent className="flex h-[85dvh] max-h-[90dvh] flex-col gap-0 overflow-hidden p-0 sm:max-w-5xl">
            <DialogHeader className="shrink-0 border-b border-border/70 px-5 py-4 pr-12">
              <DialogTitle>{selectedWorkflow?.name}</DialogTitle>
              <DialogDescription>{t(($) => $.designer.preview_description)}</DialogDescription>
            </DialogHeader>
            <div className="min-h-0 flex-1 overflow-y-auto p-5">
                  {selectedWorkflow ? (
                    <div>
                      <header className="flex flex-wrap items-start justify-between gap-3">
                        <div className="min-w-0">
                          <div className="flex flex-wrap items-center gap-2">
                            <h3 className="text-title-sm font-semibold">{selectedWorkflow.name}</h3>
                            <Badge variant={selectedWorkflow.project_id ? "outline" : "secondary"}>
                              {selectedWorkflow.project_id
                                ? t(($) => $.designer.project_variant)
                                : t(($) => $.designer.space_asset)}
                            </Badge>
                            <Badge variant="outline">
                              {t(($) => $.detail.version, {
                                version: selectedWorkflow.current_version?.version ?? 1,
                              })}
                            </Badge>
                          </div>
                          <p className="mt-1 max-w-2xl text-body leading-6 text-muted-foreground">
                            {selectedWorkflow.description || t(($) => $.designer.no_description)}
                          </p>
                        </div>
                        <div className="flex flex-wrap items-center gap-2">
                          {canManageSelected && (
                            <Button
                              size="sm"
                              variant="outline"
                              className="text-destructive hover:text-destructive"
                              disabled={!allRuns || runsCheckFailed || selectedHasRuns || deleteWorkflow.isPending}
                              title={selectedHasRuns ? t(($) => $.designer.delete_sop_blocked) : t(($) => $.designer.delete_sop)}
                              onClick={() => setWorkflowToDelete(selectedWorkflow)}
                            >
                              <Trash2 className="size-3.5" />
                              {t(($) => $.designer.delete_sop)}
                            </Button>
                          )}
                          {projectId && (
                            <Button
                              size="sm"
                              variant="outline"
                              disabled={!selectedWorkflow.current_version?.stages.length}
                              onClick={() => { setWorkflowPreviewOpen(false); customize(selectedWorkflow); }}
                            >
                              <Copy className="size-3.5" />
                              {t(($) => $.designer.customize_for_project)}
                            </Button>
                          )}
                          {projectId && (
                            <Button
                              size="sm"
                              disabled={
                                selectedWorkflow.status !== "published" ||
                                createInstance.isPending
                              }
                              onClick={() =>
                                prepareStartWorkflow(
                                  selectedWorkflow.id,
                                  selectedWorkflow.name,
                                )
                              }
                            >
                              {startingWorkflowId === selectedWorkflow.id ? (
                                <LoaderCircle className="size-3.5 animate-spin" />
                              ) : (
                                <Play className="size-3.5" />
                              )}
                              {t(($) => $.designer.start_run)}
                            </Button>
                          )}
                        </div>
                      </header>

                      <div className="mt-4 flex flex-wrap gap-x-5 gap-y-2 rounded-lg bg-muted/30 px-3 py-2 text-caption text-muted-foreground">
                        <span>{t(($) => $.designer.stage_count, { count: selectedWorkflow.current_version?.stages.length ?? 0 })}</span>
                        <span>{t(($) => $.designer.active_runs, {
                          count: runs.filter(
                            (run) =>
                              !run.archived_at &&
                              run.workflow_id === selectedWorkflow.id &&
                              run.status !== "completed" &&
                              run.status !== "cancelled",
                          ).length,
                        })}</span>
                        <span>{t(($) => $.detail[selectedWorkflow.status as keyof typeof $.detail])}</span>
                      </div>

                      <div className="mt-5 border-t border-border/70 pt-4">
                        <p className="mb-3 text-caption text-muted-foreground">{t(($) => $.designer.preview_stage_hint)}</p>
                        <div className="flex flex-wrap gap-2" role="group" aria-label={t(($) => $.designer.stage_list, { count: selectedWorkflow.current_version?.stages.length ?? 0 })}>
                          {(selectedWorkflow.current_version?.stages ?? []).map((stage, index) => {
                            const active = stage.id === (previewStageId ?? selectedWorkflow.current_version?.stages[0]?.id);
                            return (
                              <button key={stage.id} type="button" aria-pressed={active} onClick={() => setPreviewStageId(stage.id)} className={cn("flex min-h-9 max-w-full items-center gap-2 rounded-lg border px-3 py-2 text-left text-caption transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring", active ? "border-brand/40 bg-brand/8 text-brand" : "border-border bg-background text-muted-foreground hover:bg-muted hover:text-foreground")}>
                                <span className="shrink-0 font-mono text-micro">{index + 1}</span>
                                <span className="min-w-0 break-words">{stage.name}</span>
                              </button>
                            );
                          })}
                        </div>
                      </div>
                      <div className="mt-5 space-y-2">
                        {(selectedWorkflow.current_version?.stages ?? []).map((stage, index) => {
                          if (stage.id !== (previewStageId ?? selectedWorkflow.current_version?.stages[0]?.id)) return null;
                          const completionRule = stage.completion_rule ?? {};
                          const gate = stage.gate ?? {};
                          const inputSpec = stage.input_spec ?? {};
                          const outputSpec = stage.output_spec ?? {};
                          const evaluationMode = completionRule.evaluation_mode;
                          const schedule = objectString(completionRule, "schedule");
                          const gateType = gate.type ?? "none";
                          const input = objectString(inputSpec, "default_artifact");
                          const output = objectString(outputSpec, "default_artifact");
                          const deciderType = objectString(gate, "decider_type");
                          const deciderId = objectString(gate, "decider");
                          const humanDeciderId = objectString(gate, "human_decider");
                          const deciderName =
                            deciderType === "human"
                              ? members.find((member) => member.user_id === deciderId)?.name
                              : deciderType === "agent"
                                ? agents.find((agent) => agent.id === deciderId)?.name
                                : deciderType === "self_agent"
                                  ? t(($) => $.designer.current_agent)
                                  : undefined;
                          const humanDeciderName = members.find(
                            (member) => member.user_id === humanDeciderId,
                          )?.name;
                          const skillNames = (stage.required_skills ?? []).map(
                            (ref) =>
                              workspaceSkills.find(
                                (skill) => skill.id === ref || skill.name === ref,
                              )?.name ?? ref,
                          );
                          return (
                            <section key={stage.id} className="rounded-lg border border-border/80 bg-muted/10 px-3 py-3">
                              <div className="flex items-start gap-3">
                                <span className="flex size-7 shrink-0 items-center justify-center rounded-md bg-brand text-caption font-semibold text-brand-foreground">
                                  {index + 1}
                                </span>
                                <div className="min-w-0 flex-1">
                                  <div className="flex flex-wrap items-center gap-2">
                                    <h4 className="text-body font-semibold">{stage.name}</h4>
                                    <Badge variant="outline">
                                      <CircleDot className="size-3" />
                                      {t(($) => $.designer.evaluation[evaluationMode === "manual" || evaluationMode === "scheduled" ? evaluationMode : "on_task_change"])}
                                    </Badge>
                                    {schedule && (
                                      <Badge variant="outline">
                                        <CalendarClock className="size-3" />
                                        {schedule}
                                      </Badge>
                                    )}
                                    <Badge variant="outline">
                                      <ShieldCheck className="size-3" />
                                      {t(($) => $.stage[`gate_${gateType}` as keyof typeof $.stage])}
                                    </Badge>
                                  </div>
                                  {stage.description && (
                                    <p className="mt-1 text-caption leading-5 text-muted-foreground">{stage.description}</p>
                                  )}
                                  <div className="mt-2 grid gap-2 text-caption sm:grid-cols-2">
                                    <div className="min-w-0 rounded-md bg-background px-2.5 py-2">
                                      <span className="text-micro font-medium text-muted-foreground">{t(($) => $.designer.default_input)}</span>
                                      {input ? (
                                        <ReadonlyContent content={input} className="mt-1 text-caption" />
                                      ) : (
                                        <p className="mt-0.5 font-medium">{t(($) => $.designer.not_configured)}</p>
                                      )}
                                    </div>
                                    <div className="min-w-0 rounded-md bg-background px-2.5 py-2">
                                      <span className="text-micro font-medium text-muted-foreground">{t(($) => $.designer.default_output)}</span>
                                      {output ? (
                                        <ReadonlyContent content={output} className="mt-1 text-caption" />
                                      ) : (
                                        <p className="mt-0.5 font-medium">{t(($) => $.designer.not_configured)}</p>
                                      )}
                                    </div>
                                  </div>
                                  <div className="mt-2 flex flex-wrap gap-1.5">
                                    {skillNames.length > 0 ? (
                                      skillNames.map((skill) => (
                                        <Badge key={skill} variant="secondary" className="gap-1">
                                          <Wrench className="size-3" />
                                          {skill}
                                        </Badge>
                                      ))
                                    ) : (
                                      <Badge variant="outline">
                                        {t(($) => $.designer.no_skills_selected)}
                                      </Badge>
                                    )}
                                    {gateType !== "none" && deciderName && (
                                      <Badge variant="outline" className="gap-1">
                                        <UserCheck className="size-3" />
                                        {deciderName}
                                      </Badge>
                                    )}
                                    {gate.require_human === true &&
                                      humanDeciderName &&
                                      humanDeciderName !== deciderName && (
                                        <Badge variant="outline" className="gap-1">
                                          <UserRound className="size-3" />
                                          {t(($) => $.designer.final_confirmation_by, {
                                            name: humanDeciderName,
                                          })}
                                        </Badge>
                                      )}
                                  </div>
                                </div>
                              </div>
                            </section>
                          );
                        })}
                      </div>
                    </div>
                  ) : null}
            </div>
            <DialogFooter className="m-0 shrink-0 border-t border-border/70 bg-muted/15 px-5 py-3">
              <Button variant="outline" onClick={() => setWorkflowPreviewOpen(false)}>{t(($) => $.designer.preview_close)}</Button>
            </DialogFooter>
          </DialogContent>
        </Dialog>
        <Dialog
          open={Boolean(workflowToStart)}
          onOpenChange={(open) => {
            if (open || createInstance.isPending) return;
            setWorkflowToStart(undefined);
            setRunName("");
          }}
        >
          <DialogContent className={workflowOperationDialogClassName}>
            <DialogHeader>
              <DialogTitle>{t(($) => $.designer.run_dialog_title)}</DialogTitle>
              <DialogDescription>
                {t(($) => $.designer.run_dialog_description, {
                  name: workflowToStart?.name ?? "",
                })}
              </DialogDescription>
            </DialogHeader>
            <label className="space-y-1.5 text-caption font-medium">
              <span>{t(($) => $.designer.run_name)}</span>
              <Input
                autoFocus
                value={runName}
                disabled={createInstance.isPending}
                placeholder={t(($) => $.designer.run_name_placeholder)}
                onChange={(event) => setRunName(event.target.value)}
                onKeyDown={(event) => {
                  if (event.key === "Enter") {
                    event.preventDefault();
                    startWorkflow();
                  }
                }}
              />
            </label>
            <label className="space-y-1.5 text-caption font-medium">
              <span>{t(($) => $.execution_config.run_inputs)}</span>
              <Textarea className="min-h-32 font-mono" value={runInputsText} disabled={createInstance.isPending} onChange={(event) => setRunInputsText(event.target.value)} />
              <span className="block text-micro font-normal text-muted-foreground">{t(($) => $.execution_config.run_inputs_hint)}</span>
            </label>
            {!runInputs && <p role="alert" className="text-caption text-destructive">{t(($) => $.input_recovery.invalid)}</p>}
            <DialogFooter className="border-t pt-4">
              <Button
                variant="outline"
                disabled={createInstance.isPending}
                onClick={() => {
                  setWorkflowToStart(undefined);
                  setRunName("");
                }}
              >
                {t(($) => $.designer.cancel_run)}
              </Button>
              <Button
                disabled={createInstance.isPending || !runName.trim() || !runInputs}
                onClick={startWorkflow}
              >
                {createInstance.isPending ? (
                  <LoaderCircle className="size-3.5 animate-spin" />
                ) : (
                  <Play className="size-3.5" />
                )}
                {createInstance.isPending
                  ? t(($) => $.designer.starting_run)
                  : t(($) => $.designer.confirm_start)}
              </Button>
            </DialogFooter>
          </DialogContent>
        </Dialog>
        {runToAdjust && (
          <WorkflowRunPlanDialog
            instanceId={runToAdjust.id}
            sourceName={
              workflows.find((workflow) => workflow.id === runToAdjust.workflow_id)?.name ??
              t(($) => $.designer.unknown_sop)
            }
            open
            onOpenChange={(open) => {
              if (!open) setRunToAdjust(undefined);
            }}
          />
        )}
        <AlertDialog open={Boolean(workflowToDelete)} onOpenChange={(open) => { if (!open && !deleteWorkflow.isPending) setWorkflowToDelete(undefined); }}>
          <AlertDialogContent>
            <AlertDialogHeader>
              <AlertDialogTitle>{t(($) => $.designer.delete_sop)}</AlertDialogTitle>
              <AlertDialogDescription>{t(($) => $.designer.delete_sop_description, { name: workflowToDelete?.name ?? "" })}</AlertDialogDescription>
            </AlertDialogHeader>
            <AlertDialogFooter>
              <AlertDialogCancel disabled={deleteWorkflow.isPending}>{t(($) => $.designer.archive_cancel)}</AlertDialogCancel>
              <AlertDialogAction className="bg-destructive text-destructive-foreground hover:bg-destructive/90" disabled={deleteWorkflow.isPending} onClick={(event) => {
                event.preventDefault();
                if (!workflowToDelete || deleteWorkflow.isPending) return;
                deleteWorkflow.mutate(workflowToDelete.id, {
                  onSuccess: () => {
                    setWorkflowToDelete(undefined);
                    setWorkflowPreviewOpen(false);
                    setSelectedWorkflowId(undefined);
                    toast.success(t(($) => $.designer.delete_sop_success));
                  },
                  onError: () => toast.error(t(($) => $.designer.delete_sop_failed)),
                });
              }}>
                {deleteWorkflow.isPending && <LoaderCircle className="size-3.5 animate-spin" />}
                {t(($) => $.designer.delete_sop_confirm)}
              </AlertDialogAction>
            </AlertDialogFooter>
          </AlertDialogContent>
        </AlertDialog>

        <AlertDialog
          open={Boolean(runToArchive)}
          onOpenChange={(open) => {
            if (!open && !archiveInstance.isPending) setRunToArchive(undefined);
          }}
        >
          <AlertDialogContent>
            <AlertDialogHeader>
              <AlertDialogTitle>
                {t(($) => $.designer.archive_dialog_title)}
              </AlertDialogTitle>
              <AlertDialogDescription>
                {t(($) => $.designer.archive_dialog_description, {
                  name: runToArchive?.title ?? "",
                })}
              </AlertDialogDescription>
            </AlertDialogHeader>
            <AlertDialogFooter>
              <AlertDialogCancel disabled={archiveInstance.isPending}>
                {t(($) => $.designer.archive_cancel)}
              </AlertDialogCancel>
              <AlertDialogAction
                disabled={archiveInstance.isPending}
                onClick={archiveRun}
              >
                {archiveInstance.isPending && (
                  <LoaderCircle className="size-3.5 animate-spin" />
                )}
                {t(($) => $.designer.archive_confirm)}
              </AlertDialogAction>
            </AlertDialogFooter>
          </AlertDialogContent>
        </AlertDialog>
      </>
    );
  }

  return (
    <div className="flex min-h-0 flex-1 flex-col bg-background">
      <header className="flex min-h-14 items-center gap-3 border-b border-border/70 px-5 py-2">
        <Button variant="ghost" size="icon-sm" aria-label={t(($) => $.create.cancel)} onClick={() => setEditing(false)}><ArrowLeft /></Button>
        <div className="min-w-0 flex-1">
          <div className="flex items-center gap-2">
            <h2 className="truncate text-body font-semibold">{t(($) => $.designer.editor_title)}</h2>
            <Badge variant="secondary" className="hidden sm:inline-flex">{templateName}</Badge>
          </div>
          <p className="hidden text-caption text-muted-foreground md:block">{t(($) => $.designer.editor_hint)}</p>
        </div>
        <Button type="button" variant="outline" size="sm" aria-label={t(($) => $.editor.assistant)} onClick={() => { setAssistantVisited(true); setAssistantOpen((open) => !open); }}><Bot className="size-3.5" /><span className="hidden sm:inline">{t(($) => $.editor.assistant)}</span></Button>
        <Button
          size="sm"
          disabled={
            createWorkflow.isPending ||
            !name.trim() ||
            !stages.some((item) => item.name.trim()) ||
            stages.some((item) => item.name.trim() && item.executionMode === "agent" && !agents.some((agent) => agent.id === item.executorId && !agent.archived_at)) ||
            invalidRollback || invalidGateCount > 0
          }
          onClick={publish}
        >
          <Sparkles className="size-3.5" />
          {createWorkflow.isPending ? t(($) => $.designer.publishing) : t(($) => $.designer.publish)}
        </Button>
      </header>

      <div className="relative flex min-h-0 flex-1 flex-col md:flex-row">
        <WorkflowEditorNavigation stages={stages} selected={[...expanded][0] ?? "overview"}
          onSelect={(id) => { setExpanded(id === "overview" ? new Set() : new Set([id])); setEditorTab("content"); }} onAdd={addStage} disabled={createWorkflow.isPending} />
        <div className="min-h-0 min-w-0 flex-1 overflow-y-auto">
        <div className="mx-auto w-full max-w-3xl px-5 py-5 md:px-8 md:py-6">
          <div className={expanded.size === 0 ? "space-y-5" : "hidden"}>
            <label className="block space-y-1.5 text-caption font-medium">
              <span>{t(($) => $.create.name)}</span>
              <Input value={name} onChange={(event) => setName(event.target.value)} placeholder={t(($) => $.create.name_placeholder)} autoFocus />
            </label>
            <WorkflowMarkdownEditor label={t(($) => $.create.description)} value={description} onChange={setDescription} placeholder={t(($) => $.create.description_placeholder)} />
            <div className="grid gap-3 sm:grid-cols-2">
              <div className="rounded-lg border border-border/80 bg-muted/20 px-3 py-2">
                <span className="block text-micro font-medium text-muted-foreground">{t(($) => $.designer.source)}</span>
                <span className="mt-0.5 block text-caption font-medium">
                  {sourceWorkflowName
                    ? t(($) => $.designer.source_inherited, {
                        name: sourceWorkflowName,
                      })
                    : templateSource === "preset"
                      ? t(($) => $.designer.source_preset)
                      : t(($) => $.designer.source_user)}
                </span>
              </div>
              <div className="rounded-lg border border-border/80 bg-muted/20 px-3 py-2">
                <span className="block text-micro font-medium text-muted-foreground">{t(($) => $.designer.scope)}</span>
                <span className="mt-0.5 block text-caption font-medium">
                  {editingScope === "project"
                    ? t(($) => $.designer.scope_project)
                    : t(($) => $.designer.scope_space)}
                </span>
              </div>
            </div>
          </div>

          <div className={expanded.size === 0 ? "mb-4 mt-6" : "hidden"}>
            <h3 className="text-body font-semibold">{t(($) => $.designer.stage_list, { count: stages.length })}</h3>
            <p className="text-caption text-muted-foreground">{t(($) => $.designer.stage_list_hint)}</p>
            {invalidGateCount > 0 && (
              <p className="mt-2 flex items-center gap-1.5 rounded-lg border border-border bg-muted px-3 py-2 text-caption text-muted-foreground">
                <ShieldCheck className="size-3.5 shrink-0" />
                {t(($) => $.designer.incomplete_gates, {
                  count: invalidGateCount,
                })}
              </p>
            )}
          </div>
          <div className="space-y-2.5">
            {stages.map((item, index) => {
              const open = expanded.has(item.id);
              if (!open) return null;
              return (
                <section key={item.id} className="min-w-0 space-y-4">
                  <div className="flex min-h-11 items-center gap-2">
                    <button
                      type="button"
                      className="flex size-6 shrink-0 items-center justify-center rounded-md bg-muted text-micro font-semibold"
                      aria-label={t(($) => $.editor.content)}
                    >{index + 1}</button>
                    <Input
                      value={item.name}
                      onChange={(event) => updateStage(item.id, { name: event.target.value })}
                      placeholder={t(($) => $.create.stage_placeholder)}
                      className="h-8 border-transparent bg-transparent px-1 text-body font-semibold shadow-none focus-visible:border-input focus-visible:bg-background"
                    />
                    {item.gateEnabled && <ShieldCheck className="size-3.5 shrink-0 text-muted-foreground" />}
                    <Button type="button" variant="ghost" size="icon-xs" disabled={index === 0} title={t(($) => $.designer.move_up)} onClick={() => moveStage(index, -1)}><ArrowUp /></Button>
                    <Button type="button" variant="ghost" size="icon-xs" disabled={index === stages.length - 1} title={t(($) => $.designer.move_down)} onClick={() => moveStage(index, 1)}><ArrowDown /></Button>
                    <Button type="button" variant="ghost" size="icon-xs" disabled={stages.length === 1} title={t(($) => $.designer.delete_stage)} className="text-muted-foreground hover:text-destructive" onClick={() => { setStages((current) => current.filter((value) => value.id !== item.id)); setExpanded(new Set()); }}><Trash2 /></Button>
                  </div>

                  {open && (
                    <div className="space-y-5">
                      <WorkflowEditorTabs value={editorTab} onChange={setEditorTab} />
                      <div className={editorTab === "execution" ? "space-y-4" : "hidden"}>
                      <StageExecutionFields value={item} rollbackStages={stages.slice(0, index).filter((stage) => stage.stableKey).map((stage) => ({stableKey: stage.stableKey!, name: stage.name}))} precedingStages={stages.slice(0, index).filter((stage) => stage.executionMode === "agent" && stage.stableKey).map((stage) => ({stableKey: stage.stableKey!, name: stage.name}))} agents={agents} disabled={createWorkflow.isPending} onChange={(patch) => updateStage(item.id, patch)} />
                      </div>
                      <div className={editorTab === "content" ? "space-y-5" : "hidden"}>
                        <WorkflowMarkdownEditor label={t(($) => $.designer.stage_description)} value={item.description} onChange={(description) => updateStage(item.id, { description })} placeholder={t(($) => $.designer.stage_description_placeholder)} />
                      </div>
                      <div className={editorTab === "execution" ? "space-y-4" : "hidden"}>

                      <SopCompletionFields item={item} updateStage={updateStage} addCustomStatus={addCustomStatus} />
                      </div>
                      <div className={editorTab === "content" ? "grid gap-5 lg:grid-cols-2" : "hidden"}>
                        <ArtifactMarkdownEditor
                          editorKey={`${item.id}-input`}
                          icon={<FileInput className="size-3.5 text-muted-foreground" />}
                          label={t(($) => $.designer.default_input)}
                          helper={t(($) => $.designer.input_markdown_hint)}
                          placeholder={t(($) => $.designer.input_placeholder)}
                          value={item.input}
                          onChange={(input) => updateStage(item.id, { input })}
                        />
                        <ArtifactMarkdownEditor
                          editorKey={`${item.id}-output`}
                          icon={<FileOutput className="size-3.5 text-muted-foreground" />}
                          label={t(($) => $.designer.default_output)}
                          helper={t(($) => $.designer.output_markdown_hint)}
                          placeholder={t(($) => $.designer.output_placeholder)}
                          value={item.output}
                          onChange={(output) => updateStage(item.id, { output })}
                        />
                      </div>

                      <div className={editorTab === "execution" ? "space-y-5" : "hidden"}>
                      <StageSkillSelector
                        skills={workspaceSkills}
                        selectedRefs={item.skills}
                        loading={skillsLoading}
                        onChange={(skills) => updateStage(item.id, { skills })}
                      />

                      <StageMCPSelector
                        servers={workspaceMCP?.servers ?? []}
                        selectedIds={item.allowedMcpServerIds}
                        onChange={(allowedMcpServerIds) => updateStage(item.id, { allowedMcpServerIds })}
                      />

                      </div>
                      <div className={editorTab === "review" ? "space-y-4" : "hidden"}>
                      <SopGateFields item={item} members={members} agents={agents} updateStage={updateStage} />
                      </div>
                    </div>
                  )}
                </section>
              );
            })}
          </div>
          <button type="button" className="mt-3 flex h-10 w-full items-center justify-center gap-2 rounded-xl border border-dashed border-border text-caption font-medium text-brand hover:border-brand/40 hover:bg-brand/5" onClick={addStage}>
            <Plus className="size-3.5" />
            {t(($) => $.create.add_stage)}
          </button>
        </div>
        </div>
        {assistantVisited && <div>
          <Suspense fallback={<div className="p-4 text-caption text-muted-foreground">{t(($) => $.editor.thinking)}</div>}>
            <WorkflowChatBridge open={assistantOpen} projectId={projectId} selection={[...expanded].join(", ") || "overview"} revision={JSON.stringify({ name, description, stages })} onRestore={() => { setName(name); setDescription(description); setStages(stages); }}
              draft={{ name, description, stages: stages.map(({ id, name, description, input, output }) => ({ id, name, description, input, output })) }}
              onApply={(draft) => {
                const changed = stages.find((stage) => {
                  const patch = draft.stages.find((item) => item.id === stage.id);
                  return patch && ["name", "description", "input", "output"].some((field) => patch[field as keyof typeof patch] !== stage[field as keyof typeof stage]);
                });
                if (changed) { setExpanded(new Set([changed.id])); setEditorTab("content"); }
                setName(draft.name); setDescription(draft.description);
                setStages((current) => {
                  const updated = current.map((stage) => {
                    const patch = draft.stages.find((item) => item.id === stage.id);
                    return patch ? { ...stage, name: patch.name, description: patch.description, input: patch.input, output: patch.output } : stage;
                  });
                  for (const proposed of draft.stages.filter((item) => !item.id)) {
                    const stage = cloneStages(blankSopTemplate())[0]!;
                    let number = 1;
                    while (updated.some((item) => item.stableKey === `stage_${number}`)) number++;
                    updated.push({ ...stage, ...proposed, id: stage.id, stableKey: `stage_${number}` });
                  }
                  return updated;
                });
              }} />
          </Suspense>
        </div>}
      </div>
    </div>
  );
}
