# 用量统计口径与 0384 修复

## 数据源

每次 Agent Run 的原始 Token 用量保存在 `agent_run_usage`。用量总览与运行时每日用量在指定时间范围内直接查询原始账本并关联 `agent_run` 与所属工作区，而不是依赖异步小时汇总表是否追赶完成。保留既有日期和 Run ID 索引、工作区过滤及私有 Agent 汇总规则。

旧小时汇总工作仍保留。迁移 0384 修复其从 1970 年初始水位每次仅前进一步的行为：在没有待处理用量的区间跳到下一条真实记录，遇到实际历史数据仍保持单日处理上限。没有删除或改写原始 Token 记录。对同一窗口多次汇总不重复累加。

## 运行来源

`agent_run_usage_kind(chat_session_id, task_id)` 定义统一分类：有 Chat Session 的 Run 属于 `chat`（含群聊）；否则有关联 Task 的 Run 属于 `task`；其余为 `other`。工作区用量的六个查询都支持可选 `run_kind`（all/chat/task/other）。项目筛选对 Chat 使用会话项目，对 Task 使用任务项目，非项目会话只进入全部项目。运行次数不是独立 Task 或对话对象数；同一个对象的多次执行分别计数。旧服务端不会向新客户端宣告筛选能力，客户端隐藏此筛选。

## 空间模型计价

空间的 `workspace_model_price` 只接受明确的 custom、alias、free 配置。无配置的模型不自动套用参考价，显示「未配置」且保留所有 Token。原始 provider 费用仍保留在账本中；对已配置模型继续沿用实际费用优先于估算的规则。明确配置为 free 且无 provider 实际费用时可展示真实的零费用。混合已计价与未计价模型时，总费用标记为已知下限，并显示未计价 Token 数。未获取用量记录时显示 `—`，不将其误认作零消耗或免费。

## 验证

```sh
scripts/test-usage-integration.sh
node --experimental-strip-types --test scripts/test-usage-cost-model.mjs
cd core && go test ./...
```

集成测试在独立 PostgreSQL 数据库中验证普通聊天、群聊、Task、其他来源，项目归属筛选、运行数及失败数、历史水位追赶和重复汇总，不触碰生产数据。
