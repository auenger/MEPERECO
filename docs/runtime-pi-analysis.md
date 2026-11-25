# pi 作为默认 Agent 运行时 — 调研与推进记录

> 调研日期：2026-11-24 · 结论：**可行，pi 已在代码中完整支持，剩余工作集中在"自动安装 + 平台模型网关 + 默认值"三个环节**
> 状态：已批准推进（Phase 0 → 3，见 §8）

---

## 1. 背景与目标

当前 MEPER Studio 的 Agent 运行时依赖用户自行安装 Claude Code（`claude`）或 Codex（`codex`）等商业 CLI，存在安装摩擦，且模型凭据靠用户在 Agent 设置里手动贴 `custom_env`（如 `ANTHROPIC_API_KEY`）。目标：

1. 用轻量开源运行时 **pi** 作为**默认运行时**（保留 claude/codex 等为可切换选项）；
2. 桌面端**自动安装**运行时，用户零手动操作；
3. 搭配**平台模型网关**，实现模型接入全自动（用户无需自备 key）。

## 2. pi 事实卡片（2026-11-24 一手核实）

| 项 | 事实 |
| --- | --- |
| 仓库 | github.com/earendil-works/pi（原 badlogic/pi-mono，同一仓库转移改名，旧 URL 重定向） |
| 作者/维护 | Mario Zechner（badlogic），现归 Earendil Works；npm maintainers：badlogic、mitsuhiko（Armin Ronacher）、rwachtler |
| 许可 | **MIT**（可自由捆绑、自动下载、修改再分发，保留版权声明即可） |
| 热度 | 120,164 stars / 15,445 forks（GitHub API），TypeScript |
| 运行要求 | Node >= 22.19.0（npm 形态）；**官方另提供 Bun 编译的多平台单文件二进制**（免 Node）与 Nix 构建 |
| 安装 | `npm i -g @earendil-works/pi-coding-agent`（⚠️ **旧 scope `@mariozechner/pi-coding-agent` 已弃用**，末版 0.73.1；新版 1.6.0）；或 `curl -fsSL https://pi.dev/install.sh \| sh` / Windows `irm https://pi.dev/install.ps1 \| iex` |
| 遥测 | 默认仅版本检查 + 安装匿名 ping；**`PI_OFFLINE=1` 全部关闭**（含包更新检查）；`PI_SKIP_VERSION_CHECK=1` 单独跳过版本检查 |
| 配置目录 | `~/.pi/agent/`；`PI_CODING_AGENT_DIR` 可整体托管到平台指定路径 |

**四种运行模式**（daemon 集成面）：

- `-p` / `--print`：单次回答后退出；
- `--mode json`：JSON lines 事件流输出（daemon 当前使用）；
- `--mode rpc`：stdin/stdout 双向 JSONL RPC —— 适合长驻任务（Phase 3 候选）；
- SDK（`createAgentSession` 等）：可进程内嵌入 Node 宿主。

**内置工具**：`read, bash, edit, write, grep, find, ls`（`--tools` 可做只读白名单）。无内置权限系统/审批弹窗 —— 平台侧需自行隔离（官方 containerization 文档：micro-VM / Docker）。

## 3. 代码库现状：pi 已是完整支持的一等运行时

| 层 | 现状 | 代码位置 |
| --- | --- | --- |
| 探测 | `SILIECO_PI_PATH` → `agents["pi"]`（默认命令名 `pi`，`SILIECO_PI_MODEL` 设默认模型）；全库共 17 种运行时统一探测 | `core/internal/daemon/agents_probe.go:168` |
| 执行后端 | `piBackend`：`pi -p --mode json --session <file>` 非交互 JSON 模式；流式解析 `agent_start/turn_start/message_update(text|thinking_delta)/tool_execution_*/turn_end(usage 含 cache)/error/auto_retry_end`；会话恢复 = 复用 session 文件路径；`--provider/--model` 来自 "provider/model" 字符串；Windows 专用 invocation（PS1 重写） | `core/pkg/agent/pi.go`、`pi_invocation_windows.go` |
| 会话托管 | 平台自管会话路径 `~/.silieco/pi-sessions/*.jsonl`，不依赖 pi 默认目录 | `core/pkg/agent/pi.go:542` |
| 参数防护 | `piBlockedArgs`：`-p/--mode/--session` 为 daemon 保留参数，用户 custom_args 不可覆盖 | `core/pkg/agent/launch.go:62` |
| Skills | 项目级 `{workDir}/.pi/skills/`、全局 `~/.pi/agent/skills` | `core/internal/daemon/execenv/context.go:376`、`local_skills.go:155` |
| Runtime config | pi 已纳入 per-provider 配置分支 | `execenv/runtime_config.go:223`、`runtime_config_sections.go:546` |
| 凭据注入 | per-agent `custom_env` 机制（用户在 Agent 设置 UI 手动贴 key，如 `ANTHROPIC_API_KEY`），关键内部变量有 blocklist | `core/internal/daemon/daemon.go:5406` |
| 自动安装模板 | silieco CLI 的平台压缩包下载安装机制（版本化 asset 选择、legacy 回退）—— pi 自动安装可直接复用此模式 | `apps/desktop/src/main/cli-release-asset.ts` |
| 实测标注 | 代码注释 "Verified against Pi 0.67.2 (SILI-5392)" | `core/pkg/agent/pi.go:521` |

> 含义：**Phase 0 所需的执行链路已 100% 存在**，pi 现在就能作为 provider 跑任务（需用户 PATH 上有 `pi` 并配好 key）。

## 4. oh-my-pi（omp）考证

- **真实存在**：github.com/can1357/oh-my-pi（Stencil Labs / can1357），README 自述 "a fork of pi-mono" —— pi 的 **MIT 商业增强 fork**，32,410 stars。
- 形态：Bun >= 1.3.14（不依赖 Node）或预编译单文件二进制（**Windows 原生，含 arm64**）；npm `@oh-my-pi/pi-coding-agent`（~35MB + Rust 原生库：内嵌 bash 5.3、58 个进程内 coreutils、LSP/DAP/PTY/桌面控制）。
- 能力：`-p` / `--mode rpc` / `omp acp`（ACP JSON-RPC）/ Node SDK（与 pi 同一套 `createAgentSession` API 面）；60+ providers、31 内置工具；网关走 `~/.omp/agent/models.yml`（13 种协议含 `anthropic-messages`，baseUrl + apiKey 环境变量名 / `!command` / `auth: none`）；原生 `litellm` provider（`LITELLM_BASE_URL`）。
- ⚠️ 风险：体量大、发版极快（17.x/19.x）、对外遥测行为未文档化（npm 依赖含 OTel SDK 与 crash_handler，是否外发**未证实**，采纳前需自行审计）。

**决策**：短期用 pi 本体（小、稳、文档全）；omp 入观察名单。两者 SDK/会话面兼容，后续切换成本低。

## 5. 模型接入方案：平台网关"一把平台 key"

> **【网关选型结论 2026-11-24】**「Magpie 方案」考证 = yetone/magpie（MIT，40.7k stars，本地模型网关 + Agent 配置切换器，90+ agent 显式适配含 pi）。定性：**接入模式正确（baseUrl + apiKey 统一入口），但 Magpie 本体不适合做平台网关** —— 它是个人本机设计（无多租户/账户/对账体系、上游 key 存用户本地）、不收外部 PR 治理不可控。Magpie 可作为「高级用户本机 BYOK 聚合」写进用户文档（与 pi 官方兼容）。
>
> **平台侧网关推荐**：① **LiteLLM Proxy**（首选 PoC）— MIT、协议覆盖最全（原生 OpenAI 全家桶 + Anthropic `/v1/messages` + Responses/Gemini）、virtual keys/teams/budget/审计生产级；代价 Python + Postgres 运维最重。② **gpt-load**（轻量降级选项）— MIT、Go 单二进制、原生 `/v1/messages`、多渠道多凭据、中文文档；计量/多租户弱于 LiteLLM。③ new-api 体验最好但 **AGPL-3.0** 需法务确认；one-api 已停滞（~10 个月）不采用；uni-api 体量小仅参考。**不自研薄代理**（重复造轮子；平台侧只做 token 签发 + 审计落库 + 对账聚合）。
> 未证实项：one-api 原生 `/v1/messages` 对外服务；gpt-load 客户端侧 per-user key 细粒度；new-api AGPL 在我司场景的义务边界（需法务）。

### 5.1 三种模式

1. **BYOK**（现状）：用户在 Agent 设置贴 key 进 `custom_env`。可行但有摩擦 —— 作为高级选项保留。
2. **平台网关**（推荐，全自动的核心）：平台部署同时暴露 **Anthropic `/v1/messages`** 与 **OpenAI `/v1/chat/completions`** 协议的网关（LiteLLM Proxy 原生支持，或自研薄代理 —— 服务端已有 `core/pkg/llm` client 先例：`SILIECO_LLM_API_KEY` / `SILIECO_LLM_BASE_URL`，OpenAI 兼容）。运行时统一指向网关，平台只签发一把 per-user/per-run token，集中计费/审计/限流。
3. **订阅 OAuth**：⚠️ **坑（pi 官方文档证实）**——订阅（Claude Pro/Max、ChatGPT Plus 等）经第三方 harness 消耗**按 token 计费，不走套餐额度**。"用户已有订阅 = 零模型成本"不成立。自动化场景必须走 API key + 网关。

### 5.2 pi 侧配置（网关指向方式）

pi 凭据解析顺序：CLI `--api-key` > `~/.pi/agent/auth.json`（0600，支持 `!command`/环境变量名/字面量）> 环境变量 > models.json。

自定义网关在 `~/.pi/agent/models.json`（或 `PI_CODING_AGENT_DIR` 托管目录下）：

```json
{
  "providers": {
    "meper-gateway": {
      "baseUrl": "https://<平台网关>/anthropic",
      "api": "anthropic-messages",
      "apiKey": "$SILIECO_GATEWAY_TOKEN",
      "models": [
        { "id": "claude-sonnet-5-5", "name": "Claude Sonnet 5.5" },
        { "id": "glm-5.3", "name": "GLM 5.3" }
      ]
    }
  }
}
```

daemon 侧配合：运行任务前预写该文件（或 `PI_CODING_AGENT_DIR` 指向平台准备的目录）+ 注入 `SILIECO_GATEWAY_TOKEN`（custom_env 层已支持）+ `--provider meper-gateway --model ...`（`buildPiArgs` 已支持 provider/model 拆分）。

### 5.3 各运行时网关指向能力（核实矩阵）

| 运行时 | Anthropic 兼容网关 | OpenAI 兼容网关 | 方式 |
| --- | --- | --- | --- |
| **pi** | ✅ | ✅ | models.json `baseUrl` / 扩展 `registerProvider` |
| oh-my-pi | ✅ | ✅ | models.yml；原生 `litellm` provider |
| opencode | ✅ | ✅ | opencode.json `providers.*.settings.baseURL` |
| goose | ✅（`ANTHROPIC_HOST`） | ✅ | 纯 env `OPENAI_HOST`+`OPENAI_BASE_PATH` |
| Qwen Code | ✅ | ✅ | env `ANTHROPIC_BASE_URL` / `OPENAI_BASE_URL` |
| Claude Code（对照） | ✅ | — | env `ANTHROPIC_BASE_URL` + `ANTHROPIC_AUTH_TOKEN` |
| gemini-cli | ❌ 仅 Gemini 协议 | ❌ | 网关需实现 Gemini API |

## 6. 备选运行时对比结论

| 候选 | 许可 | 结论 |
| --- | --- | --- |
| **pi** ✅ | MIT | **推荐默认**：MIT、120k stars、`--mode json/rpc` 双协议、网关一等支持、遥测可全关（`PI_OFFLINE=1`）、官方单文件二进制 |
| oh-my-pi ⏳ | MIT | 观察名单：功能更强、Windows 原生单二进制；遥测需自审 |
| opencode | MIT | 生态最大（225.9k stars）但 2026-10-17 披露过 Console 安全事件；"opencode" 名称有商标争议史；需安全评估后再议 |
| goose | Apache-2.0 | 最中立（Linux 基金会 Agentic AI 基金会治理）但二进制 90–105MB；2026-08-27 后 releases 空窗约 3 个月（需复核） |
| crush | FSL-1.1-MIT | ❌ source-available 非 OSI、默认遥测开、无 JSON 输出 |
| gemini-cli | Apache-2.0 | ❌ 仅 Gemini 协议；免费个人 OAuth 已迁出（2026-06-18 → Antigravity CLI） |
| aider | Apache-2.0 | ❌ 停滞约 15 个月；PyPI 有 2 个未修复 CVE；无 JSON 输出 |
| amp | 专有 | ❌ 不可再分发 |

**中国区供应商兼容性**：pi/omp 内置 ZAI/GLM、MiniMax、Kimi、Xiaomi、Qwen 等 provider；Qwen Code 对阿里系接入最好（`ANTHROPIC_BASE_URL` 直连）。

## 7. 风险清单

1. **协议漂移**：`piBackend` 硬编码 pi 的事件协议与保留参数（基于 Pi 0.67.2 实测）。pi 升级必须过 daemon 回归测试；建议 pin 版本 + 自动化冒烟（`pi -p --mode json` 冒烟用例）。
2. **pi 无内置权限系统**（无审批弹窗）—— 平台侧需要隔离（容器/micro-VM；官方 `docs/containerization.md` 提供三种模式）。长期与 Runtime 沙箱规划合并考虑。
3. **网关是新的长期基础设施**：计费、限流、上游 key 池管理、可用性、审计。凭证脱敏可复用 `core/pkg/redact`。
4. **中国网络**：pi.dev / npm 官方源直连不稳 —— 自动安装必须走自有 release 源/镜像（同 silieco CLI 的分发策略）。
5. **npm scope 已迁移**：所有自动化安装脚本必须使用 `@earendil-works/pi-coding-agent`，旧 scope 包已弃用。
6. **会话格式**：daemon 将 session 文件路径当 opaque SessionID 用（`pi.go:189-197`）—— 若未来引入 pi 原生 session 树/分支能力（`--fork`），需同步改造。

## 8. 落地路径

```
Phase 0（1-2 天，几乎零代码）— 已批准
  □ 默认 provider 切换为 pi（Agent 创建模板/onboarding 推荐）
  □ BYOK 全链路验证：用户 PATH 装 pi + custom_env 贴 key → 跑通任务
  □ 记录 pi 版本基线（当前实测 0.67.2）

Phase 1（约 1 周）
  □ 桌面端自动安装：复用 cli-release-asset 模式下载 pi 单文件二进制
    → 安装到 ~/.silieco/bin → 设 SILIECO_PI_PATH
  □ 自有镜像/release 源分发（规避 pi.dev/npm 直连问题）
  □ PI_OFFLINE=1 + PI_CODING_AGENT_DIR=~/.silieco/pi 托管配置目录
  □ onboarding Step 3 从"引导安装"改为"自动安装 + 进度展示"

Phase 2（2-4 周）
  □ 平台网关：LiteLLM 部署或自研代理（复用 pkg/llm 模式）
    暴露 /v1/messages + /v1/chat/completions
  □ per-user/per-run token 签发 + 计量/限流/审计
  □ daemon 预写 pi models.json（baseUrl=网关, apiKey=平台 token）
  □ 用户零配置全自动：装好 APP → 选模型 → 任务直接跑

Phase 3（可选）
  □ --mode rpc 长驻运行时（长任务免反复 spawn）
  □ oh-my-pi 评估（Windows 原生二进制 / LSP / 子代理）
  □ 容器化隔离（官方 containerization 文档三模式）
```

## 9. 参考来源

- github.com/earendil-works/pi（README、docs/providers.md、docs/models.md、docs/custom-provider.md、docs/json.md、docs/rpc.md、docs/containerization.md）
- npm registry：`@earendil-works/pi-coding-agent`、`@oh-my-pi/pi-coding-agent`、`@mariozechner/pi-coding-agent`（弃用公告）
- github.com/can1357/oh-my-pi；api.github.com（stars/license 元数据）
- opencode.ai/docs、goose-docs.ai、github.com/charmbracelet/crush、qwenlm.github.io/qwen-code-docs
- docs.litellm.ai（Anthropic 统一端点）、code.claude.com/docs（Gateways）、models.dev
- 本仓库代码：§3 所列文件（以实际代码为准，本文只做索引）
