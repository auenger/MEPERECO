"use client";
import { useState, type KeyboardEvent } from "react";
import { toast } from "sonner";
import type { Agent, MemberWithUser, SkillSummary, Workflow } from "@silieco/core/types";
import { useCreateWorkflow } from "@silieco/core/workflows";
import { useT } from "../../i18n";
import { validReworkPolicyDraft, validRetryPolicyDraft } from "./stage-execution-fields";
import type { WorkflowEditorTab } from "./workflow-editor-navigation";
import { blankSopTemplate, cloneStages, resolveStageActors, gateIsConfigured, stagesFromWorkflow, serializeSopStages, type SopStageDraft, type SopTemplate } from "./sop-designer-model";

export function useSopDesigner(projectId: string | undefined, members: readonly MemberWithUser[], agents: readonly Agent[], workspaceSkills: readonly SkillSummary[]) {
 const { t } = useT("workflows");
 const createWorkflow = useCreateWorkflow();
  const [editing, setEditing] = useState(false);
  const [editingScope, setEditingScope] = useState<"space" | "project">("space");
  const [sourceWorkflowName, setSourceWorkflowName] = useState<string>();
  const [templateName, setTemplateName] = useState("");
  const [templateSource, setTemplateSource] = useState<SopTemplate["source"]>("blank");
  const [name, setName] = useState("");
  const [description, setDescription] = useState("");
  const [stages, setStages] = useState<SopStageDraft[]>([]);
  const [assistantOpen, setAssistantOpen] = useState(false);
  const [assistantVisited, setAssistantVisited] = useState(false);
  const [editorTab, setEditorTab] = useState<WorkflowEditorTab>("content");
  const [expanded, setExpanded] = useState<Set<string>>(() => new Set());
  const invalidRollback = stages.some((stage) => !validReworkPolicyDraft(stage.reworkPolicy) || !validRetryPolicyDraft(stage.retryPolicy)) || stages.some((item, index) => item.rollbackStageKey && !stages.slice(0, index).some((previous) => previous.stableKey === item.rollbackStageKey));
  const invalidGateCount = stages.filter(
    (stage) => !gateIsConfigured(stage, members, agents),
  ).length;
  const begin = (
    template: SopTemplate,
    scope: "space" | "project" = "space",
  ) => {
    const nextStages = resolveStageActors(
      cloneStages(template),
      members,
      agents,
    );
    setAssistantOpen(false);
    setAssistantVisited(false);
    setTemplateName(template.name);
    setTemplateSource(template.source);
    setName(template.source === "blank" ? "" : template.name);
    setDescription(template.source === "blank" ? "" : template.description);
    setEditingScope(scope);
    setSourceWorkflowName(undefined);
    setStages(nextStages);
    setExpanded(new Set());
    setEditing(true);
  };
  const customize = (workflow: Workflow) => {
    const nextStages = resolveStageActors(
      stagesFromWorkflow(workflow),
      members,
      agents,
    );
    if (!projectId || nextStages.length === 0) return;
    setAssistantOpen(false);
    setAssistantVisited(false);
    setTemplateName(workflow.name);
    setTemplateSource("blank");
    setName(`${workflow.name} Project 版`);
    setDescription(workflow.description ?? "");
    setEditingScope("project");
    setSourceWorkflowName(workflow.name);
    setStages(nextStages);
    setExpanded(new Set());
    setEditing(true);
  };
  const updateStage = (id: string, patch: Partial<Omit<SopStageDraft, "id">>) =>
    setStages((current) => current.map((item) => item.id === id ? { ...item, ...patch } : item));
  const moveStage = (index: number, delta: -1 | 1) =>
    setStages((current) => {
      const target = index + delta;
      if (target < 0 || target >= current.length) return current;
      const next = [...current];
      [next[index], next[target]] = [next[target]!, next[index]!];
      return next;
    });
  const addStage = () => {
    const item = cloneStages(blankSopTemplate())[0]!;
    let number = 1;
    while (stages.some((candidate) => candidate.stableKey === `stage_${number}`)) number++;
    item.stableKey = `stage_${number}`;
    setStages((current) => [...current, item]);
    setExpanded(new Set([item.id]));
    setEditorTab("content");
  };
  const addCustomStatus = (event: KeyboardEvent<HTMLInputElement>, item: SopStageDraft) => {
    if (event.key !== "Enter") return;
    event.preventDefault();
    const value = event.currentTarget.value.trim();
    if (!value || item.statuses.includes(value)) return;
    updateStage(item.id, { statuses: [...item.statuses, value] });
    event.currentTarget.value = "";
  };
  const publish = () => {
    const validStages = stages.filter((item) => item.name.trim());
    if (!name.trim() || validStages.length === 0 || invalidRollback || invalidGateCount > 0 || validStages.some((stage) => stage.executionMode === "agent" && !agents.some((agent) => agent.id === stage.executorId && !agent.archived_at))) return;
    createWorkflow.mutate(
      {
        ...(editingScope === "project" && projectId
          ? { project_id: projectId }
          : {}),
        name: name.trim(),
        description: description.trim() || null,
        publish: true,
        stages: serializeSopStages(validStages, workspaceSkills),
      },
      {
        onSuccess: () => {
          toast.success(t(($) => $.designer.created));
          setEditing(false);
          setSourceWorkflowName(undefined);
        },
        onError: (error) => toast.error(error instanceof Error ? error.message : t(($) => $.designer.create_failed)),
      },
    );
  };

 return { editing, setEditing, editingScope, sourceWorkflowName, templateName, templateSource, name, setName, description, setDescription, stages, setStages, assistantOpen, setAssistantOpen, assistantVisited, setAssistantVisited, editorTab, setEditorTab, expanded, setExpanded, invalidRollback, invalidGateCount, begin, customize, updateStage, moveStage, addStage, addCustomStatus, publish, createWorkflow };
}
