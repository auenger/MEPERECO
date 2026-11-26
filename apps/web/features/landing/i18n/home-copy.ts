import type { LandingDictionaryLocale } from "./types";

type HomeCopy = {
  pause: string;
  autoPlay: string;
  skip: string;
  intro: string;
  title: string[];
  description: string;
  watch: string;
  footnote: string;
  artLabel: string;
  human: string;
  humanRole: string;
  agentRole: string;
  handoff: string;
  result: string;
  run: string;
  evidence: string;
  review: string;
  artCaption: string;
  compatibility: string;
  more: string;
  workflowLabel: string;
  workflowTitle: string[];
  workflowDescription: string;
  useCases: string;
  demoTitle: string;
  demoLabel: string;
  chooseStep: string;
  stepCount: string;
  next: string;
  restart: string;
  steps: {
    title: string;
    owner: string;
    role: string;
    status: string;
    heading: string;
    description: string;
    file: string;
    detail: string;
    message: string;
  }[];
  filmLabel: string;
  filmTitle: string[];
  filmDescription: string;
  videoLabel: string;
  filmCaption: string;
  downloadFilm: string;
  videoError: string;
  capabilitiesTitle: string[];
  capabilitiesDescription: string;
  features: { title: string; description: string }[];
};

const zh: HomeCopy = {
  pause: "暂停演示",
  autoPlay: "自动演示",
  skip: "跳转到正文",
  intro: "人类与 Agent 的协作空间",
  title: ["人类与 Agent，", "共创一个生态。"],
  description:
    "人明确目标，Agent 推进执行。把任务、工作 SOP 与交付证据放在一起，让每一次交接都有方向。",
  watch: "看协作如何发生",
  footnote: "开源 · 可自部署 · 连接你自己的 Agent",
  artLabel:
    "人类定义目标，Agent 执行任务，在 MEPER Studio 中提交证据并交还给人验收。",
  human: "人类成员",
  humanRole: "定义目标 · 判断结果",
  agentRole: "理解上下文 · 推进执行",
  handoff: "目标与上下文",
  result: "产物已提交",
  run: "官网改版 · 等待验收",
  evidence: "页面预览与验证记录",
  review: "交给负责人确认",
  artCaption: "人与 Agent，共享一条工作流。",
  compatibility: "与你熟悉的 Agent 一起工作",
  more: "查看接入方式",
  workflowLabel: "把协作变成一条完整的流程",
  workflowTitle: ["知道怎么推进，", "也知道何时交接。"],
  workflowDescription:
    "用工作 SOP 明确阶段、负责人和验收条件。Agent 按规则推进，关键判断回到人手中，任务与证据一起交接。",
  useCases: "探索协作场景",
  demoTitle: "产品迭代 SOP",
  demoLabel: "交互演示",
  chooseStep: "选择工作流阶段",
  stepCount: "第 {n} 阶段 / 共 3 阶段",
  next: "下一步",
  restart: "重新体验",
  steps: [
    {
      title: "明确目标",
      owner: "产品负责人",
      role: "人类成员",
      status: "需求确认",
      heading: "先对齐，什么才算完成。",
      description:
        "将业务目标、关联资料和验收范围放进同一个任务，让执行从清晰的上下文开始。",
      file: "官网改版需求.md",
      detail: "目标 · 范围 · 验收条件",
      message: "首页需要讲清人机协作，嵌入宣传片，并适配手机。",
    },
    {
      title: "Agent 执行",
      owner: "开发 Agent",
      role: "数字员工",
      status: "执行与检查",
      heading: "按流程推进，带着结果回来。",
      description:
        "Agent 读取任务与资料，按 SOP 执行，并提交页面、文件和验证记录。",
      file: "页面预览与检查结果",
      detail: "实现产物 · 验证记录",
      message: "页面已经完成，附上预览和检查结果，请负责人验收。",
    },
    {
      title: "人类验收",
      owner: "研发负责人",
      role: "人类成员",
      status: "交接与验收",
      heading: "让最终判断，回到人手里。",
      description:
        "负责人核对交付物与验收条件，补充反馈或确认完成，过程留在项目中。",
      file: "验收结论.md",
      detail: "交付证据 · 业务反馈 · 完成记录",
      message: "已核对页面、视频与移动端体验，确认本次交付。",
    },
  ],
  filmLabel: "76 秒，看见协作的全貌",
  filmTitle: ["从一个任务，", "到一次共同交付。"],
  filmDescription:
    "人类员工与 Agent 数字员工，如何按照工作 SOP 协同推进？把这段过程，放进一支短片。",
  videoLabel: "MEPER Studio v3 人机协作宣传片，76 秒",
  filmCaption: "协作流程演绎 · 中文 · 可开启字幕与声音",
  downloadFilm: "下载完整影片",
  videoError: "暂时无法播放，可下载完整影片观看。",
  capabilitiesTitle: ["交付留下成果，", "协作留下经验。"],
  capabilitiesDescription:
    "把分散在对话、文件和工具里的工作，组织成下一次可以继续的上下文。",
  features: [
    {
      title: "工作 SOP，让经验可复用",
      description:
        "通过流程编辑器组织阶段与分工，用对话补全草稿，把团队的工作方法沉淀下来。",
    },
    {
      title: "项目资源，让上下文不掉线",
      description:
        "按项目和任务组织文件，让资料、执行产物和讨论围绕同一项工作聚合。",
    },
    {
      title: "自有运行环境，让执行落地",
      description:
        "连接本地或远程运行环境，让 Agent 在你的工具和基础设施里完成实际工作。",
    },
  ],
};

const en: HomeCopy = {
  pause: "Pause demo",
  autoPlay: "Play demo",
  skip: "Skip to content",
  intro: "A workspace for humans and agents",
  title: ["Human experience.", "Agent capability."],
  description:
    "People set the direction. Agents move work forward. Bring tasks, working procedures, and evidence together, so every handoff has a clear next step.",
  watch: "See it in action",
  footnote: "Open source · Self-hostable · Your own agents",
  artLabel:
    "Humans define goals, agents execute, and evidence returns to a human for review through MEPER Studio.",
  human: "Your team",
  humanRole: "Set direction · Review outcomes",
  agentRole: "Read context · Execute work",
  handoff: "Goals & context",
  result: "Deliverables submitted",
  run: "Website redesign · Ready for review",
  evidence: "Preview & verification records",
  review: "Handed to the reviewer",
  artCaption: "People and agents. One shared workflow.",
  compatibility: "Works with the agents you know",
  more: "Explore integrations",
  workflowLabel: "A complete path from intent to delivery",
  workflowTitle: ["Know the next step.", "Know when to hand off."],
  workflowDescription:
    "Define stages, owners, and acceptance criteria with SOPs. Agents follow the process; people make the key decisions. Every handoff carries its evidence.",
  useCases: "Explore use cases",
  demoTitle: "Product iteration SOP",
  demoLabel: "Interactive demo",
  chooseStep: "Choose a workflow stage",
  stepCount: "Stage {n} of 3",
  next: "Next step",
  restart: "Start again",
  steps: [
    {
      title: "Define",
      owner: "Product owner",
      role: "Human teammate",
      status: "Requirements",
      heading: "Agree on what done looks like.",
      description:
        "Keep goals, source material, and acceptance criteria in one task so execution starts with shared context.",
      file: "Website redesign brief.md",
      detail: "Goals · Scope · Acceptance criteria",
      message:
        "Explain human–agent collaboration, embed the film, and support mobile.",
    },
    {
      title: "Execute",
      owner: "Development agent",
      role: "Digital teammate",
      status: "Implementation",
      heading: "Follow the process. Return with evidence.",
      description:
        "The agent reads the task, follows the SOP, and submits deliverables with verification records.",
      file: "Preview & verification results",
      detail: "Deliverables · Verification records",
      message:
        "The page is ready. Preview and verification are attached for your review.",
    },
    {
      title: "Review",
      owner: "Engineering lead",
      role: "Human teammate",
      status: "Review & handoff",
      heading: "Keep the final judgment human.",
      description:
        "Review deliverables against the acceptance criteria, provide feedback or approve, and keep the record in the project.",
      file: "Acceptance record.md",
      detail: "Evidence · Feedback · Completion",
      message:
        "Page, video, and mobile experience reviewed. Delivery accepted.",
    },
  ],
  filmLabel: "The whole collaboration, in 76 seconds",
  filmTitle: ["From a shared task", "to a shared outcome."],
  filmDescription:
    "See how human teammates and digital workers follow an SOP, pass work along, and reach a shared delivery.",
  videoLabel: "MEPER Studio v3 collaboration film, 76 seconds",
  filmCaption: "Illustrated workflow · Chinese · Optional subtitles and sound",
  downloadFilm: "Download the film",
  videoError: "Playback is unavailable. Download the film to watch.",
  capabilitiesTitle: ["Keep the outcome.", "Build on the experience."],
  capabilitiesDescription:
    "Turn scattered conversations, files, and tools into context your team can carry into the next task.",
  features: [
    {
      title: "SOPs that carry your experience",
      description:
        "Organize stages and responsibilities in the workflow editor. Fill drafts through conversation and preserve your team’s working methods.",
    },
    {
      title: "Project resources that stay connected",
      description:
        "Organize files by project and task, keeping source material, deliverables, and discussions close to the work.",
    },
    {
      title: "Execution on your infrastructure",
      description:
        "Connect local or remote runtimes so agents can work with your tools, in environments you control.",
    },
  ],
};

export const homeCopy: Record<LandingDictionaryLocale, HomeCopy> = {
  zh,
  en,
};
