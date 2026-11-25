# ZCode（智谱官方 CLI）集成调研 — 记录

> **【决策 2026-11-24】不推进 ZCode 集成。** 原因：ZCode 专属 1.5× 额度加成是限时活动且已结束，失去核心吸引力；资源集中投入 pi 自动化（见 `docs/runtime-pi-analysis.md`）。本文保留作历史调研记录。
>
> 调研日期：2026-11-24 · 当时结论：可集成。登录官方账号 ✅、Coding Plan 套餐额度 ✅（注意：ZCode 专属 1.5× 加成已结束）、daemon headless 驱动 ✅（stream-json 一等支持）
> 关联文档：`docs/runtime-pi-analysis.md`（pi 默认运行时推进，两者分发/网关策略并轨）

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

**模型指定没有 `--model` 旗标**（官方禁止）→ 只能经 provider 配置文件 `defaultModelSelection.providerId/modelId`（如 `glm-5.4`）。daemon 每任务预写配置即可（与 pi 的 models.json 预写同模式）。

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
| 1 | 探测注册 `SILIECO_ZCODE_PATH`（默认命令 `zcode`）+ `SILIECO_ZCODE_MODEL` | `core/internal/daemon/agents_probe.go`、`config.go` | 极小 |
| 2 | **zcodeBackend**：`--prompt --output-format stream-json --resume` NDJSON 解析（仿 claude stream-json 后端） | `core/pkg/agent/zcode.go`（新建）+ `agent.go` 工厂 + `launch.go` blocked args | 主体工作 |
| 3 | 每任务 provider_config.json 预写（`defaultModelSelection` + 网关/baseUrl + apiKey） | daemon execenv（仿 pi models.json 模式） | 中 |
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
