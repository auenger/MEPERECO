# 架构重构实施与验收记录

对应计划：[architecture-refactoring-plan.md](architecture-refactoring-plan.md)。本轮在 `aed4589` 基线之后实施，目标是把业务规则与 provider 特性从通用路径中收敛，而非全面改写产品、改变 API 语义或更换技术栈。

| 优先级 | 工作包 | 本轮实现 |
|---|---|---|
| P0 | Workflow Completion Policy | `core/pkg/workflowcontract/completion_policy.go` 统一版本、兼容解析、校验及就绪评估；人工以当前有效任务 `done` 为准；Agent 必须具备有效结果、已完成主调用及原有数据/产物验证；任务变更与 cron 调度记录幂等快照；API 提供实时及自动评估信息、能力目录。 |
| P0 | Workflow 核心业务测试 | 新增领域矩阵、审核证据、结果契约、重试/返工、状态转换与 PostgreSQL 事务回归。四种 Gate 均使用真实迁移表；两个并发批准只能成功一个，未结束的 Agent 审核不能推进。 |
| P1 | Runtime Registry 与能力模型 | `core/pkg/agent/registry.go` 集中内置 Runtime 工厂、模型发现、命令探测、参数约束、名称、上下文/技能目录及能力。后端输出能力目录，前端优先使用服务端注册信息；存储检查格式而非固定 provider 列表。新增模拟 adapter 的注册测试。 |
| P1 | Daemon 执行生命周期拆分 | `runtime_lifecycle.go` 集中 provider 准备、恢复、额外参数及 watchdog 钩子；`run_launch_env.go` 集中任务凭证与进程环境组装；`run_result.go` 集中终态归类；桌面安装 bundle 探测独立；保留原资源拥有者、清理顺序及 provider 协议实现。 |
| P1 | 统一状态流转服务 | `workflow_transition.go` 共用状态令牌、下一阶段解析、决策规划、事务更新及进入 Agent Stage 的逻辑。人工 Handler 下沉至服务；Agent 自动、Agent 审核、人工审核及混合审核接受均共用推进路径；返工和持续任务共用状态写入入口。 |
| P2 | SOP 设计器组件拆分 | `sop-designer-model.ts` 管模型与模板，`use-sop-designer.ts` 管控制器，`sop-policy-fields.tsx` 管完成规则和 Gate，`sop-designer-fields.tsx` 管技能、参与者及文档字段。保留 stable key、旧配置、输入输出契约等字段。主页面保留浏览、预览及组件编排。 |
| P2 | 配置及发布清单 | 新旧环境变量读取集中到 branding 边界；Go/Electron 共用状态目录测试向量；JSON/TSV 发布清单及内容寻址目录由构建器生成；CLI updater、Desktop、Web、Shell 与 PowerShell 消费清单并保留 404 旧协议回退。详见 [配置与发布契约](configuration-release-contract.md)。 |

## 核心领域约定

- `allowed_task_statuses` 只描述任务词汇，不是成功状态集合。取消、阻塞、未完成的有效任务不满足完成条件；没有有效必需任务时保持历史人工审批行为。
- 自动评估是就绪诊断，不是审核授权。定时和任务变更评估写入独立 `workflow_stage_readiness` 快照，状态令牌随阶段变化和返工失效。审批仍在加锁的当前证据上重新校验。
- 旧 `all_runs_terminal` 根据阶段执行类型解析，不批量改写存量运行。新增写入校验只支持明确策略与有效 cron 表达式；历史自由文本调度仍可展示错误并修正。
- 新运行时能力是静态契约，不代表设备实时在线。具体 CLI/账户可用性仍由 Daemon 探测决定。

## 可重复验收命令

```sh
# 仓库根目录
scripts/test-workflow-integration.sh
node --experimental-strip-types --test scripts/test-sop-model.mjs scripts/test-cli-contract.mjs
pnpm --filter @silieco/core --filter @silieco/views --filter @silieco/web --filter @silieco/desktop --filter @silieco/mobile typecheck
pnpm --filter @silieco/web build
make cli-release
python3 scripts/test-cli-release.py
docker compose -f docker-compose.selfhost.yml config -q
```

```sh
# core/
go test ./...
go test -race ./internal/service ./pkg/workflowcontract ./pkg/agent ./internal/daemon
```

集成脚本启动并清理独立 PostgreSQL 容器，不读取应用的生产数据库 URL，执行真实数据库迁移及 race 测试。CLI 安装测试使用临时 HOME 和本机 HTTP 服务，不修改正常用户配置。

## 本地验收结果（2026-11-26）

- Go 全量测试、关键包 race 测试、真实 PostgreSQL 四类审核/并发/返工测试：通过。
- 新建数据库全量迁移，0380 代表性旧 SOP、运行阶段及 Runtime Profile 的增量升级：通过；旧规则原样保留。
- Web、Desktop、Mobile、shared core/views 类型检查、Web 生产构建：通过（构建仍有既有 CSS `::highlight` 兼容警告）。
- SOP 草稿往返、稳定键、共享状态目录及清单消费者测试：通过。
- 六架构及兼容归档校验、macOS ARM64 真实 HTTP 安装/旧协议/损坏包回退、Linux AMD64 容器内运行、Self-host Compose 配置解析：通过。

## 发布前仍需验收的环境项

- 真实 provider 的全部账户/版本组合、外部 CLI 启动/恢复/取消、原生桌面 bundle 行为仍需各自环境烟测；本轮使用已有 fixture 与模拟 adapter，而非请求真实账户。
- SOP 浏览器端真实登录后的完整操作链路（从创建空间到发布、执行、审核、返工）没有以 UI E2E 验收；本轮通过共享模型测试、编译/构建及后端数据库真实业务事务。
- Windows x64/ARM64、macOS x64、Linux ARM64 原生安装/更新及 Electron 打包后更新需相应机器；本轮六架构均交叉构建与校验，macOS ARM64 实际 HTTP 安装与故障回退，Linux AMD64 在本机容器模拟环境完成解包、运行与配置。
- 仍保留兼容期全局环境变量 alias、旧 profile/协议以及旧前端 catalog fallback；不是全面删除 `SILIECO_*`。新增适配器通常只需要描述器和适配器；若提供独有 MCP、资源或协议行为仍需实现相应策略。
- 生产镜像滚动升级、真实历史数据库恢复演练、旧客户端/Daemon 的实际混合版本矩阵与生产发布验证，须在目标环境单独完成，不能以本地通过替代。

本轮代码未改变生产实例或发布目录。按原计划的严格完成标准，上述未验证项不能勾选为已通过。
