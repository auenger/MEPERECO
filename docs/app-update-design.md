# APP 更新方案设计 — 归档

> 日期：2026-11-24 · 状态：客户端已实现（feed 推导 + setFeedURL）；服务端部署待做；服务地址切换器待实现（本文 §4）
> 决策方向：**不依赖 GitHub / 公网 CDN，更新完全走部署自己的服务器；服务地址在 App 内可切换，更新跟随当前服务地址**

---

## 1. 背景与问题

| 问题 | 现状 |
| --- | --- |
| 自动更新空转 | electron-updater 全套逻辑/UI 在（`src/main/updater.ts`），但 `electron-builder.yml` 里 `publish: null`，无 app-update.yml，检查必然失败。原设计指向 GitHub Releases（`scripts/package.mjs` 注释可证），但 GitHub 在部署网络不可达 |
| 外部依赖不可用 | silieco.ai 完全不可达（实测连接失败）；GitHub releases 下载 ECONNRESET |
| 网络环境 | 目标用户网络直连 GitHub/公网 CDN 不稳定，自有服务器（yilia.me:6688）可达且已承载 API |

**决策**：更新源从 runtime config（部署的 API 服务器）推导，与 API/WS 同源。放弃 GitHub Releases、放弃公网 CDN。

## 2. 客户端设计（已实现）

### 2.1 更新源推导

`apps/desktop/src/main/index.ts`（app.whenReady 内，runtime config 加载后）：

```
feedUrl = <apiUrl>/updates/desktop        （例：http://yilia.me:6688/updates/desktop）
setupAutoUpdater(() => mainWindow, feedUrl)
```

`src/main/updater.ts`：

```
autoUpdater.setFeedURL({ provider: "generic", url: feedUrl })
```

- `apiUrl` 未配置时（理论不发生，DEFAULT_RUNTIME_CONFIG 有默认值）不设置 feed，行为等同旧状态（静默失败）。
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
| Linux | latest | `latest-linux.yml` |

## 3. 服务端契约（待部署）

在 API 服务器（当前 yilia.me:6688，Go 后端）增加静态文件服务，路径前缀：

```
/updates/desktop/
├── latest.yml                # electron-builder 生成，含 version/path/sha512
├── latest-arm64.yml
├── latest-mac.yml
├── latest-x64-mac.yml
├── latest-linux.yml
└── <安装包产物>               # yml 的 path 字段引用，如 silieco-desktop-4.0.0-win-x64.exe
```

- 文件由 `electron-builder` 构建时本地生成（publish: null 不上传）；**发布 = 把构建产物 + 对应 yml 上传到该路径**（手动或 CI）。
- yml 内 sha512 由 electron-updater 自动校验，防损坏/防篡改（传输层建议后续上 HTTPS / 反向代理）。
- Go 后端实现建议：一个 `GET /updates/desktop/*` 静态文件 handler（或 nginx alias），Content-Type: application/xml（yml）/ 对应安装包类型；无需鉴权（electron-updater 不带凭证）。
- 版本号比较由 electron-updater 按 semver 处理（`version-decision.ts` 现有的降级/灰度决策逻辑继续生效）。

## 4. 服务地址切换器（待实现，设计定稿）

目标：App 内可切换部署环境（如 yilia.me ↔ 其它自托管实例），**API / WS / 更新源全部跟随切换**，无需重装。

### 4.1 数据层（已存在，缺 UI）

- 持久化文件：`~/.silieco/desktop.json`（`runtime-config-loader.ts:desktopConfigPath`），schema：

```json
{ "schemaVersion": 1, "apiUrl": "http://yilia.me:6688" }
```

- `parseRuntimeConfig` 已支持：apiUrl 必填，wsUrl/appUrl 缺省自动推导（`deriveWsUrl` / `deriveAppUrl`）。
- 打包模式（非 dev）已读取该文件 —— 缺的只是 **App 内编辑入口 + 生效流程**。

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
  （main，读写 ~/.silieco/desktop.json，复用 parseRuntimeConfig 校验；清理旧会话后重启）
☑ 登录页服务地址显示/切换（保存并重启，登录 URL 跟随新地址）
☑ Settings → Daemon → Server 区（已登录态切换入口，保存后退出旧会话并重启）
□ （可选）连接状态探针：保存前 GET <apiUrl>/api/config 探活，失败给确认弹窗
□ （可选）预设环境列表
```

## 5. 已实现 / 待实现总账

| 项 | 状态 |
| --- | --- |
| 客户端 feed 推导 + setFeedURL | ✅ 已实现（`index.ts` / `updater.ts`，注释含完整契约） |
| 服务端静态更新路径 | ⬜ 部署动作（Go handler 或 nginx，§3） |
| 发布流程（构建 → 上传） | ⬜ 团队流程约定（可后续 CI 化） |
| 服务地址切换器（登录页 + Settings，写后退出旧会话并重启） | ✅ 已实现（§4） |
| HTTPS / 反代 | ⬜ 运维项（当前 http，sha512 已兜底完整性） |

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
