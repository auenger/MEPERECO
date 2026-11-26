# MEPERStudio 配置与 CLI 发布契约（v1）

## 配置边界与兼容

新配置采用 `MEPERSTUDIO_` 前缀。Go 的 `core/internal/branding.LookupEnv` 是当前直接读取配置时的统一入口：同后缀的 canonical 值优先于 `SILIECO_`，即使 canonical 值显式为空；各配置解析器再按本身规则处理空值和类型。CLI 中显式参数优先，配置文件和默认值由调用方作为后备值传入。旧进程入口的 alias bridge 暂时保留，供尚未迁移的内部读取者和旧客户端兼容。第三方 provider（如 `CODEX_HOME`）的原生环境变量不改名。

| Canonical 配置 | 兼容配置 | 作用域/合法值 | 默认或后备 |
|---|---|---|---|
| `MEPERSTUDIO_SERVER_URL` | `SILIECO_SERVER_URL` | CLI/Daemon；HTTP(S)、WS(S) URL | CLI 配置文件；Daemon `ws://localhost:8080/ws` |
| `MEPERSTUDIO_APP_URL` | `SILIECO_APP_URL` | 服务端/安装器；Web URL | 部署配置或安装器由下载地址推导 |
| `MEPERSTUDIO_CLI_DOWNLOAD_BASE_URL` | `MEPERSTUDIO_DOWNLOAD_BASE_URL`、`SILIECO_DOWNLOAD_BASE_URL` | CLI updater/Desktop/安装器；发布目录 URL；按列出的顺序选择首个非空项 | `https://studio.meper.cn/cli` |
| `MEPERSTUDIO_BIN_DIR` | `SILIECO_BIN_DIR` | Shell/PowerShell 安装器；本机安装目录 | Shell `~/.local/bin`；Windows 采用安装器原有路径 |
| `MEPERSTUDIO_DAEMON_MAX_CONCURRENT_RUNS` | 同后缀旧前缀 | Daemon；正整数 | 20 |
| `MEPERSTUDIO_DAEMON_POLL_INTERVAL` | 同后缀旧前缀 | Daemon；正 Go duration | 30s |
| `MEPERSTUDIO_DAEMON_HEARTBEAT_INTERVAL` | 同后缀旧前缀 | Daemon；正 Go duration | 15s |
| `MEPERSTUDIO_AGENT_TIMEOUT` | 同后缀旧前缀 | Daemon；非负 Go duration；0 表示不设总时限 | 0 |
| `MEPERSTUDIO_AGENT_IDLE_WATCHDOG` | 同后缀旧前缀 | Daemon；非负 Go duration；0 禁用 | 30m |
| `MEPERSTUDIO_AGENT_TOOL_WATCHDOG` | 同后缀旧前缀 | Daemon；非负 Go duration；0 禁用 | 2h |
| `MEPERSTUDIO_<PROVIDER>_PATH` / `_MODEL` | 同后缀旧前缀 | Daemon；由 Runtime Registry 描述每个适配器的实际支持情况 | PATH 自动探测 / provider 自选模型 |
| `MEPERSTUDIO_TOKEN`、`MEPERSTUDIO_WORKSPACE_ID`、`MEPERSTUDIO_RUN_ID` 等 | 同后缀旧前缀 | 任务执行上下文，仅由 Daemon 可信执行边界注入 | 无；不继承 Daemon 用户凭证 |

状态目录优先选择已存在的 `~/.meperstudio`，否则复用已存在的 `~/.silieco`，新装默认 `~/.meperstudio`。Go 和 Electron 的共同测试向量位于 `core/internal/branding/testdata/state-dir.json`。不自动移动旧目录，不擅自改写 `.env` 或旧 profile。

## CLI Release Manifest

`make cli-release` 和 `Dockerfile.web` 都使用 `scripts/build-cli-release.py`。生成的 `manifest.json` 是版本化公共发布接口，`manifest.tsv` 是同一数据的便携安装器投影，旧 `version.txt`、`checksums.txt` 与旧新品牌归档路径继续兼容。

- `schema_version`: 固定为 `meperstudio.cli/v1`。
- `version`: 三段数字版本。
- `artifacts`: 每个平台/架构唯一条目，包含 `os`、`arch`、`archive_name`、`binary`、`sha256`、`size_bytes`、`path`。
- `path`: `releases/<version>/<sha256>/<archive_name>`，不可由客户端拼装另一个版本的 URL。
- 新客户端只在清单不存在（HTTP 404）时退回旧协议；存在但格式错误、下载不全或校验不符时失败，不以旧协议掩盖发布错误。
- 部署时必须先生成完整文件，再公开指针；清理旧哈希目录应由发布保留策略控制。仓库生成器提供内容寻址布局，不代表已有生产 CDN 自动实现长期保留或滚动发布原子性。

## 回归入口

```sh
cd core && go test ./...
```

在仓库根目录：

```sh
node --experimental-strip-types --test scripts/test-cli-contract.mjs
make cli-release
python3 scripts/test-cli-release.py
```

末条测试在临时 HOME 中通过本机 HTTP 服务运行真实安装脚本，并验证旧清单回退及损坏包失败时旧二进制仍可用。Windows 原生安装和 Desktop 原生更新仍需对应平台验证，交叉编译不是原生测试。
