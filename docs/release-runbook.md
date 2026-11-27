# MEPERStudio 正式版本发布标准流程

> 基于 2026-11-27 实际完成的 1.0.2 发布整理。后续发布以本手册为入口，结合当前仓库、服务器环境及待发布平台调整具体参数。不要直接复制上次版本的构建产物或依赖临时目录。

## 1. 发布范围与环境

| 项目 | 当前正式发布范围 |
| --- | --- |
| 源码 | `auenger/MEPERECO` 的 `main`，成功发布后推送 `vX.Y.Z` tag |
| 正式入口 | `https://studio.meper.cn`，API 同源，WebSocket `wss://studio.meper.cn/ws` |
| 服务端 | Docker Compose 项目 `meperstudio`，Go backend 与 Next.js frontend 镜像均为 Linux AMD64 |
| 桌面端 | 当前线上渠道：macOS Apple Silicon（DMG + ZIP）、Windows x64（NSIS EXE）以及 blockmap 和更新元数据 |
| CLI / Daemon | macOS、Linux、Windows 的 AMD64 与 ARM64，共六种平台架构；保留旧 `silieco-*` 兼容包 |
| 生产服务器 | `119.119.118.86`，部署目录 `/home/ags/meperstudio/source`，发布目录 `/home/ags/meperstudio/releases/desktop` |

生产服务器目前无法稳定直连构建所需的 Docker Hub / Go 代理，优先在开发机完成交叉编译、构建与校验，再经授权的 SSH 通道上传。生产 `.env`、数据库与存储卷独立于源码，严禁用本地 `.env` 覆盖。凭据只从现有安全配置读取，不记录进本手册或 Git。

新增 Intel macOS、Windows ARM64 或 Linux 桌面渠道时，先明确该渠道的安装包、架构、更新元数据及原生验证范围；不要将不同架构的 `latest*.yml` 相互覆盖。iOS/Android 源码随 Git 推送，应用商店发布是独立流程。

## 2. 发布前必须同步的版本字段

设本次版本为 `X.Y.Z`，Git 提交为 `COMMIT`，构建时间为 UTC `DATE`。

| 位置 | 内容 |
| --- | --- |
| `package.json` | `version: X.Y.Z` |
| `apps/web/package.json` | `version: X.Y.Z`（Web 版本后备值） |
| `apps/desktop/package.json` | `version: X.Y.Z`（Electron 安装包与本地构建） |
| Go backend / CLI 构建 | `-ldflags` 注入 `main.version`、`main.commit`，CLI 另注入 `main.date` |
| 桌面打包 | `MEPERSTUDIO_BUILD_VERSION=X.Y.Z`；打包脚本将版本写入 Electron metadata，并构建匹配架构的内置 CLI |
| Web 构建 | `NEXT_PUBLIC_APP_VERSION=X.Y.Z` |
| CLI 发布 | `scripts/build-cli-release.py --version X.Y.Z --commit COMMIT` |
| 生产 `.env` | `VERSION`、`MEPERSTUDIO_IMAGE_TAG`、`SILIECO_IMAGE_TAG`、**`MEPERSTUDIO_DESKTOP_VERSION`** 全部为 `X.Y.Z`；同步 `COMMIT`、`DATE` |
| 桌面更新元数据 | 由本次实际二进制生成 `latest.yml`、`latest-mac.yml`，其中版本、文件名、SHA-512 和大小必须一致 |

**关键经验：`MEPERSTUDIO_DESKTOP_VERSION` 与镜像 tag 是独立配置。** 1.0.2 切换服务和更新源后，下载页一度仍显示 1.0.1，原因是生产 `.env` 的该字段仍为旧值。必须在同一轮配置切换中更新，而不是只修改 `VERSION`。不要全库盲目替换旧版本字符串：协议版本、迁移编号、历史发布记录及兼容性测试不应跟着应用版本变化。

## 3. 冻结源码与回归测试

1. 确认本地测试 App 已关闭或按需求保留；核对 `git status --short` 的全部变更，排除本地环境文件、凭证、构建产物和无关修改。
2. 确认生产机器当前镜像版本、服务健康、磁盘空间与 SSH 通畅；核对远端 `main`，不要强制推送。
3. 更新第 2 节的仓库版本字段，执行 `git diff --check`。检查发布涉及的 migrations、API、数据访问控制和客户端兼容性。
4. 执行通用测试及本次变更涉及的回归测试：

```bash
(cd core && go test ./...)
pnpm typecheck
node --test scripts/test-cli-contract.mjs
```

例如 1.0.2 的专项回归：

```bash
node --test scripts/test-desktop-update-lifecycle.mjs \
  scripts/test-desktop-update-notice.mjs scripts/test-message-notifications.mjs \
  scripts/test-password-policy.mjs scripts/test-sop-model.mjs

pnpm exec esbuild scripts/test-sop-visibility.mjs --bundle --platform=node \
  --format=esm --outfile=/tmp/meperstudio-sop-visibility-test.mjs
node --test /tmp/meperstudio-sop-visibility-test.mjs
```

有数据库迁移时，先启动独立的 PostgreSQL 测试实例（当前生产为 `pgvector/pgvector:pg17`），使用独立 `DATABASE_URL` 运行 `(cd core && go run ./cmd/migrate up)`。运行涉及实际数据库的测试，例如 `MEPERSTUDIO_PROJECT_TEST_DATABASE_URL`、`MEPERSTUDIO_INVITE_TEST_DATABASE_URL` 指向该独立实例时：

```bash
(cd core && go test ./pkg/db/generated ./internal/handler ./internal/daemon \
  -run 'TestProjectAccessPostgres|TestFixedProjectGroupChatPostgres|TestInviteCandidateSearch|TestNormalizeInviteSearch|TestProjectChatWorkingDirectory' -v)
```

前端全量类型检查当前按仓库约定排除 Expo mobile；若本次包含独立移动端发版，另验证移动端及对应原生构建。不要将交叉编译成功等同于原生平台安装验证。

5. 通过检查后提交本次全部相关源码、测试和版本字段，推送 `main`。从固定提交构建各产物，禁止上线后用同一个版本号覆盖内容不同的二进制；需要实质性修复时发布下一版本。`vX.Y.Z` tag 在正式发布验证通过后创建并推送。

## 4. 构建产物

下列示例从仓库根目录执行。变量均采用本次发布的真实版本；临时构建目录独立，桌面安装包要及时归档，避免下一次打包脚本清空 `apps/desktop/dist`。

```bash
release_version=X.Y.Z  # 替换为真实三段数字版本
release_commit=$(git rev-parse HEAD)
release_date=$(date -u +%Y-%m-%dT%H:%M:%SZ)
release_work=$(mktemp -d /tmp/meperstudio-release.XXXXXX)
mkdir -p "$release_work/backend" "$release_work/cli" "$release_work/desktop"
```

### 4.1 Go 服务和 CLI

网络允许时按仓库原生 Dockerfile 编译：

```bash
docker build --platform linux/amd64 -f Dockerfile \
  --build-arg VERSION="$release_version" --build-arg COMMIT="$release_commit" \
  --build-arg DATE="$release_date" -t "meperstudio-backend:$release_version" .
```

如果 Docker 内网络不稳定，采用已验证的交叉编译方案：本机 Go 构建 `server`、`meperstudio`、`migrate`、`backfill_agent_run_usage_hourly`、`backfill_codex_usage_cache`、`git-bindings-backfill`（具体二进制列表以当前 `Dockerfile` 为准），复制完整 `core/migrations/` 和 `docker/entrypoint.sh`，基于上一版已验证的 AMD64 backend runtime image 打包。示例：

```bash
(
  cd core
  for target in server silieco migrate backfill_agent_run_usage_hourly backfill_codex_usage_cache git-bindings-backfill; do
    output_name=$target
    if [ "$target" = silieco ]; then output_name=meperstudio; fi
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
      -ldflags "-s -w -X main.version=$release_version -X main.commit=$release_commit -X main.date=$release_date" \
      -o "$release_work/backend/$output_name" "./cmd/$target" || exit 1
  done
)
cp -R core/migrations "$release_work/backend/"
cp docker/entrypoint.sh "$release_work/backend/"
```

替代镜像 Dockerfile（如原 Dockerfile 增加系统依赖，需同步更新该方案）：

```dockerfile
ARG BASE_IMAGE
FROM ${BASE_IMAGE}
COPY server meperstudio migrate backfill_agent_run_usage_hourly backfill_codex_usage_cache git-bindings-backfill /app/
COPY migrations/ /app/migrations/
COPY --chmod=755 entrypoint.sh /app/entrypoint.sh
RUN ln -sfn meperstudio /app/silieco
```

将 `BASE_IMAGE` 指向上一版生产 runtime image，以 `--platform linux/amd64` 构建并标记新版本。`entrypoint.sh` 必须先执行迁移，成功后才启动 server。

单独构建完整 CLI 分发目录，后续用于 Web 镜像：

```bash
python3 scripts/build-cli-release.py --output "$release_work/cli" \
  --version "$release_version" --commit "$release_commit"
```

确认六个平台/架构与旧品牌兼容包均生成。`manifest.json`、`manifest.tsv`、`checksums.txt`、`version.txt` 及 `releases/<version>/<sha256>/...` 内容寻址的归档路径必须完整，引用内容不得变更。详细一致性契约见 [配置与 CLI 发布契约](configuration-release-contract.md)。后续发布宜保留上一版的不可变 CLI 路径（指针仅指向新版本），避免正下载旧清单的客户端失去对应归档。

现有原生 CLI 安装回归脚本读取 `apps/web/public/cli/`，因此先将本次生成目录同步至该本地目录（保留仓库自带的 `install.sh`/`install.ps1`，不使用 `--delete`），再运行：

```bash
rsync -a "$release_work/cli/" apps/web/public/cli/
python3 scripts/test-cli-release.py
```

这一步测试实际归档、安装脚本、旧协议回退和损坏包不会覆盖可用 CLI；不会安装到用户真实配置目录。生成的 CLI 分发文件属于构建产物，不提交 Git。

### 4.2 Web 镜像

标准构建使用当前 `Dockerfile.web`：

```bash
docker build --platform linux/amd64 -f Dockerfile.web \
  --build-arg NEXT_PUBLIC_APP_VERSION="$release_version" \
  -t "meperstudio-web:$release_version" .
```

网络受限时，分离 CLI 构建：从当前 `Dockerfile.web` 生成临时副本，去掉开头 `cli-dist` 阶段和末尾 `COPY --from=cli-dist` 行，其他 deps、builder、runner 阶段保持一致。以新版本构建 `meperstudio-web:$release_version-stage`，再以它为基础镜像执行 `COPY --chown=nextjs:nodejs . /app/apps/web/public/cli/`，构建 context 为上一步完整 CLI 目录，最终标记 `meperstudio-web:$release_version`。1.0.2 使用这一方案。不要复用旧版本 Web 静态 bundle 或仅更改镜像标签。

### 4.3 macOS Apple Silicon 安装包

```bash
MEPERSTUDIO_BUILD_VERSION="$release_version" \
  pnpm --filter @silieco/desktop package --mac --arm64 --publish never
```

将 `apps/desktop/dist` 内本次版本的 DMG、ZIP、两份 blockmap 与 `latest-mac.yml` 立即复制到 `$release_work/desktop/`，再开始 Windows 构建。

签名注意：正式外部分发应使用有效且连续的 **Developer ID Application** 签名及公证。1.0.2 当前沿用已知 ad-hoc 测试分发方式（`CSC_IDENTITY_AUTO_DISCOVERY=false`，无公证）；旧 Mac 客户端需要手动从下载页安装，不得宣称已验证跨版本原生自动安装。真实原生自动更新的验证条件见 [桌面端更新方案](app-update-design.md)。如果有正式签名身份，优先在配置完整的构建机签名，不要将 Apple Development 证书当作正式分发证书。

### 4.4 Windows x64 NSIS 安装包

优先使用支持原生 Windows 签名和打包工具的构建机：

```bash
MEPERSTUDIO_BUILD_VERSION="$release_version" \
  pnpm --filter @silieco/desktop package --win --x64 --publish never
```

Apple Silicon 开发机上，之前的旧 Wine/rcedit 与内置 x86 NSIS 工具不能原生执行。1.0.2 验证过的替代流程：

1. 执行 `MEPERSTUDIO_BUILD_VERSION="$release_version" CSC_IDENTITY_AUTO_DISCOVERY=false pnpm --filter @silieco/desktop package --win dir --x64 --publish never -c.win.signAndEditExecutable=false`，得到 `apps/desktop/dist/win-unpacked`（内置 CLI 为 Windows x64）。
2. 用 Node 的 `pe-library` / `resedit` 对 `MEPERStudio.exe` 更新品牌图标、产品名及文件/产品版本，保持原有 PE 其他资源和 ASAR 完整性资源。下面是 1.0.2 的实际做法，版本通过环境变量传入：

```bash
MEPERSTUDIO_BUILD_VERSION="$release_version" node --input-type=module - <<'JS'
import fs from 'node:fs';
import * as PE from 'pe-library';
import * as RE from 'resedit';
const version = process.env.MEPERSTUDIO_BUILD_VERSION;
const [major, minor, patch] = version.split('.').map(Number);
const file = 'apps/desktop/dist/win-unpacked/MEPERStudio.exe';
const exe = PE.NtExecutable.from(fs.readFileSync(file));
const res = PE.NtExecutableResource.from(exe);
const icons = RE.Data.IconFile.from(fs.readFileSync('apps/desktop/build/icon.ico')).icons.map(x => x.data);
const groups = RE.Resource.IconGroupEntry.fromEntries(res.entries);
for (const group of groups) RE.Resource.IconGroupEntry.replaceIconsForResource(res.entries, group.id, group.lang, icons);
if (!groups.length) RE.Resource.IconGroupEntry.replaceIconsForResource(res.entries, 101, 1033, icons);
for (const vi of RE.Resource.VersionInfo.fromEntries(res.entries)) {
  vi.setFileVersion(major, minor, patch, 0, 1033);
  vi.setProductVersion(major, minor, patch, 0, 1033);
  vi.setStringValues({lang:1033,codepage:1200},{FileDescription:'MEPERStudio',ProductName:'MEPERStudio',CompanyName:'MEPERStudio',InternalName:'MEPERStudio',OriginalFilename:'MEPERStudio.exe',FileVersion:version,ProductVersion:version});
  vi.outputToResourceEntries(res.entries);
}
res.outputResource(exe);
fs.writeFileSync(file, Buffer.from(exe.generate()));
const verified = RE.Resource.VersionInfo.fromEntries(PE.NtExecutableResource.from(PE.NtExecutable.from(fs.readFileSync(file))).entries);
if (!verified.some(x=>x.getStringValues({lang:1033,codepage:1200}).ProductVersion===version)) throw new Error('PE version mismatch');
JS
```

3. 准备本机 ARM64 `makensis`（Homebrew `makensis`），复制 Electron Builder 当前兼容 NSIS 工具目录至本次临时目录（1.0.2 使用 `nsis-3.0.4.1`）。保留工具资源、Include、Plugins、Stubs 等，在复制后的 `mac/makensis` 放置本机原生 makensis 的符号链接；**不要修改全局 Electron Builder 缓存或依赖上次 `/tmp/meperstudio-nsis.*` 目录**。
4. 从已打好的目录生成 NSIS（下方 `nsis_dir` 指向第 3 步的实际目录），而不是重新打包 App：

```bash
(
  cd apps/desktop
  ELECTRON_BUILDER_NSIS_DIR="$nsis_dir" CSC_IDENTITY_AUTO_DISCOVERY=false \
    pnpm exec electron-builder --win nsis --x64 --prepackaged dist/win-unpacked \
    --publish never "-c.extraMetadata.version=$release_version"
)
```

5. 将 EXE、EXE blockmap 与 `latest.yml` 复制到 `$release_work/desktop/`。无 Authenticode 发布者证书时属于当前测试分发签名限制，不能当作已通过正式签名验证。打包完成后如还需要本地开发，重新执行本机 `bundle-cli` 恢复正确架构的开发用二进制。

## 5. 产物校验、备份与上传

检查两个镜像均为 `amd64`，tag 与本次版本一致；桌面 ZIP/DMG/EXE 的 `latest*.yml` 每一项 SHA-512（Base64）及 `size` 与实际文件一致，blockmap 存在。CLI 的六条 manifest 条目按照 `sha256` 与 `size_bytes` 校验，确保同一版本的真实内容一致。

在服务器 `/home/ags/meperstudio/backups/pre-X.Y.Z-时间戳/` 生成 PostgreSQL `pg_dump -Fc` 备份、生产 `.env` 备份、旧 `latest*.yml` 备份；限制目录和文件权限、确保 dump 非空，保留旧镜像及旧版安装包。备份不要上传 Git。必要时另做对象存储备份（有相关数据变更时）。

经 SSH 上传至独立 staging，网络不稳定时使用可续传 rsync：

- `git archive` 当前固定提交的源码（不包含 `.env`），解包至 staging 的 `source/`。
- backend、frontend 镜像：本机 `docker save <image>:<version> | gzip -1`，服务器 `gzip -dc | docker load`。
- 桌面产物与元数据目录：上传至 `/home/ags/meperstudio/releases/staging/X.Y.Z/`。

在服务器再次核对镜像 tag、实际文件大小及 SHA-512，避免传输截断或不完整。不要向正式 `latest*.yml` 提前写入新版本。

## 6. 正式切换（生产服务器）

1. 确认备份与 staging 校验通过，保留原有 `.env` 和数据卷；把固定提交源码同步到 `/home/ags/meperstudio/source`，不覆盖 `.env`。
2. 一次性原子修改生产 `.env` 中第 2 节的全部版本字段（尤其 `MEPERSTUDIO_DESKTOP_VERSION`），保留其他配置和权限。用 `docker compose config --images` 确认实际将使用新 tag 的 backend、frontend，而不是输出含凭证的完整配置。
3. 使用实际生产 Compose 组合切换，不在生产机器临时重新构建：

```bash
cd /home/ags/meperstudio/source
compose() {
  docker compose -p meperstudio --env-file .env \
    -f docker-compose.selfhost.yml -f docker-compose.selfhost.build.yml \
    -f docker-compose.production.yml "$@"
}
compose config --images
compose up -d --no-build --no-deps backend frontend
```

4. 等待新 backend 容器 `/healthz`（返回 `db: ok` / `migrations: ok`）与 frontend HTTP 200。`docker/entrypoint.sh` 会先迁移再启动 server；有错误时不要继续发布更新元数据。
5. 重建 gateway，让 nginx 解析新容器地址，而不是继续连接旧容器 IP：`compose up -d --no-build --no-deps --force-recreate gateway`。确认 Postgres、Redis 未被重建且健康。
6. 复制本次版本号命名的正式安装包及 blockmap 至 `/home/ags/meperstudio/releases/desktop/`。文件全部存在并通过校验后，分别将 `latest.yml` / `latest-mac.yml` 复制成同目录临时文件，再用 `mv` 原子替换各自的更新指针。新版安装包、CLI 与服务端必须来自同一发布版本。

## 7. 发布后验收（全部通过才宣布完成）

| 检查 | 预期 |
| --- | --- |
| `https://studio.meper.cn/`、`/login`、`/download`、`/health` | HTTP 200 |
| `/api/config` | HTTP 200，`server_version == X.Y.Z` |
| backend 内部 `/healthz` | `status: ok`，数据库及迁移健康 |
| `/cli/version.txt`、`/cli/manifest.json` | 版本正确，六条架构归档均可下载并匹配 SHA-256 |
| `/updates/desktop/latest.yml`、`/updates/desktop/latest-mac.yml` | 两个版本均为 `X.Y.Z`，引用新包 |
| 每个正式桌面安装包 | HEAD 为 200，Range GET 为 206；已上传二进制的 SHA-512 应与元数据相同 |
| `/download` HTML 页面 | 实际包含新版本 Mac 和 Windows 安装包的 URL，而非仅确认页面 200 |
| 容器 | backend / frontend 为新 tag，gateway、数据库和 Redis 正常 |
| 本机开发服务 | 若要求停掉，确认 `5173` 不再监听、对应 dev App 已退出；无需关闭用户的已安装正式 App |

抽样公开 URL 可用 `curl -fsS`、`curl -I` 及 `curl -H 'Range: bytes=0-31'` 检测。避免在 zsh 中使用变量名 `path`（它会覆盖命令搜索路径）。检查具体功能时用授权的测试账户，避免在正式环境创建多余的用户、邀请或项目数据；用户验收的实际客户端安装/升级、登录和核心功能测试另记录结果。

验收通过后创建和推送 Git tag（不移动已存在的 tag）：

```bash
git tag -a "v$release_version" -m "MEPERStudio $release_version" "$release_commit"
git push origin "v$release_version"
```

归档本次产物、校验元数据、发布记录及宣传海报到本地 `MEPERStudio-releases/X.Y.Z/`（大体积安装包不提交 Git）。对外提供 [正式站](https://studio.meper.cn) 和 [下载页](https://studio.meper.cn/download)，说明签名、平台范围及尚未完成的原生人工验证。

## 8. 故障与回滚

- **切换前失败：** 保留旧生产服务与更新指针，修复并重新构建未发布版本的产物。
- **服务切换后健康检查失败：** 尚未替换更新指针时，恢复备份生产 `.env` 的旧 tag/版本配置，使用旧镜像重建 backend、frontend 和 gateway，并确认旧服务正常。迁移可能已执行；仅在确认与旧版本兼容的情况下回滚应用代码。不要自动删除新表或用过期数据库 dump 覆盖用户新写入的数据。
- **仅下载页面版本错误：** 检查 `MEPERSTUDIO_DESKTOP_VERSION` 是否仍为旧版本，修复配置后重建 frontend 与 gateway，复验真实链接。不要只检查 `latest.yml`。
- **原生安装失败：** 先区分未下载、下载完毕、原生准备、安装失败。Mac ad-hoc 包无法保证连续原生更新，提供手动下载路径。不要仅凭下载成功宣称安装成功。
- **发布指针已经公开后发现实质性修复：** 留存现有不可变版本包，优先发布下一个版本，不覆写同名新旧签名或内容不同的二进制。必要的紧急回滚先确认数据库兼容性与更新客户端行为，保留完整证据和可恢复副本。

## 9. 1.0.2 实际发布记录

- 日期：2026-11-27；源码提交 `87b3f2a6b84fc8beae2096ec470b6d1157c2f8fe`，tag `v1.0.2`。
- 已发布：Web、Go API、六架构 CLI、macOS ARM64 DMG/ZIP、Windows x64 EXE 及更新元数据。
- 项目权限迁移 `0385_project_members` 在独立 PG17 环境及正式环境均成功；发布前生成数据库及配置备份。
- Go 全量、Web/Desktop 类型检查、权限/群聊/邀请数据库集成、消息提醒、密码、更新状态与 SOP 开关专项测试通过。
- CLI 契约、真实本地临时目录安装、旧协议回退与损坏包防覆盖测试通过。1.0.2 独立 CLI 实际构建使用了默认 `commit: source`；下次按第 4 节传入真实提交，便于精确追溯。
- 生产 API、CLI、两个桌面 update feed 与下载页均已验证为 1.0.2，所有正式桌面包 HEAD 200 / Range 206，上传后 SHA-512 与元数据一致；公开 CLI 六个架构归档全部下载并验证 SHA-256 和长度一致。
- Mac 使用 ad-hoc 签名且未公证，Windows 未使用正式 Authenticode 证书；发布可下载不等同于目标平台原生安装或跨版本自动更新已验证。后续由实际客户端继续验收，不阻断本次已授权的生产测试版本发布。
