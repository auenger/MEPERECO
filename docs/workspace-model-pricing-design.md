# Workspace 模型计价

状态：第一阶段已实现；2026-11-25 已部署到 yilia 测试服务。

## 数据模型与权限

- `0376_workspace_model_pricing`：每个 workspace、运行时 provider（规范化大小写）、原始模型名对应一个规则。
- 规则为 `custom`（四项美元/百万 Token 单价）、`alias`（明确映射至内置参考目录）、`free`（明确免费）；恢复默认删除该 workspace 的规则。
- GET `/api/workspaces/{id}/model-pricing`：成员可读，发现历史 Agent 使用过的模型并返回规则和参考价格目录；PUT 同一路由仅 owner/admin 可写，不允许 Agent Run token 写入。
- 新规则不改变 Token 原始数据和 provider 上报的费用。第一阶段采用当前配置重算全部历史估算，不进行账单结算/锁账；后续如需按合同生效日期计价，再增加价格版本。

## 服务端计算

`core/internal/handler/model_pricing.go` 按 workspace 一次加载规则，使用已有聚合行的 provider/model 与 `uncosted_*` Token 计算估计费用。实际费用仍来自 `cost_usd_ticks`，不会与估算重复计入。内置参考价格来自 Go catalog；未知模型不做相似度猜测，第三方渠道必须显式配置。保留未计价 Token 数、实际与估算金额、分类型成本及估算缓存节省。实际费用的分类型展示按参考权重分摊（总额才是供应商上报的实际费用）。

用量 Dashboard、Project 筛选、Agent/Runtime 日/小时统计和 Task 用量 API 均返回服务端 `pricing`；旧 API 仍保留原始字段，旧版服务兼容前端估算。新版客户端优先使用服务端结果，未计价显示待计价/已知最低金额，而非免费；成本图含缓存读取。

## 部署与验收

- yilia Go backend 已升级，数据库 0376 迁移。保留 `/opt/silieco-internal/releases/meper-model-pricing-20261125/before.dump` 和 `silieco-backend:rollback-meper-model-pricing-20261125`。
- 前端类型检查、计价专项 Go 单测、真实测试环境模型发现、Dashboard、Runtime、Task 查询通过；用隔离临时模型验证自定义/别名/免费/恢复默认和无效负价，测试后已删除。
- `Silieco Test` 原有 `codex/deepseek-flash`、`claude/workbuddy/deepseek-v4.1-flash[1m]` 保持待计价。未猜测或写入真实供应商价格。
- 本次未升级线上 Web 前端容器；Desktop 和共享 Web 源码已更新。后续增强点：计价渠道/配置 ID、历史价格版本、更多供应商账单数据的真实采集、管理审计记录。

## 币种配置（0377）

- Workspace 管理员可选择 USD（$）或 CNY（¥），CNY 需输入内部固定汇率（1 USD 对应人民币金额），并保存。默认 USD，系统不猜测实时汇率。
- `workspace_pricing_currency` 保存币种与汇率；GET `/api/workspaces/{id}/pricing-currency` 成员只读、PUT 管理员写入。
- 原始费用、内置价格及自定义规则在服务端统一存储 USD。人民币编辑携带币种及汇率快照，服务端转成 USD 后保存；汇率配置已变化时拒绝过期报价。切换展示币种不改变已有 USD 金额。调整汇率会改变折算的人民币金额，属于当前汇率下的参考估算，并非结账时的历史汇率。
- 设置页面单价、用量 KPI、Actor/Runtime/Task 费用、缓存节省、走势图坐标与提示、活动热图统一使用同一 Workspace 币种及换算比例，不做仅替换符号的伪换算。
