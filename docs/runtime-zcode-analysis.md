# ZCode（智谱官方 CLI）Runtime 集成

> **【历史决策 2026-11-24】暂不推进 ZCode 集成。** 原因：ZCode 专属 1.5× 额度加成是限时活动且已结束，失去核心吸引力；资源集中投入 pi 自动化（见 `docs/runtime-pi-analysis.md`）。本文保留作历史调研记录。
>
> 调研日期：2026-11-24 · 当时结论：可集成。登录官方账号 ✅、Coding Plan 套餐额度 ✅（注意：ZCode 专属 1.5× 加成已结束）、daemon headless 驱动 ✅（stream-json 一等支持）
> 关联文档：`docs/runtime-pi-analysis.md`（pi 默认运行时推进，两者分发/网关策略并轨）

> **【更新 2026-11-26】恢复 ZCode 集成，优先复用用户已有本机生态。** 不分发第三方安装包，不复制或迁移登录凭据，不修改用户 `~/.zcode` 配置与会话数据。

### 首版实现

- PATH / `SILIECO_ZCODE_PATH` 自动识别；macOS 可识别官方桌面包内可执行的 CLI kernel。有 CLI 版本探测，不注册损坏的安装。
- 使用官方 headless stream-json 协议，显式 guarded 权限模式，workflow onDemand，支持按 sessionId 恢复会话；不使用默认会话防止并发串话。
- 继承已有 ZCode 账号、模型、用户技能、子 Agent、命令及工作流。MEPERStudio 的任务级 `AGENTS.md` 与 `.zcode/skills` 写在任务工作目录内；本机技能可从 `~/.zcode/skills` 或用户配置的数据基目录加载。
- **模型配置归属：ZCode 或 Magpie（可选）**。MEPERStudio 仅驱动 CLI，不读取、复制或改写用户的 provider 配置，不生成临时模型配置，Agent 模型字段显示「由运行时管理」。旧显式模型覆盖应清除（后端在启动前拒绝，避免误以为已指定模型）；使用 CLI 实际报告的模型进行用量统计。不自动授权 Hook，也不接管 ZCode 自己的 MCP 配置。
- 官方桌面 CLI 包的内置 provider 配置路径不匹配时，在启动进程环境中补上已安装的官方资源文件路径，不改用户数据。
- macOS 后台执行：优先使用官方安装包中带 `LSUIElement=true` 的 `ZCode Helper.app` 以 Node 模式执行 CLI，避免每次对话拉起主 GUI 宿主而出现 Dock 临时进程图标。沿用原生凭据模块与系统钥匙串授权；实际 Coding Plan GLM-5.4 冒烟通过。
- 本机实测 CLI 0.18.0：可读 119 个 Skill 与 8 个命令。官方桌面包的 terminal kernel 原生凭据模块解析失败，通过官方 Electron Node host 加载原包内的钥匙串模块后，已完成 BigModel Coding Plan 终端 OAuth 授权。实际 MEPERStudio Go Runtime 指定原生模型 GLM-5.4 冒烟通过（包含 session ID、流式回复和用量）；GLM-5.3-Flash 的临时配置实测回退至 GLM-5.3，不能视为指定成功，因此当前官方账号只使用原生选定模型。同日用户在官方桌面会话中切换至 GLM-5.3-Flash，已从原生 model_usage 记录确认该 Coding Plan 账号实际调用了 Flash；但切换只改变桌面会话的模型，终端全局默认仍为 GLM-5.4。再次用独立配置指定 Flash，终端仍回退 GLM-5.3，Runtime 已中止该次运行；这属于终端模型选择链路限制，并非套餐没有 Flash。改为完全由运行时管理模型后，未指定 Agent 模型的真实 CLI 冒烟再次通过（会话、响应、用量齐全）。真实会话续跑及 Windows 上的实际模型调用仍待实测。
- 以本机 ZCode 原生生态复用为目标；不把原生 Agent / workflow 自动创建为 MEPERStudio 的同名数据库对象，不上传本地凭据、会话或配置。
- 官方 [CLI 参数文档](https://github.com/zai-org/ZCode/blob/main/apps/zcode-cli/docs/usage/02-cli-reference.md) 明确目前没有 `--model`，官方反馈仓库的 [#1044](https://github.com/zai-org/feedback/issues/1044) 记录过无头模式无法指定订阅模型及静默回退的同类现象。MEPERStudio 在选择 ZCode Runtime、编辑 Agent、连接运行时及运行时详情时展示非阻断提示：直接使用官方订阅时先完成终端授权；由 ZCode 或可选的 [Magpie](https://usemagpie.ai/) 管理模型，桌面会话模型切换不保证同步，最终以 CLI 实际调用为准。Magpie 的 [官方配置参考](https://github.com/yetone/magpie/blob/main/docs/reference.md) 已列出 ZCode provider 支持；本项目不集成或代替 Magpie，不要求安装，也不承诺其对特定订阅模型的配置必然在 headless 中生效。

---

## 1. 背景与期望

团队期望：① 用户可直接登录智谱官方账号；② 享受 token 用量加成；③ daemon 可驱动 ZCode 作为运行时。仓库：https://github.com/zai-org/ZCode 。

## 2. ZCode 事实卡片（2026-11-24 一手核实）

| 项 | 事实 |
| --- | --- |
| 身份 | 智谱 Z.ai 官方 Agent 开发环境（ADE）的终端 CLI/TUI，官方 GitHub `zai-org/ZCode`（"Z.ai's coding agent harness"） |
| 官网/文档 | https://zcode.z.ai/en 、https://zcode.z.ai/en/docs |
| 许可 | 第一方代码 **Apache-2.0**（第三方字体/图标/原生二进制有独立条款，见 NOTICE.md）；商标/Logo 不可用 |
| 版本节奏 | 2026-09-20 建仓，8,883 stars；约每周发版：v3.14.3（9-24）→ v4.0.0（**2026-11-23**） |
| 平台 | Windows x64 + ARM64、macOS（AS/Intel）、Linux x64/ARM64 全覆盖 |
| 分发 | ① 桌面安装包（Electron 44，官方 CDN `cdn-zcode.z.ai`）；② CLI：Node bundle `zcode.cjs` 或 **SEA 单文件**（darwin/linux/win × arm64/x64，Windows 产物 `zcode-windows-<arch>.exe`）；③ 命令行版发行包（TUI+Web+后端）。**⚠️ SEA 二进制无公网下载渠道**（CI 产内部共享盘）；**npm 无官方包且全是冒名**（`zcode` 占位、`zcode-cli`、`@clid/zcode`、`@mbearo/zcode`（Rust 重实现）等均为社区第三方 —— 绝不能 `npm install zcode`） |
| 源码构建 | `pnpm install --frozen-lockfile && pnpm --filter @zcode/cli... build`，需 Node 24.14.0 + pnpm 10.33.2 |
| ACP | **官方无 ACP 支持**（社区桥 `zcode-acp-server` 的存在反证这一点） |

## 3. 登录与鉴权（期望 ①）

两条官方路径（https://zcode.z.ai/en/docs 、docs/qa）：

1. **Coding Plan 订阅 OAuth（推荐给用户）**：`zcode login` 浏览器授权。OAuth 端点（repo `docs/zai-oauth-environment-origin.md`）：authorize `https://chat.z.ai/api/oauth/authorize`、token `https://zcode.z.ai/api/v1/oauth/token`、业务 token `https://api.z.ai/api/auth/z/login`、client_id `client_P8X5CMWmlaRO9gyO-KSqtg`。远程环境 `zcode login --no-browser` 可出登录 URL。中国区 zchat.z.ai / BigModel 同手机号套餐权益自动关联。
   - 凭据存本地**加密**凭据文件（密钥从本机环境派生）→ 跨机迁移脆弱（未证实可行）；但 daemon 与用户同机，直接复用本机登录态即可，无需注入。
2. **API Key（daemon 静默注入路径）**：写 `~/.zcode/v2/provider_config.json`（明文 apiKey，官方文档有完整格式），可经 `ZCODE_PERSONAL_PROVIDER_CONFIG_FILE` 指定路径、`ZCODE_DATA_BASE_DIR` / `HOME` / `USERPROFILE` 隔离整个数据域（`apps/zcode-cli/docs/usage/03-config.md`、`13-automation.md`）。

**模型指定没有 `--model` 旗标** → provider 配置文件提供 `defaultModelSelection.providerId/modelId`。2026-11-26 实测独立配置对官方 OAuth 账号可能导致模型回退；采用原生配置驱动模式：官方账号和自定义 provider 均由 ZCode / Magpie 配置，MEPERStudio 不生成临时配置。

**网关 override**：`api.type` 支持 `anthropic-messages` / `openai-chat-completions` / `openai-responses` + 自定义 `api.baseUrl`。注意 `ZCODE_BASE_URL` 是产品服务端点不是模型 API 地址；旧 `ZCODE_MODEL` / `ZCODE_API_KEY` 环境变量已不是通用覆盖入口。

## 4. 额度加成核实（期望 ②，官方 FAQ 原文）

- ❌ **ZCode 专属额外 1.5× 加成已结束**：Q2 原文 "ZCode's extra 1.5×: that was a limited-time promotion, and it has ended. ZCode now has the same quota as other tools that support the Coding Plan"（https://zcode.z.ai/en/docs/qa ）。
- ✅ **GLM-5.4 模型自带 1.5×**："1.5×: on the same plan quota, you can use 1.5 times as many tokens with GLM-5.4 as with GLM-5.3"，适用于现役 Coding Plan Lite/Pro/Max/Team，且不限于 ZCode。
- ✅ **核心价值成立**：一方 CLI 消耗 **Coding Plan 套餐额度**；对比 pi 等第三方 harness 按 token 计费（见 pi 文档 §5.1），"登录官方账号"路线对订阅用户是真省钱。
- 🎁 ZCode 独有：闲时任务免费（"Idle-time tasks run for free... without consuming plan quota"）。

## 5. Headless 驱动协议（期望 ③）

官方自动化文档：`apps/zcode-cli/docs/usage/02-cli-reference.md`、`13-automation.md`。

- **单发/流式（MVP 推荐）**：
  ```
  zcode --prompt "..." --cwd <abs> --mode plan|build|edit|yolo|guarded \
        --output-format text|json|stream-json --no-color
  ```
  - `--prompt` 缺省权限模式 `yolo`（NOTICE.md 明示）
  - `--output-format json` → `{sessionId, traceId, response, eventCount, projection}`；`stream-json` → NDJSON 事件流，`type:"result"` 行收尾；stdout 业务 / stderr 诊断分离
- **会话**：`--resume <sessionId>` / `--continue`；会话库 `~/.zcode/cli/db/db.sqlite`（`ZCODE_SESSION_DB_PATH` 可重定向）；`--prompt` 会等后台任务结束才退出（无内置超时，超时由启动器管 —— daemon 已有超时机制）
- **常驻 RPC（Phase 3 候选）**：`zcode-cli app-server`，stdio NDJSON（LF 分帧），协议文档 `docs/zcode-protocol.md`：V4 conversation topic（subscribe/snapshot/delta + `v4/command` 信封，commandId 幂等/CAS），命令含 createSession/sendText/stop/compact/retryTurn/forkAssistant —— 桌面 Host 即走此链路
- **禁止参数**（blocked args 依据）：`--print`、`--model`、`--settings`、`--max-turns`、`--allowed-tools`、`--permission-mode`（解析器不认）；daemon 应把 `--prompt/--output-format/--mode/--resume/--no-color` 也列入保留
- **退出码**：参数/运行错误 `1`；权限拒绝可能作为工具错误继续运行
- **Windows**：官方文档含 PowerShell 示例；win-x64/arm64 SEA 与桌面包都在发版清单

## 6. 再分发与许可

- Apache-2.0：允许商用再分发/自动下载部署（保留版权 + THIRD-PARTY-NOTICES；不用 ZCode 商标/品牌素材）。
- ⚠️ 官方 Terms（https://zcode.z.ai/en/terms ）禁止绕过本地安全限制、禁止自动化抓取 ZCode 内容，且"将 ZCode 服务提供给第三方，由你自行承担全部责任" —— **搬运官方 CDN 桌面包/内网发行包嵌入商业平台属 ToS 灰色地带，需法务确认**。
- **分发策略：自建不搬运** —— 我们自己从源码 CI 构建 SEA（锁定 v4.x tag + Node 24）走自有 release 源；绝不 npm 安装（冒名包风险）；"官方完整体验"（Browser Use/闲时任务）引导用户装官方桌面版。
- 遥测：官方文档未发现专门开关（未证实不存在）；后台请求类型逐项列于 NOTICE.md 第二节，可用 `ZCODE_HTTP_PROXY` / `ZCODE_NO_PROXY` / `ZCODE_AGENT_CA_CERT` 或防火墙管控；`doctor --json` 可自检。

## 7. 集成清单（协议已确认，沿用既有模式）

| # | 改动点 | 位置 | 说明 |
| --- | --- | --- | --- |
| 1 | 探测注册 `SILIECO_ZCODE_PATH`（默认命令 `zcode`；模型由外部配置） | `core/internal/daemon/agents_probe.go`、`config.go` | 极小 |
| 2 | **zcodeBackend**：`--prompt --output-format stream-json --resume` NDJSON 解析（仿 claude stream-json 后端） | `core/pkg/agent/zcode.go`（新建）+ `agent.go` 工厂 + `launch.go` blocked args | 主体工作 |
| 3 | 模型配置交给 ZCode / Magpie，MEPERStudio 不生成临时配置或修改 provider | daemon execenv（仿 pi models.json 模式） | 中 |
| 4 | 数据域隔离：`ZCODE_DATA_BASE_DIR` / `ZCODE_SESSION_DB_PATH` / `ZCODE_PERSONAL_PROVIDER_CONFIG_FILE` | daemon execenv | 小 |
| 5 | Skills 目录映射（zcode 原生位置待探明 —— 集成时实测） | `execenv/context.go`、`local_skills.go` | 小 |
| 6 | 模型定价：GLM 价格表前端已有（`packages/views/runtimes/utils.ts`），provider="zcode" 接入用量统计 | `pkg/agent/models.go` + views | 小 |
| 7 | 前端 ProviderLogo / runtimeDisplayName | `packages/views/runtimes` | 极小 |
| 8 | 登录 UX：onboarding 引导 `zcode login`（或贴 API Key → 托管 provider_config.json） | views/onboarding + desktop | 中 |
| 9 | 自动安装：Phase 1 引导官方安装；Phase 2 自建 SEA 走自有 release 源 | desktop `cli-release-asset` 模式复用 | 中 |

**预估**：MVP（探测 + stream-json 驱动 + 会话恢复 + BYOK/API key）1-2 周。

## 8. 未证实项与待验证

- 官方 ACP 支持（未发现，按无处理）
- 遥测开关是否存在（文档未发现）
- OAuth 凭据文件跨机迁移（加密密钥本机派生，按不可迁移处理）
- Linux 桌面包仅官网 CDN 提供、GitHub Releases 缺失（原因未证实）
- zcode 原生 skills 目录位置（集成时实测）
- GLM Coding Plan 端点 `https://open.bigmodel.cn/api/coding/paas/v4` / Anthropic 兼容 `https://open.bigmodel.cn/api/anthropic` 在 daemon 驱动场景下的实际行为（集成冒烟时验证）

## 9. 来源

- github.com/zai-org/ZCode（README、LICENSE/NOTICE.md、apps/zcode-cli/docs/*、docs/zcode-protocol.md、docs/zai-oauth-environment-origin.md）
- https://zcode.z.ai/en（docs、docs/qa、zsrc、terms）
- GitHub Releases / API（版本与资产清单）；npm registry（冒名包考证）
- 本仓库：`packages/views/runtimes/utils.ts`（GLM 定价）、`docs/runtime-pi-analysis.md`（pi 推进与网关方案）
