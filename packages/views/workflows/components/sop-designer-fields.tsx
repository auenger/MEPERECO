"use client";
import { useMemo, useState } from "react";
import { Bot, ChevronDown, FileCheck2, UserRound, Wrench, X } from "lucide-react";
import type { Agent, MemberWithUser, SkillSummary } from "@silieco/core/types";
import { Badge } from "@silieco/ui/components/ui/badge";
import { Button } from "@silieco/ui/components/ui/button";
import { useT } from "../../i18n";
import { SkillPickerList } from "../../agents/components/skill-picker-list";
import { ActorAvatar } from "../../common/actor-avatar";
import { matchesPinyin } from "../../editor/extensions/pinyin-match";
import { PickerEmpty, PickerItem, PickerSection, PropertyPicker } from "../../tasks/components/pickers/property-picker";
import { WorkflowMarkdownEditor } from "./workflow-markdown-editor";
export function ArtifactMarkdownEditor({
  label,
  helper,
  placeholder,
  value,
  onChange,
}: {
  editorKey: string;
  icon: React.ReactNode;
  label: string;
  helper: string;
  placeholder: string;
  value: string;
  onChange: (value: string) => void;
}) {
  return <WorkflowMarkdownEditor label={label} value={value} onChange={onChange} placeholder={placeholder} helper={helper} />;
}

export function StageSkillSelector({
  skills,
  selectedRefs,
  loading,
  onChange,
}: {
  skills: readonly SkillSummary[];
  selectedRefs: readonly string[];
  loading: boolean;
  onChange: (next: string[]) => void;
}) {
  const { t } = useT("workflows");
  const [expanded, setExpanded] = useState(false);
  const selectedIds = useMemo(
    () =>
      new Set(
        skills
          .filter((skill) =>
            selectedRefs.some(
              (ref) => ref === skill.id || ref === skill.name,
            ),
          )
          .map((skill) => skill.id),
      ),
    [selectedRefs, skills],
  );
  const unresolved = selectedRefs.filter(
    (ref) => !skills.some((skill) => skill.id === ref || skill.name === ref),
  );

  const toggle = (skill: SkillSummary) => {
    const withoutSkill = selectedRefs.filter(
      (ref) => ref !== skill.id && ref !== skill.name,
    );
    onChange(
      selectedIds.has(skill.id)
        ? withoutSkill
        : [...withoutSkill, skill.id],
    );
  };

  const originLabel = (skill: SkillSummary) => {
    const origin = skill.config.origin;
    if (!origin || typeof origin !== "object" || Array.isArray(origin)) return null;
    const data = origin as Record<string, unknown>;
    const source = typeof data.type === "string" ? data.type : null;
    const revision = typeof data.revision === "string" ? data.revision.slice(0, 8) : null;
    return [source, revision].filter(Boolean).join(" · ") || null;
  };

  return (
    <div>
      <div className="flex items-center justify-between gap-2">
        <span className="flex items-center gap-1.5 text-caption font-medium">
          <Wrench className="size-3.5 text-muted-foreground" />
          {t(($) => $.designer.required_skills)}
        </span>
        <Button
          type="button"
          variant="ghost"
          size="sm"
          className="h-7 gap-1.5 px-2 text-caption"
          onClick={() => setExpanded((value) => !value)}
        >
          {expanded ? <X className="size-3" /> : <ChevronDown className="size-3" />}
          {expanded
            ? t(($) => $.designer.collapse_skills)
            : t(($) => $.designer.choose_skills)}
        </Button>
      </div>
      <div className="mt-1.5 rounded-lg border border-border/80 bg-muted/10 p-2.5">
        {selectedIds.size === 0 && unresolved.length === 0 ? (
          <p className="text-caption text-muted-foreground">
            {t(($) => $.designer.no_skills_selected)}
          </p>
        ) : (
          <div className="flex flex-wrap gap-1.5">
            {skills
              .filter((skill) => selectedIds.has(skill.id))
              .map((skill) => {
                const origin = originLabel(skill);
                return (
                  <Badge key={skill.id} variant="secondary" className="gap-1.5">
                    <FileCheck2 className="size-3" />
                    {skill.name}
                    {origin && <span className="font-mono text-micro text-muted-foreground">{origin}</span>}
                  </Badge>
                );
              })}
            {unresolved.map((ref) => (
              <Badge key={ref} variant="outline" className="gap-1.5">
                {ref}
                <button
                  type="button"
                  className="rounded-sm text-muted-foreground hover:text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
                  aria-label={t(($) => $.designer.remove_skill, { name: ref })}
                  onClick={() => onChange(selectedRefs.filter((value) => value !== ref))}
                >
                  <X className="size-3" />
                </button>
              </Badge>
            ))}
          </div>
        )}
        {expanded && (
          <div className="mt-2.5 border-t border-border/70 pt-2.5">
            <SkillPickerList
              skills={skills}
              selectedIds={selectedIds}
              onToggle={toggle}
              loading={loading}
              emptyMessage={t(($) => $.designer.no_workspace_skills)}
              noMatchMessage={t(($) => $.designer.no_skill_matches)}
            />
          </div>
        )}
        <p className="mt-2 text-micro text-muted-foreground">
          {t(($) => $.designer.skill_snapshot_hint)}
        </p>
      </div>
    </div>
  );
}

export function GateActorPicker({
  kind,
  value,
  members,
  agents,
  onChange,
}: {
  kind: "human" | "agent";
  value: string;
  members: readonly MemberWithUser[];
  agents: readonly Agent[];
  onChange: (id: string) => void;
}) {
  const { t } = useT("workflows");
  const [open, setOpen] = useState(false);
  const [filter, setFilter] = useState("");
  const query = filter.trim().toLowerCase();
  const matches = (name: string) =>
    !query || name.toLowerCase().includes(query) || matchesPinyin(name, query);
  const activeAgents = agents.filter((agent) => !agent.archived_at);
  const filteredMembers = members.filter((member) => matches(member.name));
  const filteredAgents = activeAgents.filter((agent) => matches(agent.name));
  const selectedMember = members.find((member) => member.user_id === value);
  const selectedAgent = activeAgents.find((agent) => agent.id === value);
  const selected = kind === "human" ? selectedMember : selectedAgent;

  return (
    <PropertyPicker
      open={open}
      onOpenChange={setOpen}
      width="w-72"
      align="start"
      searchable
      searchPlaceholder={
        kind === "human"
          ? t(($) => $.designer.search_human)
          : t(($) => $.designer.search_agent)
      }
      onSearchChange={setFilter}
      triggerRender={
        <button
          type="button"
          className="flex min-h-10 w-full items-center gap-2 rounded-lg border border-border/80 bg-background px-3 py-2 text-left transition-colors hover:border-brand/35 hover:bg-muted/30 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
        />
      }
      trigger={
        selected ? (
          <>
            <ActorAvatar
              actorType={kind === "human" ? "member" : "agent"}
              actorId={kind === "human" ? selectedMember!.user_id : selectedAgent!.id}
              size="sm"
              showStatusDot={kind === "agent"}
            />
            <span className="min-w-0 flex-1 truncate text-body font-medium">
              {selected.name}
            </span>
            <ChevronDown className="size-3.5 shrink-0 text-muted-foreground" />
          </>
        ) : (
          <>
            {kind === "human" ? (
              <UserRound className="size-4 text-muted-foreground" />
            ) : (
              <Bot className="size-4 text-muted-foreground" />
            )}
            <span className="min-w-0 flex-1 text-body text-muted-foreground">
              {kind === "human"
                ? t(($) => $.designer.select_human)
                : t(($) => $.designer.select_agent)}
            </span>
            <ChevronDown className="size-3.5 shrink-0 text-muted-foreground" />
          </>
        )
      }
    >
      {kind === "human" ? (
        filteredMembers.length > 0 ? (
          <PickerSection label={t(($) => $.designer.space_members)}>
            {filteredMembers.map((member) => (
              <PickerItem
                key={member.user_id}
                selected={member.user_id === value}
                onClick={() => {
                  onChange(member.user_id);
                  setOpen(false);
                }}
              >
                <ActorAvatar actorType="member" actorId={member.user_id} size="sm" />
                <span className="min-w-0 flex-1">
                  <span className="block truncate font-medium">{member.name}</span>
                  <span className="block truncate text-caption text-muted-foreground">
                    {member.email}
                  </span>
                </span>
              </PickerItem>
            ))}
          </PickerSection>
        ) : (
          <PickerEmpty />
        )
      ) : filteredAgents.length > 0 ? (
        <PickerSection label={t(($) => $.designer.space_agents)}>
          {filteredAgents.map((agent) => (
            <PickerItem
              key={agent.id}
              selected={agent.id === value}
              onClick={() => {
                onChange(agent.id);
                setOpen(false);
              }}
            >
              <ActorAvatar actorType="agent" actorId={agent.id} size="sm" showStatusDot />
              <span className="min-w-0 flex-1">
                <span className="block truncate font-medium">{agent.name}</span>
                {agent.description && (
                  <span className="block truncate text-caption text-muted-foreground">
                    {agent.description}
                  </span>
                )}
              </span>
            </PickerItem>
          ))}
        </PickerSection>
      ) : (
        <PickerEmpty />
      )}
    </PropertyPicker>
  );
}
