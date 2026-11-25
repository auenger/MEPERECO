# MEPER Studio

MEPER Studio 是面向 AI Agent 任务与团队协作的工作平台，提供 Web、桌面端和移动端入口，并通过 Go 后端及本地运行服务连接工作空间、会话与任务执行。

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

3. 在已配置环境变量的终端启动后端：

   ```powershell
   cd core
   go run ./cmd/server
   ```

4. 在项目根目录的另一个终端启动 Web：

   ```sh
   pnpm dev:web
   ```

   默认 Web 地址为 `http://localhost:3000`，后端端口为 `8080`。Web 开发脚本依赖 `sh`，Windows 下可使用 Git Bash 或 WSL。

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

部署参数参见 `.env.example` 和 Compose 文件。对外提供服务时，应配置域名、HTTPS、反向代理及实际使用的邮件、模型和存储服务。

## 命名约定

项目名称统一为 **MEPER Studio**。当前源码中的内部包作用域、Go 模块路径、部分命令和环境变量仍使用既有标识；开发和部署时请以实际配置为准。
