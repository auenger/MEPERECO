import type { Agent, MemberWithUser, SkillSummary, Workflow, WorkflowGateType, WorkflowStageInput } from "@silieco/core/types";

export type StageStatus = string;
export type CanonicalStageStatus = "backlog" | "todo" | "in_progress" | "in_review" | "done" | "blocked";
export type DeciderType = "human" | "self_agent" | "agent";
export type EvaluationMode = "on_task_change" | "manual" | "scheduled";
export interface SopStageDraft {
  executionMode?: string;
  executorId?: string | null;
  inputSpec?: Record<string, unknown>;
  inputBindings?: Record<string, unknown>;
  stableKey?: string;
  rollbackStageKey?: string | null;
  outputSpec?: Record<string, unknown>;
  reworkPolicy?: Record<string, unknown>;
  retryPolicy?: Record<string, unknown>;
  completionRule?: Record<string, unknown>;
  gateSpec?: Record<string, unknown>;
  id: string;
  name: string;
  description: string;
  statuses: StageStatus[];
  evaluationMode: EvaluationMode;
  schedule: string;
  input: string;
  output: string;
  skills: string[];
  allowedMcpServerIds: string[] | null;
  gateEnabled: boolean;
  deciderType: DeciderType;
  decider: string;
  requireHuman: boolean;
  humanDecider: string;
}
export interface SopTemplate {
  id: string;
  name: string;
  description: string;
  source: "preset" | "blank";
  stages: Array<Omit<SopStageDraft, "id">>;
}

export const DEFAULT_STATUSES: StageStatus[] = ["todo", "in_progress", "done"];
export const ALL_STATUSES: CanonicalStageStatus[] = ["backlog", "todo", "in_progress", "in_review", "done", "blocked"];
export function stage(
  name: string,
  input: string,
  output: string,
  skills: string,
  gate: Partial<Omit<SopStageDraft, "id" | "name" | "input" | "output" | "skills">> = {},
): Omit<SopStageDraft, "id"> {
  return {
    name,
    description: "",
    statuses: gate.statuses ?? DEFAULT_STATUSES,
    evaluationMode: "on_task_change",
    schedule: "",
    input,
    output,
    skills: skills.split(",").map((value) => value.trim()).filter(Boolean),
    allowedMcpServerIds: null,
    gateEnabled: gate.gateEnabled ?? false,
    deciderType: gate.deciderType ?? "human",
    decider: gate.decider ?? "",
    requireHuman: gate.requireHuman ?? false,
    humanDecider: "",
  };
}

export const SOP_TEMPLATES: SopTemplate[] = [
  {
    id: "system-development",
    name: "系统研发 SOP",
    description: "以四个阶段完成需求输入与整理、研发设计、任务实现及本地测试验收；每个阶段通过可追溯的 Markdown 产物交接项目上下文、决策和验证证据。",
    source: "preset",
    stages: [
      {
        ...stage(
          "需求的输入和整理",
          `## 原始材料
- 当前 Task 的目标、需求说明、讨论记录，以及上传的 Office、PDF、Markdown、图片等附件；列出文件名、版本、来源与提供人。
- 原文件作为来源保留，分析内容另行整理；扫描件、图片或表格无法完整读取时，注明缺失页、工作表或字段并提出补充请求。

## 项目上下文
- 阅读 Project 描述、绑定目录中的 README、项目规范、相关模块和现有设计文档，记录实际读取的路径与当前代码版本。
- 说明业务背景、目标用户、现有流程和已知约束；无法访问目录或缺少资料时列为待确认项，不凭空补全。

## 整理原则
- 区分用户明确提出的需求、已有系统事实、分析推断及待确认问题；每条关键需求能够追溯到材料位置或讨论记录。`,
          `## 产物存放与引用
- Markdown 产物必须实际写入 Project 绑定的项目目录，优先遵循已有文档组织和命名规范；没有约定时使用 docs/features/<需求或任务标识>/，同一需求的各阶段沿用同一目录。
- 在当前 Task 的交付记录中列出相对项目根目录的文件路径、用途与版本或更新时间；后续阶段按这些路径读取和更新，并使用相对链接关联来源与前置产物。
- 检查现有文件再写入，避免不同需求或运行覆盖同名文档。项目目录未绑定、不可访问或不可写时，说明阻塞并请求补齐，不将仅在聊天回复中给出的内容标记为已交付。

## 必交产物
- **requirements.md**：写明背景与业务目标、用户角色、核心场景、功能需求、非功能需求、范围内与范围外事项、依赖和约束。
- **source-index.md**：登记原始材料、版本、文件或附件位置、相关章节/页码/工作表，以及需求引用关系。
- **open-questions.md**：逐项列出歧义、缺失信息、负责人、建议选项和是否阻塞研发；没有问题也应明确记录。

## 质量与交接标准
- 为需求分配稳定编号（如 REQ-001），为验收条件分配编号（如 AC-001），说明可观察的输入、行为、结果和异常情况。
- 优先级、边界条件及权限、数据、安全或性能要求需具体到可讨论、可验证；未知指标标注待确认。
- 进入研发设计前应确认目标和本次范围；影响方案的阻塞问题未解决时保持阻塞，并附用户确认记录或待确认清单。`,
          "",
        ),
        description: `## 阶段目标
把分散的原始材料整理成可追溯、可确认、可用于设计的需求基线，同时保留原文及其来源。

## 执行内容
1. 盘点附件和讨论，结合 Project 描述与绑定目录了解系统现状，提取业务场景、功能边界和非功能约束。
2. 合并重复诉求，明确冲突和遗漏，为需求与验收条件编号，形成范围、优先级和待确认问题。
3. 与需求提出方核对关键结论；将确认事实与假设分开记录，给下一阶段交付需求基线和来源索引。`,
      },
      {
        ...stage(
          "需求转研发设计",
          `## 需求基线
- 上一阶段的 requirements.md、source-index.md、open-questions.md，以及关键范围和问题的确认记录。
- 确认 REQ 与 AC 编号可引用；仍存在影响数据、接口或权限设计的阻塞问题时，先补充澄清。

## 研发上下文
- 可访问的 Project 绑定目录、仓库版本、开发规范、现有架构、相关代码、数据模型、接口与测试入口。
- 复用已有组件和约定；记录外部依赖、兼容性要求、运行环境及本次改动涉及的模块。`,
          `## 产物存放与引用
- Markdown 产物必须实际写入 Project 绑定的项目目录，优先遵循已有文档组织和命名规范；没有约定时使用 docs/features/<需求或任务标识>/，同一需求的各阶段沿用同一目录。
- 在当前 Task 的交付记录中列出相对项目根目录的文件路径、用途与版本或更新时间；后续阶段按这些路径读取和更新，并使用相对链接关联来源与前置产物。
- 检查现有文件再写入，避免不同需求或运行覆盖同名文档。项目目录未绑定、不可访问或不可写时，说明阻塞并请求补齐，不将仅在聊天回复中给出的内容标记为已交付。

## 必交产物
- **development-design.md**：描述总体方案、模块职责、主要调用或数据流、接口输入输出与错误处理、数据结构与状态变化、权限边界及异常路径。
- 涉及页面时补充交互、空状态、加载和错误反馈；涉及数据变更时说明迁移、历史数据与回滚；不适用的部分说明原因。
- **development-tasks.md**：拆出可独立验证的研发任务，逐项写明目标、关联 REQ/AC、涉及路径、实现边界、依赖顺序、预期产物和完成条件。
- **test-plan.md**：按 AC 编号规划正常、异常、边界及回归场景，注明执行环境、测试数据、验证方式和通过标准。

## 设计通过标准
- 说明关键方案的选择理由、风险和待验证假设，覆盖每条范围内需求；对暂不实现的内容给出明确决定。
- 任务拆分应覆盖设计与验收条件，不能只有任务标题；如建立子 Task，记录对应编号与依赖关系。
- 方案经负责人确认后交接实现；关键技术假设无法验证或范围发生变化时，更新需求与设计记录。`,
          "",
          { gateEnabled: true, deciderType: "human", requireHuman: true },
        ),
        description: `## 阶段目标
将需求基线转换为可实现的研发方案和可执行的任务清单，让开发者明确改哪里、为什么改、如何验证。

## 执行内容
1. 阅读相关代码和规范，分析模块、接口、数据、权限及兼容性影响，确定最小可交付方案。
2. 给出具体设计和关键决策，按依赖关系划分任务，并把需求编号、设计项、研发任务和测试场景关联起来。
3. 评审实现风险与验收可行性，确认方案后进入实现；设计和需求有冲突时回到澄清，不将未确认假设当作结论。`,
      },
      {
        ...stage(
          "研发任务实现",
          `## 已确认的交接材料
- requirements.md、development-design.md、development-tasks.md、test-plan.md，以及设计评审和问题确认记录。
- 当前研发 Task 或子 Task 的编号、依赖任务结果、对应 REQ/AC、实现范围和完成条件。

## 开发环境
- Project 绑定的代码目录、目标分支和基线提交、项目规范、相关 Skill、构建及聚焦测试命令。
- 检查已有本地修改和依赖环境，明确本次任务涉及的路径；缺少执行权限、依赖或前置产物时记录阻塞原因。`,
          `## 产物存放与引用
- Markdown 产物必须实际写入 Project 绑定的项目目录，优先遵循已有文档组织和命名规范；没有约定时使用 docs/features/<需求或任务标识>/，同一需求的各阶段沿用同一目录。
- 在当前 Task 的交付记录中列出相对项目根目录的文件路径、用途与版本或更新时间；后续阶段按这些路径读取和更新，并使用相对链接关联来源与前置产物。
- 检查现有文件再写入，避免不同需求或运行覆盖同名文档。项目目录未绑定、不可访问或不可写时，说明阻塞并请求补齐，不将仅在聊天回复中给出的内容标记为已交付。

## 必交产物
- **实现代码与必要配置**：按设计完成对应模块，保持接口、权限和错误处理一致；按改动风险补充有意义的单元或集成测试。
- **implementation-notes.md**：逐项记录研发任务完成情况、关联 REQ/AC、实际变更路径、关键实现、设计偏差及其确认记录。
- **验证与运行交接**：记录已执行的构建/聚焦测试命令和结果、本地启动方式、所需配置及测试数据准备方式；敏感信息用配置项名称引用。

## 完成标准
- 每个研发任务有明确的代码或文档产物及可验证结果；未完成、失败和阻塞事项单独列出，不能直接标记已完成。
- 涉及迁移或兼容性变化时交付对应脚本、执行顺序与恢复说明，更新受影响的项目文档。
- 为下一阶段提供可定位的代码版本或变更清单；需求变更先同步基线和设计，不在实现中扩大范围。
- 提交、推送和服务器部署按任务授权执行；本阶段的聚焦自测结果作为交接证据，最终本地验收由下一阶段完成。`,
          "",
          { statuses: ["todo", "in_progress", "in_review", "done", "blocked"] },
        ),
        description: `## 阶段目标
依据已确认方案按任务完成可运行、可验证、可交接的实现，持续记录需求与代码之间的对应关系。

## 执行内容
1. 按依赖顺序开展研发任务，复用项目已有能力，并遵守绑定目录中的开发规范和架构边界。
2. 实现业务逻辑、界面或接口，处理异常与权限，执行与改动匹配的聚焦检查并修复问题。
3. 汇总代码变更、任务状态和剩余风险，准备本地启动和验证所需资料，将实现交给测试验收阶段。`,
      },
      {
        ...stage(
          "研发任务本地测试和验收",
          `## 待验收实现
- 可定位的代码版本或完整变更清单、implementation-notes.md、研发任务及子 Task 的完成情况。
- requirements.md 中的 AC 验收条件、已确认设计、test-plan.md，以及实现阶段的测试结果和已知问题。

## 本地验证准备
- 可复现的启动命令、依赖与配置说明、测试账号/角色、测试数据及清理方式；涉及迁移时先验证迁移与恢复步骤。
- 明确本地验证覆盖的服务、前后端连接目标和环境差异；测试数据与操作范围必须清楚，无法模拟的条件单独说明。`,
          `## 产物存放与引用
- Markdown 产物必须实际写入 Project 绑定的项目目录，优先遵循已有文档组织和命名规范；没有约定时使用 docs/features/<需求或任务标识>/，同一需求的各阶段沿用同一目录。
- 在当前 Task 的交付记录中列出相对项目根目录的文件路径、用途与版本或更新时间；后续阶段按这些路径读取和更新，并使用相对链接关联来源与前置产物。
- 检查现有文件再写入，避免不同需求或运行覆盖同名文档。项目目录未绑定、不可访问或不可写时，说明阻塞并请求补齐，不将仅在聊天回复中给出的内容标记为已交付。

## 必交产物
- **verification-report.md**：注明代码版本、测试环境、实际命令与结果，按 AC 编号逐项填写通过/失败/未验证，附日志、截图或操作证据的位置。
- 覆盖主要用户路径、异常和边界情况、角色权限及相关回归；记录构建、类型检查、单元/集成测试和界面操作验证中实际执行的项目。
- **acceptance-summary.md**：总结本次范围、需求和任务覆盖情况、遗留问题、风险、验收结论、验收人及确认时间；标明本地测试结果与其他环境验证的边界。

## 验收通过标准
- 范围内 AC 有逐项证据且全部通过，阻塞缺陷已修复并复测；无法执行的检查说明原因、影响和后续处理，不得记为通过。
- 测试失败应记录复现步骤、预期/实际结果和对应研发任务，退回实现修复后重复相关验收与回归。
- 非阻塞遗留项需有负责人和明确处理决定，最终由人工确认验收结论；未经确认不结束运行。
- 交付本地运行说明和产物索引，验收完成不代表已完成服务器部署。`,
          "",
          { statuses: ["todo", "in_progress", "in_review", "done", "blocked"], gateEnabled: true, deciderType: "human", requireHuman: true },
        ),
        description: `## 阶段目标
在明确的本地环境中验证实现是否满足需求和设计，以实际证据完成验收，形成可复现的交付记录。

## 执行内容
1. 准备环境与数据，按测试计划启动系统并执行必要的自动化检查和用户流程验证。
2. 对照 AC 逐项核验功能、异常、权限和回归结果，区分通过、失败与未验证，保留命令和操作证据。
3. 推动缺陷回到实现阶段修复并复测，汇总遗留风险，由验收负责人确认最终结果。`,
      },
    ],
  },
  {
    id: "feature-development",
    name: "Feature 研发工作流",
    description: "面向第一版交付的轻量 Feature 流程：用 Task 和 Markdown 产物串联需求、拆分、开发、验证与归档。",
    source: "preset",
    stages: [
      {
        ...stage(
          "需求输入与分析",
          "当前 Task / 原始需求 / 用户反馈 / 业务目标",
          "feature-brief.md 或 Task 内的等价需求说明",
          "silieco-new-feature",
        ),
        description: "把当前大 Task 收敛为目标、范围、约束、验收条件和待确认问题。",
      },
      {
        ...stage(
          "PRD 产出",
          "已确认的 feature-brief.md / Task 需求说明",
          "PRD.md（含验收条件与候选开发切片）",
          "silieco-enrich-feature",
        ),
        description: "输出足够支撑第一版开发的轻量 PRD，不强制额外生成 task.md 和 checklist.md。",
      },
      {
        ...stage(
          "PRD 校验",
          "PRD.md / 项目与仓库上下文",
          "spec-review.md（pass 或 block）",
          "silieco-review-spec",
          {
            statuses: ["todo", "in_review", "done", "blocked"],
            gateEnabled: true,
            deciderType: "human",
            requireHuman: true,
          },
        ),
        description: "只检查会阻碍第一版实现或验收的问题，通过人工 Gate 确认范围。",
      },
      {
        ...stage(
          "原型或 MVP",
          "已通过校验的 PRD.md",
          "prototype-decision.md + 可选 Prototype / MVP",
          "silieco-prototype-feature",
          {
            statuses: ["todo", "in_progress", "in_review", "done", "blocked"],
            gateEnabled: true,
            deciderType: "human",
            requireHuman: true,
          },
        ),
        description: "先判断是否需要原型；仅在存在关键不确定性时产出最小 Demo 或技术 Spike。",
      },
      {
        ...stage(
          "Feature 拆分与开发准备",
          "当前大 Task / PRD.md / 原型结论",
          "已关联的开发子 Task / 分组顺序 / 覆盖摘要",
          "silieco-split-feature",
        ),
        description: "把大 Task 拆成父子关联的小 Task，用子任务 Stage 屏障表达并行组与先后顺序。",
      },
      {
        ...stage(
          "开发实现",
          "当前开发子 Task / PRD.md / 前置子 Task 结果",
          "代码 / 聚焦测试 / 实现交接说明",
          "silieco-run-feature",
          {
            statuses: ["todo", "in_progress", "in_review", "done", "blocked"],
          },
        ),
        description: "每个子 Task 独立执行开发前检查和实现；不在本 Stage 重复执行最终验收。",
      },
      {
        ...stage(
          "本地化测试与验收",
          "待验收实现 / PRD 验收条件 / 子 Task 结果",
          "verification-report.md 或 Task 验收记录",
          "silieco-verify-feature",
          {
            statuses: ["todo", "in_progress", "in_review", "done", "blocked"],
            gateEnabled: true,
            deciderType: "human",
            requireHuman: true,
          },
        ),
        description: "按改动范围执行本地检查并验证验收条件；失败保持 block，不进入完成阶段。",
      },
      {
        ...stage(
          "完成与归档",
          "通过验收的实现 / 子 Task 状态 / verification-report.md",
          "completion-summary.md / Release Notes / 后续项",
          "silieco-complete-feature",
          {
            statuses: ["todo", "in_review", "done", "blocked"],
            gateEnabled: true,
            deciderType: "human",
            requireHuman: true,
          },
        ),
        description: "汇总子 Task 和验证结果，准备交付记录；合并、推送与发布仍按授权执行。",
      },
    ],
  },
];

export function blankSopTemplate(): SopTemplate {
  return {
    id: "blank",
    name: "空白 SOP",
    description: "",
    source: "blank",
    stages: [stage("", "", "", "")],
  };
}

let stageSequence = 0;
export function cloneStages(template: SopTemplate) {
  return template.stages.map((item, index) => ({
    ...item,
    statuses: [...item.statuses],
    id: `sop-stage-${++stageSequence}`,
    stableKey: `stage_${index + 1}`,
    rollbackStageKey: index > 0 ? `stage_${index}` : null,
  }));
}
export function resolvedGateType(item: SopStageDraft): WorkflowGateType {
  if (!item.gateEnabled) return "none";
  if (item.deciderType === "human") return "human";
  return item.requireHuman ? "hybrid" : "agent";
}

export function objectString(value: Record<string, unknown>, key: string): string {
  return typeof value[key] === "string" ? value[key] : "";
}

export function normalizeSkillRefs(
  refs: readonly string[],
  workspaceSkills: readonly SkillSummary[],
): string[] {
  return refs.map((ref) => {
    const match = workspaceSkills.find(
      (skill) => skill.id === ref || skill.name === ref,
    );
    return match?.id ?? ref;
  });
}

export function resolveStageActors(
  drafts: SopStageDraft[],
  members: readonly MemberWithUser[],
  agents: readonly Agent[],
): SopStageDraft[] {
  const defaultHuman =
    members.find((member) => member.role === "owner") ?? members[0];
  const activeAgents = agents.filter((agent) => !agent.archived_at);
  const defaultAgent = activeAgents[0];

  return drafts.map((draft) => {
    const humanExists = members.some(
      (member) => member.user_id === draft.decider,
    );
    const agentExists = activeAgents.some((agent) => agent.id === draft.decider);
    const finalHumanExists = members.some(
      (member) => member.user_id === draft.humanDecider,
    );
    return {
      ...draft,
      decider:
        draft.deciderType === "human"
          ? humanExists
            ? draft.decider
            : (defaultHuman?.user_id ?? "")
          : draft.deciderType === "agent"
            ? agentExists
              ? draft.decider
              : (defaultAgent?.id ?? "")
            : "@self",
      requireHuman:
        draft.deciderType === "human" ? true : draft.requireHuman,
      humanDecider:
        draft.requireHuman && draft.deciderType !== "human"
          ? finalHumanExists
            ? draft.humanDecider
            : (defaultHuman?.user_id ?? "")
          : draft.humanDecider,
    };
  });
}

export function gateIsConfigured(
  draft: SopStageDraft,
  members: readonly MemberWithUser[],
  agents: readonly Agent[],
): boolean {
  if (!draft.gateEnabled) return true;
  if (
    draft.deciderType === "human" &&
    !members.some((member) => member.user_id === draft.decider)
  ) {
    return false;
  }
  if (
    draft.deciderType === "agent" &&
    !agents.some((agent) => !agent.archived_at && agent.id === draft.decider)
  ) {
    return false;
  }
  return !(
    draft.deciderType !== "human" &&
    draft.requireHuman &&
    !members.some((member) => member.user_id === draft.humanDecider)
  );
}

export function updateArtifactDescription(spec: Record<string, unknown> | undefined, value: string) {
  const updated = { ...spec };
  if (value.trim()) updated.default_artifact = value.trim();
  else delete updated.default_artifact;
  return updated;
}

export function stagesFromWorkflow(workflow: Workflow): SopStageDraft[] {
  return (workflow.current_version?.stages ?? []).map((item) => {
    const completionRule = item.completion_rule ?? {};
    const inputSpec = item.input_spec ?? {};
    const outputSpec = item.output_spec ?? {};
    const gate = item.gate ?? {};
    const mode = completionRule.evaluation_mode;
    const allowedStatuses = completionRule.allowed_task_statuses;
    const gateType = gate.type;
    const deciderType = gate.decider_type;
    return {
      id: `sop-stage-${++stageSequence}`,
      executionMode: item.execution_mode ?? "manual",
      executorId: item.executor_id ?? null,
      inputSpec: { ...inputSpec },
      inputBindings: { ...item.input_bindings },
      stableKey: item.stable_key,
      rollbackStageKey: item.rollback_stage_key ?? null,
      outputSpec: { ...outputSpec },
      completionRule: { ...completionRule }, reworkPolicy:{...item.rework_policy}, retryPolicy:{...item.retry_policy},
      name: item.name,
      description: item.description ?? "",
      statuses: Array.isArray(allowedStatuses)
        ? allowedStatuses.filter((value): value is string => typeof value === "string")
        : [...DEFAULT_STATUSES],
      evaluationMode:
        mode === "on_task_change" || mode === "scheduled"
          ? mode
          : "manual",
      schedule: objectString(completionRule, "schedule"),
      input: objectString(inputSpec, "default_artifact"),
      output: objectString(outputSpec, "default_artifact"),
      skills: [...item.required_skills],
      allowedMcpServerIds: item.allowed_mcp_server_ids ?? null,
      gateSpec: { ...gate },
      gateEnabled: gateType !== undefined && gateType !== "none",
      deciderType:
        deciderType === "self_agent" || deciderType === "agent"
          ? deciderType
          : deciderType === "human" ? "human"
            : gateType === "agent" || gateType === "hybrid" ? "agent" : "human",
      decider: objectString(gate, "decider"),
      requireHuman: gateType === "human" || gateType === "hybrid" || gate.require_human === true,
      humanDecider: objectString(gate, "human_decider"),
    };
  });
}


// Stable keys are independent of display ordering and survive editing.
export function serializeSopStages(stages: readonly SopStageDraft[], workspaceSkills: readonly SkillSummary[]): WorkflowStageInput[] {
  const valid = stages.filter(item => item.name.trim());
  const reserved = new Set(valid.flatMap(item => item.stableKey ? [item.stableKey] : []));
  return valid.map((item, index) => {
    let key = item.stableKey;
    if (!key) { let number = index + 1; while (reserved.has(`stage_${number}`)) number++; key = `stage_${number}`; reserved.add(key); }
    const completion = { ...item.completionRule };
    if (item.executionMode !== "agent") {
      completion.type = "all_tasks_terminal";
      completion.evaluation_mode = item.evaluationMode;
      completion.allowed_task_statuses = [...item.statuses];
      if (item.evaluationMode === "scheduled") { completion.schedule = item.schedule.trim(); }
      else { delete completion.schedule; delete completion.timezone; }
    } else { completion.type = "validated_result"; delete completion.evaluation_mode; delete completion.allowed_task_statuses; delete completion.schedule; delete completion.timezone; }
    return {
      stable_key: key, name: item.name.trim(), description: item.description.trim() || null,
      execution_mode: item.executionMode ?? "manual", executor_id: item.executorId ?? null,
      rework_policy: item.reworkPolicy ?? {}, retry_policy: item.retryPolicy ?? {}, completion_rule: completion,
      input_bindings: item.inputBindings ?? {}, input_spec: updateArtifactDescription(item.inputSpec, item.input), output_spec: updateArtifactDescription(item.outputSpec, item.output),
      required_skills: normalizeSkillRefs(item.skills, workspaceSkills), allowed_mcp_server_ids: item.allowedMcpServerIds,
      gate: { ...item.gateSpec, type: resolvedGateType(item), decider_type: item.deciderType, decider: item.decider.trim() || null, require_human: item.deciderType === "human" ? true : item.requireHuman, human_decider: item.deciderType === "human" ? item.decider.trim() || null : item.requireHuman ? item.humanDecider.trim() || null : null },
      rollback_stage_key: item.rollbackStageKey ?? null,
    };
  });
}
