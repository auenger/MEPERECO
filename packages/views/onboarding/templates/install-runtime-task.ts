/**
 * Skip path, task 1/2: "Connect a runtime to start using agents".
 *
 * Written to a new task (assigned to the user themselves) by the welcome
 * hook when the user took the Skip exit on Step 3. Content is the
 * install-runtime tutorial; each supported locale can recommend the
 * quickest runtime path that best fits that audience.
 *
 * Title is stable — kept identical to the v2 server-side
 * `NoRuntimeTaskTitle` so any existing dedupe code elsewhere keeps
 * matching by title.
 */

/**
 * Step 1 of the skip-path bundle. Localized so users see the title in
 * their current supported locale on the board.
 *
 * Note: server's deprecation shim (`onboarding_shim.go:noRuntimeTaskTitle`)
 * still uses the bare English string for its title-based dedupe — that
 * codepath only runs for pre-v3 desktop builds and never overlaps with
 * the v3 frontend population, so the two title-spaces drifting is fine.
 */
export const INSTALL_RUNTIME_TASK_TITLE = {
  en: "Step 1 — Connect a runtime to start using agents",
  zh: "第 1 步 —— 连接运行时,开始使用 agent",
} as const;

const en = `Welcome to MEPERStudio.

Agents need a runtime before they can execute work. You can still use MEPERStudio as a lightweight project-management workspace while you install one.

## Try MEPERStudio first

Before the runtime is ready, you can:

1. Create a project for your current work.
2. Create a few tasks and move them across backlog, todo, in_progress, and done.
3. Add priorities, labels, comments, and subscriptions.
4. Use Inbox to track assignments and mentions.

That gives you the project-management layer first. Once a runtime is connected, agents can start working from the same tasks.

## Connect MEPERStudio to an execution machine (optional)

Desktop includes the daemon. If using the browser or a remote server, open Runtimes → Connect remote and follow the platform-specific MEPERStudio CLI installation instructions. No daemon is required to create or manage a Space.

## Install your first agent runtime

Recommended: Pi — open-source, lightweight, works with 25+ model providers.

1. Install Pi (requires Node.js 22.19+, or use the official installer):
   npm i -g @earendil-works/pi-coding-agent
   macOS / Linux one-liner:
   curl -fsSL https://pi.dev/install.sh | sh
   Windows PowerShell:
   irm https://pi.dev/install.ps1 | iex
2. Confirm your terminal can find it:
   pi --version
3. Start it once and log in (pick a provider, then sign in to a subscription or paste an API key):
   pi
4. Wait for MEPERStudio to pick it up. A running daemon re-checks for newly
   installed CLIs every couple of minutes, so no restart is normally needed.
   To apply it immediately:
   meperstudio daemon restart
   In the desktop app, open any local runtime and click Restart. Quitting and
   reopening the app is NOT enough — the daemon keeps running in the background.
5. Return to Runtimes and refresh. You should see a Pi runtime online.
6. Create your first agent from that runtime, then assign an task to the agent and set status to todo.

Pi reference: https://github.com/earendil-works/pi

Tip — one place for all your models: Magpie (https://usemagpie.ai) runs a
local gateway that lets Pi, Claude Code, Codex and other agents share your
subscriptions and API keys, with per-agent model switching. Pi is officially
supported by Magpie, so you can point Pi at it once and switch models
without touching Pi's config again.

Prefer Codex instead?

1. Make sure Node.js is installed.
2. Install Codex:
   npm i -g @openai/codex
3. Sign in:
   codex
4. Confirm your terminal can find it:
   which codex
   codex --version
5. Wait for MEPERStudio to pick it up (same as above — \`meperstudio daemon restart\` applies it immediately).
6. Return to Runtimes and refresh. You should see a Codex runtime online.
7. Create your first agent from that runtime, then assign an task to the agent and set status to todo.

Codex reference: https://developers.openai.com/codex/cli

Already use ZCode?
1. Install the official ZCode application/CLI from https://zcode.z.ai/ and sign in if needed.
2. Verify \`zcode --version\` in your terminal, then refresh Runtimes.
3. For a Coding Plan subscription, authorize the CLI with \`zcode login bigmodel\` (or \`zcode login zai\` for a global Z.ai account).
4. Create an agent using ZCode. The runtime reuses your existing account, skills, native agents and commands. Manage models in ZCode or optionally Magpie (https://usemagpie.ai/); MEPERStudio drives the CLI without modifying provider configuration. A desktop conversation's model may not be the CLI default.

When the runtime is connected, you can create MEPER Agent for a guided first run.`;

const zh = `欢迎来到 MEPERStudio。

智能体需要先连上运行时才能执行工作。运行时还没准备好时,你也可以先把 MEPERStudio 当作轻量项目管理工具体验起来。

## 先体验项目管理功能

运行时安装前,你可以先做这些事:

1. 为当前工作创建一个项目。
2. 新建几个 task,并在 backlog、todo、in_progress、done 之间流转。
3. 给 task 加优先级、标签、评论和订阅。
4. 用收件箱追踪分配给你的事项和 @mention。

这样你先熟悉项目管理层。连上运行时后,智能体会直接在这些 task 上开始工作。

## 可选：连接 MEPERStudio 执行机器

桌面端内置 Daemon。通过浏览器或远程服务器使用时，进入运行时页面，选择连接远程机器并按对应平台安装 MEPERStudio CLI。创建和管理空间不需要 Daemon。

## 安装第一个 Agent 运行时

推荐:Pi —— 开源轻量,支持 25+ 模型供应商。

1. 安装 Pi(需要 Node.js 22.19+,或直接用官方安装脚本):
   npm i -g @earendil-works/pi-coding-agent
   macOS / Linux 一行命令:
   curl -fsSL https://pi.dev/install.sh | sh
   Windows PowerShell:
   irm https://pi.dev/install.ps1 | iex
2. 确认终端能找到 Pi:
   pi --version
3. 启动一次并登录(选一个供应商,订阅登录或粘贴 API key 均可):
   pi
4. 等 MEPERStudio 识别到它。运行中的守护进程每隔几分钟会重新检查一次新装的 CLI,通常不需要重启。
   想立刻生效:
   meperstudio daemon restart
   桌面端请打开任意一个本机 runtime 并点 Restart。退出再打开 app 是不够的 —— 守护进程会继续在后台运行。
5. 回到 Runtimes 页面刷新。你应该能看到一个在线的 Pi 运行时。
6. 用这个运行时创建第一个智能体,再把一个 task 分配给它,并把状态切到 todo。

Pi 官方仓库:https://github.com/earendil-works/pi

小提示 —— 模型统一入口:Magpie(https://usemagpie.ai)是一个本地网关,
能让 Pi、Claude Code、Codex 等智能体共享你的订阅账号和 API key,并按
智能体切换模型。Magpie 官方支持 Pi,把 Pi 指向它一次,之后换模型不用
再改 Pi 的配置。

也可以选择 Kimi CLI:

1. 在 macOS / Linux 终端安装 Kimi CLI:
   curl -LsSf https://code.kimi.com/install.sh | bash
   Windows PowerShell:
   Invoke-RestMethod https://code.kimi.com/install.ps1 | Invoke-Expression
2. 确认终端能找到 Kimi:
   kimi --version
3. 在你想让 Kimi 工作的项目目录里启动一次:
   kimi
4. 首次启动后输入 /login,按提示完成 Kimi Code 或 API key 配置。
5. 等 MEPERStudio 识别到它(同上 —— \`meperstudio daemon restart\` 立即生效)。
6. 回到 Runtimes 页面刷新。你应该能看到一个在线的 Kimi 运行时。
7. 用这个运行时创建第一个智能体,再把一个 task 分配给它,并把状态切到 todo。

Kimi CLI 官方文档:https://moonshotai.github.io/kimi-cli/zh/guides/getting-started.html

已经在使用 ZCode？
1. 从 https://zcode.z.ai/ 安装官方 ZCode，按需完成登录。
2. 在终端确认 \`zcode --version\` 可用，回到运行时页面刷新。
3. Coding Plan 订阅用户请执行 \`zcode login bigmodel\`（海外 Z.ai 账号使用 \`zcode login zai\`），完成官方终端授权。
4. 选择 ZCode 创建 Agent，沿用本机账号、技能、原生 Agent 和命令。模型由 ZCode 或 Magpie（可选，https://usemagpie.ai/）管理，MEPERStudio 仅驱动 CLI，不修改 provider 配置。桌面会话内切换模型不一定同步到终端。

运行时连上后，你就可以创建 MEPER Agent，开始一次有智能体参与的上手引导。`;

export const INSTALL_RUNTIME_TASK_BODY = { en, zh,} as const;

/**
 * Prefix sentence for the follow-up comment posted on this task (the one
 * that links to the create-agent-guide task via a mention chip). Kept
 * here as a TS const rather than an i18n JSON key because anything that
 * gets persisted to the DB must be available at write time without
 * depending on an i18n bundle having loaded the new key — otherwise a
 * cold dev server / stale build writes the raw key string into
 * `comment.content` and the comment is permanently broken.
 */
export const FOLLOWUP_COMMENT_PREFIX = {
  en: "Your next step:",
  zh: "完成后的下一步：",
} as const;
