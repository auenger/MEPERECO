# MEPERStudio 邮箱服务配置

## 1. 默认公司邮件服务

Core Server 通过 SMTP 发送注册、登录验证码及工作区邀请邮件。公司邮件服务器已于 2026-11-26 使用 Core Server 的邮件发送实现完成实际投递测试，收件人确认收到验证码。

| 参数 | 默认配置 | 说明 |
| --- | --- | --- |
| `SMTP_HOST` | `mail.amaxgs.com` | 公司 SMTP 服务器 |
| `SMTP_PORT` | `587` | SMTP Submission 端口 |
| `SMTP_USERNAME` | `business_trip@amaxgs.com` | 邮箱认证账号 |
| `SMTP_PASSWORD` | 在私有 `.env` 中填写 | 邮箱登录密码或授权码 |
| `SMTP_FROM_EMAIL` | `business_trip@amaxgs.com` | 发件地址 |
| `SMTP_TLS` | `starttls` | 通过 STARTTLS 升级为加密连接 |
| `SMTP_TLS_INSECURE` | `false` | 验证服务器 TLS 证书 |
| `SMTP_EHLO_NAME` | 留空 | 默认使用机器主机名 |

默认配置已提供在项目根目录的 `.env.example` 中，其中不包含真实密码。在项目根目录的私有 `.env` 中配置如下：

```dotenv
SMTP_HOST=mail.amaxgs.com
SMTP_PORT=587
SMTP_USERNAME=business_trip@amaxgs.com
SMTP_PASSWORD='实际邮箱密码或授权码'
SMTP_FROM_EMAIL=business_trip@amaxgs.com
SMTP_TLS=starttls
SMTP_TLS_INSECURE=false
SMTP_EHLO_NAME=
RESEND_API_KEY=
SILIECO_DEV_VERIFICATION_CODE=
```

不要将真实邮箱密码提交到 Git；生产环境可以通过部署平台的密钥管理或环境变量注入。

## 2. 启动 Core Server

### 本地开发

首次部署时，从模板创建配置文件（已有 `.env` 则直接编辑，不要覆盖）：

```bash
cp .env.example .env
chmod 600 .env
```

填好邮箱密码、数据库连接及其他服务参数后，在项目根目录执行：

```bash
docker compose up -d postgres
make server
```

`make server` 会自动读取根目录 `.env` 并将变量传入 Core Server。也可以指定自定义配置文件：

```bash
make server SERVER_ENV_FILE=/path/to/server.env
```

如果直接使用 `go run ./cmd/server`，需要事先将相关配置注入终端环境。

### Docker 自托管

`docker-compose.selfhost.yml` 已将 SMTP 参数传入后端容器。配置项目根目录的 `.env` 后启动或重新创建后端服务：

```bash
docker compose -f docker-compose.selfhost.yml up -d
```

SMTP 配置在 Core Server 启动时读取，变更后需要重启或重新创建后端；不需要重新构建 Web 前端。

## 3. 注册与登录流程

- 默认登录：`POST /auth/password/login`，邮箱与密码正确后复用原有 JWT/Cookie 会话。
- 已有账号验证登录：`POST /auth/send-code`、`POST /auth/verify-code`，不再自动创建新用户。
- 注册、已有账号首次设置密码及忘记密码：通过 `POST /auth/password/send-code` 指定 `register` 或 `reset`，在 `POST /auth/password/verify-code` 中验证并获得短期一次性凭证，然后通过 `POST /auth/password/complete` 提交密码。新用户在最后一步才创建账号；重置密码保留原用户 ID。
- 已登录用户也可在个人设置中凭当前密码修改密码，或通过原邮箱验证码设置/重置。

验证码有效期 10 分钟，发送间隔 60 秒，最多允许 5 次错误验证；密码设置凭证有效期 10 分钟，只能使用一次且绑定验证目的。新密码至少 12 个字符、最大 72 字节，使用 bcrypt 哈希保存。密码登录使用数据库限流，即使未配置 Redis 也会生效。生产域名为 `https://studio.meper.cn`，部署时确保 TLS 终止及安全 Cookie 配置。

设置 `ALLOW_SIGNUP=false` 可关闭新用户注册，`ALLOWED_EMAIL_DOMAINS` 和 `ALLOWED_EMAILS` 可设置注册白名单，已有用户仍可登录。Google OAuth 以及无需验证邮箱的密码注册不启用。

## 4. ModelHub 参数映射

ModelHub 通过 SMTP 发送验证邮件，其本地代码位于 `AIHub/aihub/common/message/email.go`，MEPERStudio 使用以下对应参数：

| ModelHub | MEPERStudio |
| --- | --- |
| SMTPServer | SMTP_HOST |
| SMTPPort | SMTP_PORT |
| SMTPAccount | SMTP_USERNAME |
| SMTPToken | SMTP_PASSWORD |
| SMTPFrom | SMTP_FROM_EMAIL（省略时默认使用 SMTP_USERNAME） |

邮件服务实现位于 `core/internal/service/email.go`，注册与验证码处理位于 `core/internal/handler/auth.go`。

如果使用其他服务器，465 端口通常配置 `SMTP_TLS=implicit`；587 端口使用 `SMTP_TLS=starttls`。未认证的内部 SMTP 中继可以将账号、密码留空，但需指定实际发件人。

## 5. 排查邮件发送问题

- 启动日志应显示 SMTP relay、服务器端口、TLS 模式和发件地址。
- 无法连接时，检查 Core Server 所在机器或容器到 SMTP 服务器 587 端口的网络连通性。
- 认证失败时，核对邮箱用户名、密码及邮箱服务器的 SMTP 权限。
- 服务端已接收邮件但收件箱未出现时，检查垃圾箱或企业邮件网关。
- SMTP 优先于 Resend；生产环境未配置任何邮件服务时，验证码发送失败，不会将验证码输出到日志。
- 如果需要纯本地开发、不实际发送邮件，可以显式清空 `SMTP_HOST` 和 `RESEND_API_KEY`，并使用非生产环境；验证码会输出到服务端日志。
