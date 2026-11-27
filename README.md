# MEPERStudio

MEPERStudio 是面向 AI Agent 任务与团队协作的工作平台，提供 Web、桌面端和移动端入口，并通过 Go 后端及本地运行服务连接工作空间、会话与任务执行。

## 项目结构

| 目录 | 内容 |
| --- | --- |
| `apps/web` | 基于 Next.js 和 React 的 Web 应用 |
| `apps/desktop` | 基于 Electron 的桌面客户端 |
| `apps/mobile` | 基于 Expo 和 React Native 的移动客户端 |
| `core` | Go 后端、命令行工具、本地运行服务和数据库迁移 |
| `packages/core` | 前端共享核心模块 |
| `packages/ui` | 共享 UI 组件 |
| `packages/views` | 共享视图模块 |
| `packages/eslint-config`、`packages/tsconfig` | 共享开发配置 |
| `docker` | 容器相关配置 |
| `scripts` | 项目辅助脚本 |

## 技术栈

- 前端：TypeScript、React、Next.js、Tailwind CSS。
- 桌面端：Electron、electron-vite。
- 移动端：Expo、React Native。
- 后端：Go、PostgreSQL、pgvector。
- 工程管理：pnpm Workspace、Turborepo、Docker Compose。

## 本地开发

环境要求：Node.js 22、pnpm 10.28.2、Go 1.26.1，以及 Docker Compose 或可用的 PostgreSQL 数据库。

1. 安装依赖并复制环境配置：

   ```powershell
   pnpm install
   Copy-Item .env.example .env
   ```

   根据运行环境编辑 `.env`，配置数据库连接、JWT 密钥和所需服务参数。生产环境应使用独立的数据库密码及 JWT 密钥。

2. 启动开发数据库：

   ```powershell
   docker compose up -d postgres
   ```

3. 在项目根目录运行 `make server`（自动加载根目录 `.env`，也可通过 `SERVER_ENV_FILE` 指定配置文件）。Windows 或其他不使用 Make 的环境，可先将配置注入终端环境，再启动后端：

   ```powershell
   cd core
   go run ./cmd/server
   ```

4. 在项目根目录的另一个终端启动 Web：

   ```sh
   pnpm dev:web
   ```

   默认 Web 地址为 `http://localhost:3000`，后端端口为 `8080`。Web 开发脚本依赖 `sh`，Windows 下可使用 Git Bash 或 WSL。

## 邮箱验证、密码注册与登录

Web、Desktop 和 Mobile 默认支持邮箱和密码登录，也保留已有账号的邮箱验证码登录。新用户通过邮箱验证码验证后设置密码，才完成账号创建（遵循 `ALLOW_SIGNUP`、`ALLOWED_EMAIL_DOMAINS` 和 `ALLOWED_EMAILS`）；已有无密码账号可通过邮箱验证设置密码。验证码 10 分钟内有效，同一邮箱发送间隔 60 秒，最多允许 5 次错误验证；验证后的一次性密码设置凭证有效期为 10 分钟。

公司邮件服务默认采用 `mail.amaxgs.com:587`（STARTTLS），发件人与认证账号均为 `business_trip@amaxgs.com`，已通过真实邮件投递测试。详细的 SMTP 参数、Core Server 启动方式、Docker 部署和排查指南见 [邮箱服务配置文档](docs/email-configuration.md)。

真实邮箱密码仅配置在未纳入 Git 的 `.env` 或部署环境的密钥中。修改后运行 `make server`，或重新创建自托管后端容器使配置生效。旧账号仍可使用原邮箱验证码登录，密码设置和重置不改变已有用户身份或工作空间。

## 常用命令

以下命令在项目根目录执行：

| 命令 | 用途 |
| --- | --- |
| `pnpm dev:web` | 启动 Web 开发服务 |
| `pnpm dev:desktop` | 启动桌面端开发环境 |
| `pnpm dev:mobile` | 启动移动端开发环境 |
| `pnpm build` | 构建前端工作区，默认不包含移动端 |
| `pnpm typecheck` | 检查 TypeScript 类型，默认不包含移动端 |
| `pnpm lint` | 执行代码检查，默认不包含移动端 |

Go 后端的构建和测试在 `core` 目录执行：

```sh
go build ./...
go test ./...
```

## 容器部署

配置好根目录 `.env` 后，通过构建覆盖文件从当前源码构建并启动数据库、后端和 Web：

```sh
docker compose -f docker-compose.selfhost.yml -f docker-compose.selfhost.build.yml up -d --build
```

正式域名为 `https://studio.meper.cn`，部署时通过同源反向代理转发 Web、API、认证及 WebSocket；现有本地开发地址仍可通过环境变量或客户端配置覆盖。

部署参数参见 `.env.example` 和 Compose 文件。对外提供服务时，应配置域名、HTTPS、反向代理及实际使用的邮件、模型和存储服务。

正式版本发布（版本字段同步、源码推送、构建、备份、部署、下载验证和回滚）统一参照 [MEPERStudio 正式版本发布标准流程](docs/release-runbook.md)。

## 命名约定

项目名称统一为 **MEPERStudio**。对外 CLI 命令统一为 `meperstudio`，新配置使用 `MEPERSTUDIO_*`。内部包作用域、Go 模块路径、数据库标识、旧协议和 Skill ID 保留兼容。旧 `SILIECO_*` 环境变量仍可使用；两者同时设置时新名称优先。新安装默认使用 `~/.meperstudio`，已有 `~/.silieco` 配置目录继续使用，不搬迁正在运行的 Daemon 状态。


## CLI 与 Daemon 安装及发布

创建 Space 不需要连接 Daemon。桌面端内置 CLI 并自动管理本地 Daemon；Web 用户可以在运行时页面连接远程机器，或者到 [下载页](https://studio.meper.cn/download#cli) 下载安装包。

macOS / Linux：

```bash
curl -fsSL https://studio.meper.cn/cli/install.sh | bash
meperstudio setup self-host --server-url https://studio.meper.cn --app-url https://studio.meper.cn
```

Windows PowerShell：

```powershell
irm https://studio.meper.cn/cli/install.ps1 | iex
meperstudio setup self-host --server-url https://studio.meper.cn --app-url https://studio.meper.cn
```

`make cli-release VERSION=1.0.0` 从当前 Go 源码交叉构建 macOS、Linux 和 Windows 的 AMD64 / ARM64 安装包，输出到 `apps/web/public/cli/`，同时生成 `checksums.txt` 和 `version.txt`。旧安装包名称一并生成以兼容已有桌面客户端。发布新版本时使用新的 `VERSION`；自托管 Web Docker 镜像构建时自动执行该流程，不需要手动复制二进制文件。CLI 更新与桌面端自动恢复使用同一发布路径，其他部署可通过 `MEPERSTUDIO_CLI_DOWNLOAD_BASE_URL` 指定自己的 `/cli` 路径。
