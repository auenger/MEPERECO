"use client";
import type { KeyboardEvent } from "react";
import { useConfigStore } from "@silieco/core/config";
import type { MemberWithUser, Agent } from "@silieco/core/types";
import { Bot, Check, ShieldCheck, UserCheck, UserRound } from "lucide-react";
import { Input } from "@silieco/ui/components/ui/input";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@silieco/ui/components/ui/select";
import { Switch } from "@silieco/ui/components/ui/switch";
import { cn } from "@silieco/ui/lib/utils";
import { useT } from "../../i18n";
import { GateActorPicker } from "./sop-designer-fields";
import { ALL_STATUSES, gateIsConfigured, type SopStageDraft, type EvaluationMode, type CanonicalStageStatus, type DeciderType } from "./sop-designer-model";

type UpdateStage = (id: string, patch: Partial<Omit<SopStageDraft, "id">>) => void;

export function SopCompletionFields({item, updateStage, addCustomStatus}: {item: SopStageDraft; updateStage: UpdateStage; addCustomStatus: (event: KeyboardEvent<HTMLInputElement>, item: SopStageDraft) => void}) {
  const { t } = useT("workflows");
  const catalog = useConfigStore(state => state.completionPolicy);
  const supportedModes = (["on_task_change", "manual", "scheduled"] as EvaluationMode[]).filter(mode => !catalog || catalog.manual_evaluations.includes(mode));
  if (item.executionMode === "agent") return null;
  return (<>

    <div className="grid gap-3 sm:grid-cols-2">
      <label className="space-y-1.5 text-caption font-medium">
        <span>{t(($) => $.designer.evaluation_label)}</span>
        <Select
          items={supportedModes.map((mode) => ({
            value: mode,
            label: t(($) => $.designer.evaluation[mode]),
          }))}
          value={item.evaluationMode}
          onValueChange={(value) => {
            if (value) {
              updateStage(item.id, {
                evaluationMode: value as EvaluationMode,
              });
            }
          }}
        >
          <SelectTrigger>
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            {supportedModes.map((mode) => (
              <SelectItem key={mode} value={mode}>
                {t(($) => $.designer.evaluation[mode])}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
        <span className="block text-micro font-normal leading-4 text-muted-foreground">
          {t(($) => $.designer.evaluation_hint[item.evaluationMode])}
        </span>
        {item.evaluationMode === "scheduled" && (
          <span className="block space-y-1 pt-1">
            <span className="block text-micro font-medium text-muted-foreground">
              {t(($) => $.designer.schedule_label)}
            </span>
            <Input
              value={item.schedule}
              placeholder={t(($) => $.designer.schedule_placeholder)}
              onChange={(event) =>
                updateStage(item.id, {
                  schedule: event.target.value,
                })
              }
            />
            <span className="block text-micro font-medium text-muted-foreground">{t(($) => $.designer.schedule_timezone)}</span>
            <Input value={typeof item.completionRule?.timezone === "string" ? item.completionRule.timezone : ""} placeholder="UTC" onChange={(event) => updateStage(item.id, { completionRule: { ...item.completionRule, timezone: event.target.value.trim() } })} />
          </span>
        )}
      </label>
      <div className="rounded-lg border border-border/80 bg-muted/20 px-3 py-2.5">
        <span className="block text-caption font-medium">{t(($) => $.designer.completion_rule)}</span>
        <span className="mt-1 block text-caption leading-5 text-muted-foreground">
          {t(($) => $.designer.completion_all_tasks)}
        </span>
      </div>
    </div>

    <div>
      <label className="mb-1.5 block text-caption font-medium">{t(($) => $.designer.statuses)}</label>
      <div className="flex flex-wrap gap-1.5">
        {ALL_STATUSES.map((status) => {
          const active = item.statuses.includes(status);
          return (
            <button
              key={status}
              type="button"
              className={cn("flex h-7 items-center gap-1.5 rounded-md border px-2 text-caption", active ? "border-brand/40 bg-brand/8 text-foreground" : "border-border text-muted-foreground hover:bg-muted")}
              onClick={() => updateStage(item.id, { statuses: active ? item.statuses.filter((value) => value !== status) : [...item.statuses, status] })}
            >
              {active && <Check className="size-3" />}
              {t(($) => $.designer.status[status])}
            </button>
          );
        })}
        {item.statuses
          .filter((status) => !ALL_STATUSES.includes(status as CanonicalStageStatus))
          .map((status) => (
            <button
              key={status}
              type="button"
              className="flex h-7 items-center gap-1.5 rounded-md border border-brand/40 bg-brand/8 px-2 text-caption text-foreground"
              title={t(($) => $.designer.remove_status)}
              onClick={() => updateStage(item.id, {
                statuses: item.statuses.filter((value) => value !== status),
              })}
            >
              <Check className="size-3" />
              {status}
            </button>
          ))}
        <Input className="h-7 w-36 text-caption" placeholder={t(($) => $.designer.custom_status)} onKeyDown={(event) => addCustomStatus(event, item)} />
      </div>
    </div>


  </>);
}

export function SopGateFields({item, members, agents, updateStage}: {item: SopStageDraft; members: readonly MemberWithUser[]; agents: readonly Agent[]; updateStage: UpdateStage}) {
  const { t } = useT("workflows");
  const defaultHumanId = (members.find((member) => member.role === "owner") ?? members[0])?.user_id ?? "";
  const defaultAgentId = agents.find((agent) => !agent.archived_at)?.id ?? "";
  const gateConfigured = gateIsConfigured(item, members, agents);
  return (
<div
      className={cn(
        "rounded-lg border p-4",
        item.gateEnabled
          ? gateConfigured
            ? "border-border bg-muted/20"
            : "border-destructive/40 bg-destructive/5"
          : "border-dashed border-border bg-muted/10",
      )}
    >
      <div className="flex items-start justify-between gap-4">
        <div className="min-w-0">
          <div className="flex items-center gap-2">
            <ShieldCheck className="size-4 text-muted-foreground" />
            <span className="text-body font-semibold">
              {t(($) => $.designer.gate)}
            </span>
          </div>
          <p className="mt-1 text-caption leading-5 text-muted-foreground">
            {t(($) => $.designer.gate_hint)}
          </p>
        </div>
        <Switch
          checked={item.gateEnabled}
          aria-label={t(($) => $.designer.gate)}
          onCheckedChange={(checked) =>
            updateStage(item.id, { gateEnabled: checked })
          }
        />
      </div>
      {item.gateEnabled && (
        <div className="mt-3 space-y-3 border-t border-border/70 pt-3">
          <div
            role="radiogroup"
            aria-label={t(($) => $.designer.decision_mode)}
            className="grid gap-2 md:grid-cols-3"
          >
            {(["human", "self_agent", "agent"] as DeciderType[]).map((type) => {
              const active = item.deciderType === type;
              return (
                <button
                  key={type}
                  type="button"
                  role="radio"
                  aria-checked={active}
                  className={cn(
                    "min-h-20 rounded-lg border p-2.5 text-left transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring",
                    active
                      ? "border-brand/45 bg-brand/8"
                      : "border-border/80 bg-background hover:border-brand/25 hover:bg-muted/30",
                  )}
                  onClick={() =>
                    updateStage(item.id, {
                      deciderType: type,
                      decider:
                        type === "human"
                          ? defaultHumanId
                          : type === "agent"
                            ? defaultAgentId
                            : "@self",
                      requireHuman: type === "human",
                      humanDecider:
                        type === "human"
                          ? defaultHumanId
                          : item.humanDecider || defaultHumanId,
                    })
                  }
                >
                  <span className="flex items-center gap-2">
                    <span
                      className={cn(
                        "flex size-7 items-center justify-center rounded-md",
                        active
                          ? "bg-brand text-brand-foreground"
                          : "bg-muted text-muted-foreground",
                      )}
                    >
                      {type === "human" ? (
                        <UserRound className="size-3.5" />
                      ) : type === "self_agent" ? (
                        <Bot className="size-3.5" />
                      ) : (
                        <UserCheck className="size-3.5" />
                      )}
                    </span>
                    <span className="text-caption font-semibold">
                      {t(($) => $.designer.decider[type])}
                    </span>
                  </span>
                  <span className="mt-1.5 block text-micro leading-4 text-muted-foreground">
                    {t(($) => $.designer.decider_hint[type])}
                  </span>
                </button>
              );
            })}
          </div>

          {item.deciderType === "human" && (
            <div>
              <label className="mb-1.5 block text-caption font-medium">
                {t(($) => $.designer.human_decider)}
              </label>
              <GateActorPicker
                kind="human"
                value={item.decider}
                members={members}
                agents={agents}
                onChange={(decider) =>
                  updateStage(item.id, {
                    decider,
                    humanDecider: decider,
                  })
                }
              />
            </div>
          )}

          {item.deciderType === "agent" && (
            <div>
              <label className="mb-1.5 block text-caption font-medium">
                {t(($) => $.designer.agent_decider)}
              </label>
              <GateActorPicker
                kind="agent"
                value={item.decider}
                members={members}
                agents={agents}
                onChange={(decider) =>
                  updateStage(item.id, { decider })
                }
              />
            </div>
          )}

          {item.deciderType === "self_agent" && (
            <div className="flex items-start gap-2 rounded-lg border border-border/70 bg-background px-3 py-2.5">
              <Bot className="mt-0.5 size-4 shrink-0 text-brand" />
              <p className="text-caption leading-5 text-muted-foreground">
                {t(($) => $.designer.self_agent_hint)}
              </p>
            </div>
          )}

          {item.deciderType !== "human" && (
            <div className="rounded-lg border border-border/80 bg-background p-3">
              <div className="flex items-start justify-between gap-3">
                <div className="min-w-0">
                  <p className="text-caption font-medium">
                    {t(($) => $.designer.require_human)}
                  </p>
                  <p className="mt-1 text-micro leading-4 text-muted-foreground">
                    {t(($) => $.designer.require_human_hint)}
                  </p>
                </div>
                <Switch
                  checked={item.requireHuman}
                  aria-label={t(($) => $.designer.require_human)}
                  onCheckedChange={(requireHuman) =>
                    updateStage(item.id, {
                      requireHuman,
                      humanDecider:
                        requireHuman && !item.humanDecider
                          ? defaultHumanId
                          : item.humanDecider,
                    })
                  }
                />
              </div>
              {item.requireHuman && (
                <div className="mt-3 border-t border-border/70 pt-3">
                  <label className="mb-1.5 block text-caption font-medium">
                    {t(($) => $.designer.final_human_decider)}
                  </label>
                  <GateActorPicker
                    kind="human"
                    value={item.humanDecider}
                    members={members}
                    agents={agents}
                    onChange={(humanDecider) =>
                      updateStage(item.id, { humanDecider })
                    }
                  />
                </div>
              )}
            </div>
          )}

          {!gateConfigured && (
            <p role="alert" className="text-caption text-destructive">
              {t(($) => $.designer.gate_actor_required)}
            </p>
          )}
        </div>
      )}
    </div>
  );
}
