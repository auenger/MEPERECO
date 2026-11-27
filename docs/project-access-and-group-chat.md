# 项目权限与群聊绑定

## 产品规则

- 空间邀请、空间成员和项目层级保持不变；空间成员资格是项目访问的前提。
- Owner / Admin 默认可以管理和访问空间内所有项目；普通员工需要显式项目授权。
- 迁移 0385 为已有普通成员保留当时的项目访问权，避免升级中断；此后新加入的普通成员默认无项目授权。普通员工创建项目时，创建人自动获准访问。
- 授权可从员工详情「项目权限」或项目详情「更多 → 项目权限」修改。撤销授权后，后端拒绝访问项目及其任务/资源/绑定群聊，实时通知相关客户端重新读取权限。
- 未归属项目的任务、未绑定项目的群聊仍按照既有空间/群成员规则运行。
- 新建群聊可选择项目；已有未绑定群聊允许创建人或空间管理员绑定（需等待当前运行结束）。一旦绑定，不能清除或换绑；新增群成员必须拥有相应项目权限。
- 删项目不应把受保护的任务或群聊自动变成空间公开数据，因此项目仍有任务、聊天或自动化时禁止直接删除，可将项目设为完成/暂停等状态。

## API

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| GET | /api/projects/member-access/{userId} | 管理员查询员工可访问的项目 ID |
| GET | /api/projects/{id}/members | 管理员查询显式授权的员工 ID |
| PUT | /api/projects/{id}/members/{userId} | 管理员增删授权，JSON: `{"granted":true}` 或 `{"granted":false}` |
| POST | /api/chat/sessions | 群聊创建时传 `kind: "group", project_id: "..." ` |
| PATCH | /api/chat/sessions/{sessionId} | 未绑定群聊首次绑定项目，传 `project_id` |

授权记录使用 `project_member(project_id, member_id)`，不是独立账号 ID。员工离开空间后关联记录被清理，重新邀请不会恢复旧权限。

## Agent 工作目录

群聊的项目 ID 储存在现有 `chat_session.project_id`。群聊的 Agent run 领取时使用现有项目资源解析和项目上下文生成逻辑，与项目任务共用工作目录机制。项目绑定了与该 Agent 所用 daemon 匹配的本地工作目录且选择原地执行时，`pwd` 为该目录；仓库 worktree 模式则位于工作树内。没有适用目录资源时仍使用常规运行工作目录，不凭空构造本地路径。绑定群聊中通过 Agent run token 创建的任务默认归入该项目。

项目权限是应用层访问控制，并不等同于 Agent 所在操作系统的文件沙箱；已启动的运行不会被撤权操作强制停止。公开对象存储 URL 和已签发、尚未到期的文件临时授权链接不受应用内项目权限即时撤销控制，部署需使用私有对象存储才能保护文件内容。

## 验证

- `MEPERSTUDIO_PROJECT_TEST_DATABASE_URL` 指向隔离 PostgreSQL 时运行 `go test ./pkg/db/generated ./internal/handler -run 'TestProjectAccessPostgres|TestFixedProjectGroupChatPostgres'`：覆盖迁移、授权/撤销、成员移除重邀、跨空间拒绝、附件、群聊访问及不可换绑。
- `go test ./internal/daemon -run TestProjectChatWorkingDirectory`：验证群聊运行项目目录解析及实际 `pwd`。
