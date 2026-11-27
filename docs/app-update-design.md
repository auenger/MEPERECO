# MEPERStudio 桌面端更新方案

> 更新：2026-11-26 · 发布基线：1.0.0 · 客户端、打包配置、服务端文件服务均已实现；正式部署与安装包发布待完成
> 决策方向：**不依赖 GitHub / 公网 CDN，更新完全走部署自己的服务器；服务地址在 App 内可切换，更新跟随当前服务地址**

---

## 1. 背景与问题

| 问题 | 现状 |
| --- | --- |
| 自动更新空转 | 原先只有客户端逻辑；现已补齐 generic provider、安装包更新元数据和自托管文件服务 |
| 外部依赖不可用 | 原 GitHub 下载不稳定；正式域名确定为 studio.meper.cn，发布前需验证 HTTPS 与反向代理 |
| 网络环境 | 目标用户网络直连 GitHub/公网 CDN 不稳定，自有服务器（yilia.me:6688）可达且已承载 API |

**决策**：更新源从 runtime config（部署的 API 服务器）推导，与 API/WS 同源。放弃 GitHub Releases、放弃公网 CDN。

## 2. 客户端设计（已实现）

### 2.1 更新源推导

`apps/desktop/src/main/index.ts`（app.whenReady 内，runtime config 加载后）：

```
feedUrl = <apiUrl>/updates/desktop        （正式：https://studio.meper.cn/updates/desktop）
setupAutoUpdater(() => mainWindow, feedUrl)
```

`src/main/updater.ts`：

```
autoUpdater.setFeedURL({ provider: "generic", url: feedUrl })
```

- `apiUrl` 未配置时（理论不发生，DEFAULT_RUNTIME_CONFIG 有默认值）使用安装包内的默认 generic feed。
- 服务端未部署该路径时：检查 404，错误被现有 handler 吞掉（console.error），不影响使用；部署后即刻生效，无需改客户端。

### 2.2 既有行为（未改动，沿用）

| 行为 | 值 |
| --- | --- |
| 自动下载 | `autoDownload = true`（update-available 即后台静默下载） |
| 安装时机 | `autoInstallOnAppQuit = true`（退出时安装）；下载完成后右下角弹提示（`update-notification.tsx`）可手动触发 |
| 检查节奏 | 启动后 5s 首查 + 每小时轮询；设置页可关自动更新（`updater-preferences.ts`，持久化在 userData） |
| 手动检查 | `updater:check` IPC（设置页 Updates tab 的 Check for updates 按钮） |

### 2.3 Channel 矩阵（沿用现有逻辑）

| 平台 | channel | 元数据文件 |
| --- | --- | --- |
| Windows x64 | latest | `latest.yml` |
| Windows arm64 | latest-arm64（`updater.ts` 设置） | `latest-arm64.yml` |
| macOS Apple Silicon | latest（mac 默认） | `latest-mac.yml` |
| macOS Intel | latest-x64（`configureMacX64UpdateChannel`） | `latest-x64-mac.yml` |
| Linux x64 | latest | `latest-linux.yml` |
| Linux arm64 | latest | `latest-linux-arm64.yml` |

## 3. 服务端与发布契约（已实现，待部署）

Go 后端提供公开 GET / HEAD 文件服务，Web 同源代理转发此路径。Docker 将宿主机目录（默认 `./data/updates/desktop`，可通过 `MEPERSTUDIO_DESKTOP_UPDATE_HOST_DIR` 修改）只读挂载至容器；独立运行时使用 `MEPERSTUDIO_DESKTOP_UPDATE_DIR` 配置目录。

```text
https://studio.meper.cn/updates/desktop/
├── latest.yml
├── latest-arm64.yml
├── latest-mac.yml
├── latest-x64-mac.yml
├── latest-linux.yml
├── latest-linux-arm64.yml
└── meperstudio-desktop-1.0.0-<platform>-<arch>.<ext>
```

- `electron-builder.yml` 配置 generic provider，生成 `app-update.yml` 和对应平台的元数据文件。打包脚本默认 `--publish never`，不会上传到 GitHub。
- 服务端支持 Range / HEAD / 206、元数据不缓存、安装包缓存及文件名白名单，不列目录。
- SHA-512 用于验证安装包完整性；正式发布使用 HTTPS，macOS 更新需有效签名和公证，Windows 发布也应使用签名。
- 按 SemVer 判断新版本，架构专用 channel 不允许降级。1.0.0 是正式发布基线，后续版本从 1.0.1 递增。
- Linux 自动更新主要适用于 AppImage；deb/rpm 安装方式需根据实际包管理流程验证。

### 3.1 发布步骤

1. 在仓库根目录确认版本号，运行 `pnpm --filter @silieco/desktop package:all`（macOS 签名、公证及跨平台构建环境需准备好），或按平台/架构分别执行 `pnpm --filter @silieco/desktop package --mac --arm64` 等命令。每次打包会清理本地 dist，所以分批构建时先将已完成产物转存到发布暂存目录。
2. 从 `apps/desktop/dist/` 收集各架构的安装包、blockmap（若生成）以及对应的 latest*.yml，保持元数据中引用的文件名不变。macOS 需同时保留 ZIP（自动更新）和 DMG（首次安装）。
3. 先上传全部安装包及 blockmap，再替换各 channel 元数据，避免客户端在发布过程中下载到缺失的安装包。保留旧安装包以支持已有客户端的增量下载。Web 下载页也读取该目录，通过 `MEPERSTUDIO_DESKTOP_VERSION`（默认等于构建 VERSION）选择首次安装的版本；无需复制一份安装包到 Web 静态目录。
4. 确认正式域名下每个元数据文件可返回 200、引用的安装包存在并支持 Range，并从上一正式版本进行一次真实升级验证。1.0.0 首次发布后，可用 1.0.1 验证完整升级链路。

## 4. 服务地址切换器（已实现）

目标：App 内可切换部署环境（如 yilia.me ↔ 其它自托管实例），**API / WS / 更新源全部跟随切换**，无需重装。

### 4.1 数据层（已实现）

- 持久化文件：`~/.meperstudio/desktop.json`（旧目录存在时继续使用 `~/.silieco/desktop.json`）（`runtime-config-loader.ts:desktopConfigPath`），schema：

```json
{ "schemaVersion": 1, "apiUrl": "http://yilia.me:6688" }
```

- `parseRuntimeConfig` 已支持：apiUrl 必填，wsUrl/appUrl 缺省自动推导（`deriveWsUrl` / `deriveAppUrl`）。
- 打包模式（非 dev）读取该文件，App 内编辑入口与重启生效流程已实现。

### 4.2 UI 设计（已实现：两处入口）

**入口 ① 登录页（`src/renderer/src/pages/login.tsx`）** —— 登录前先选对服务器：

- 登录卡片下方显示当前服务地址 + "Change server"；点开后输入框 + Save & restart/Cancel
- 保存后**统一重启生效**：`desktop-config:write` 写文件 → 清理旧会话和 daemon 凭据 → 重启应用 → 主进程重新加载 runtime config，API、WebSocket、登录、daemon 与更新源统一指向新环境。
- 提示语 "Signing in connects this app to the server above."

**入口 ② Settings → Daemon → Server 区**（已登录态）：

- 同样的输入 + 保存并重启；保存成功后退出旧会话并重启应用。
- 已登录会话与旧服务器绑定，切换会清理旧会话和 daemon 凭据；应用重启后在新服务器上重新登录。

```
┌────────────────────────────────────────────────────────┐
│ Server                                                  │
│                                                          │
│ 服务地址   [ http://yilia.me:6688            ] [保存]    │
│ 说明：API、实时通道与自动更新都使用此地址。保存后重启；   │
│ daemon 将按新地址选择 profile。                                    │
└────────────────────────────────────────────────────────┘
```

- 输入校验复用 `normalizeHttpUrl`（http/https、去尾斜杠）。
- **更新源自动跟随**：feed 在 runtime config 加载后推导（§2.1）；保存并重启后指向 `新地址/updates/desktop`，避免旧更新源和新 API 混用。
- 预设列表（可选增强）：硬编码常用环境 + 从 `desktop.json` 历史读取；不做远程下发（避免鸡生蛋问题）。

### 4.3 边界情况

| 场景 | 处理 |
| --- | --- |
| 切换到不可达地址 | daemon 启动失败 → 现有 cli_not_found/auth 流程；用户可再次切换（配置文件独立于连接状态） |
| dev 模式 | `loadRuntimeConfig` 走 VITE_API_URL 分支，desktop.json 不生效 —— 开发时用环境变量，切换器仅打包版生效（开发模式保存时返回环境变量配置提示） |
| WebSocket / daemon 目标 | daemon profile 已按 apiUrl 派生（`desktop-<host>-<port>`），切换后 daemon 重启即跟随 |
| 已登录会话 | token 与服务器绑定，保存前后的服务器切换通过清理旧会话并重启实现，要求重新登录 |

### 4.4 实现清单

```
☑ IPC: desktop-config:read / desktop-config:write / desktop-config:restart
  （main，读写当前用户配置目录内的 desktop.json，复用 parseRuntimeConfig 校验；清理旧会话后重启）
☑ 登录页服务地址显示/切换（保存并重启，登录 URL 跟随新地址）
☑ Settings → Daemon → Server 区（已登录态切换入口，保存后退出旧会话并重启）
□ （可选）连接状态探针：保存前 GET <apiUrl>/api/config 探活，失败给确认弹窗
□ （可选）预设环境列表
```

## 5. 已实现 / 待实现总账

| 项 | 状态 |
| --- | --- |
| 客户端 feed 推导 + setFeedURL | ✅ 已实现（`index.ts` / `updater.ts`，注释含完整契约） |
| 服务端静态更新路径 | ✅ Go handler、Docker 挂载与 Web 同源代理已实现，待部署 |
| 发布流程（构建 → 上传） | ✅ 打包生成元数据；实际安装包构建与上传待执行 |
| 服务地址切换器（登录页 + Settings，写后退出旧会话并重启） | ✅ 已实现（§4） |
| HTTPS / 反代 | ⬜ 正式域名 HTTPS / 反代连通性待验证 |

## 6. 关联文件索引

| 文件 | 职责 |
| --- | --- |
| `apps/desktop/src/main/updater.ts` | autoUpdater 封装、channel、setFeedURL（本次改动） |
| `apps/desktop/src/main/index.ts` | feed 从 runtime config 推导（本次改动） |
| `apps/desktop/src/shared/runtime-config.ts` | DEFAULT_RUNTIME_CONFIG / deriveWsUrl / deriveAppUrl / parseRuntimeConfig |
| `apps/desktop/src/main/runtime-config-loader.ts` | desktop.json 读取（切换器的持久层） |
| `apps/desktop/src/renderer/src/components/update-notification.tsx` | 下载完成提示 UI |
| `apps/desktop/src/renderer/src/components/updates-settings-tab.tsx` | 手动检查 / 自动更新开关 |
| `apps/desktop/scripts/package.mjs` | 打包与 channel 命名（latest-arm64.yml 等的对端） |
| `docs/runtime-pi-analysis.md` | pi 默认运行时（更新方案与之并行） |

## 7. 1.0.0 首次部署记录（2026-11-26）

- 正式入口：https://studio.meper.cn/；服务器 119.119.118.86 的 80 端口由 nginx 容器接收上游 HTTPS 网关转发。
- 服务部署目录：`/home/ags/meperstudio/source`；Compose 项目名 `meperstudio`。使用 `docker-compose.selfhost.yml`、`docker-compose.selfhost.build.yml`、`docker-compose.production.yml` 三份配置；PostgreSQL、Redis、Go API、Web 和 nginx 已启动，数据库与 Redis 未暴露宿主机端口。
- 服务器无法直连 Docker Hub 与 Go 代理，此次在开发机交叉构建 amd64 Go 服务镜像和 Web 镜像，再通过 SSH 导入。服务器上的生产 `.env` 独立保存，不随源码同步。
- 发布目录：`/home/ags/meperstudio/releases/desktop`。当前包含 Windows x64 NSIS EXE、Apple Silicon DMG/ZIP、对应 blockmap 及 `latest.yml`、`latest-mac.yml`。先传安装包，再更新元数据。
- 已验证正式 HTTPS 首页、登录页、下载页、API 和更新元数据（HTTP 200），安装包 HEAD 与 Range（206），以及上传后的 SHA-512。
- 当前安装包用于内测：Mac 为 ad-hoc 签名、未公证，Windows 无 Authenticode 发布者签名。真实安装与跨版本自动更新仍需在目标系统验证；正式分发前配置相应签名证书。
- Apple Silicon 本机构建 Windows 时，旧版 Wine 与内置 x86 macOS NSIS 工具不可运行。本次使用原生 JS PE 资源编辑保留 MEPERStudio 图标和版本信息，配合本机 ARM64 `makensis` 构建 NSIS。签名正式版本优先在受支持的 Windows 构建环境生成。


## 8. 1.0.1 实际升级问题与后续修复（2026-11-27）

正式更新源已发布 1.0.1。Mac 1.0.0 可下载 ZIP，但点击 Restart now 或普通重启后版本仍为 1.0.0。核验实际安装包及 1.0.1 ZIP 内应用可执行文件：两者均为 ad-hoc 签名，designated requirement 是各自不同的 cdhash，因此新版本无法满足旧版本的签名要求。不是更新源或下载中断问题。Electron-updater 的 Mac `update-downloaded` 事件仅表示 ZIP 已可交给 Squirrel，不能认定系统安装器已就绪。

后续代码修复：区分下载、原生准备、就绪、安装、失败状态；Mac 等待 Electron native updater 的 `update-downloaded` 事件才启用重启安装。安装超时和异常可见，设置页可恢复查看状态、手动下载，通知支持中英文。已知 ad-hoc 安装仅检查版本并提供手动下载安装入口，不再反复尝试不能满足签名要求的自动安装。保留 Windows/Linux 原有安装机制；自动安装就绪的通知同时提供立即重启和跳转下载页两种操作，所有桌面平台的设置页始终提供下载页入口。

后续正式 macOS 分发需要 **Developer ID Application** 签名、公证，并保持相同的有效签名身份；当前使用 ad-hoc 签名的 1.0.0/1.0.1 客户端需先从下载页手动安装一次新签名版本，之后才能真正验证连续跨版本自动更新。不要将 ZIP 已下载当成跨版本升级成功。发布时使用实际已安装的上个正式签名版本验证点击重启、正常退出安装、重启后版本号变化。

单元测试：`node --experimental-strip-types --test scripts/test-desktop-update-lifecycle.mjs`。
