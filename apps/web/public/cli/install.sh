#!/usr/bin/env bash
set -euo pipefail

BASE_URL="${MEPERSTUDIO_CLI_DOWNLOAD_BASE_URL:-${MEPERSTUDIO_DOWNLOAD_BASE_URL:-${SILIECO_DOWNLOAD_BASE_URL:-https://studio.meper.cn/cli}}}"
BASE_URL="${BASE_URL%/}"
APP_URL="${MEPERSTUDIO_APP_URL:-${SILIECO_APP_URL:-${BASE_URL%/cli}}}"
SERVER_URL="${MEPERSTUDIO_SERVER_URL:-${SILIECO_SERVER_URL:-$APP_URL}}"

info() { printf '==> %s\n' "$*"; }
fail() { printf 'Error: %s\n' "$*" >&2; exit 1; }

case "$(uname -s)" in
  Darwin) os="darwin" ;;
  Linux) os="linux" ;;
  MINGW*|MSYS*|CYGWIN*)
    fail "Windows 请使用 PowerShell 安装命令：\$env:MEPERSTUDIO_DOWNLOAD_BASE_URL='${BASE_URL}'; irm ${BASE_URL}/install.ps1 | iex"
    ;;
  *) fail "不支持的操作系统：$(uname -s)" ;;
esac

case "$(uname -m)" in
  x86_64|amd64) arch="amd64" ;;
  arm64|aarch64) arch="arm64" ;;
  *) fail "不支持的处理器架构：$(uname -m)" ;;
esac

archive="meperstudio-${os}-${arch}.tar.gz"
tmp_dir="$(mktemp -d)"
trap 'rm -rf "$tmp_dir"' EXIT

info "正在从 ${BASE_URL} 下载 MEPERStudio CLI (${os}/${arch})..."
# Prefer the portable TSV projection of the versioned release manifest.
manifest_code=$(curl -sSL -w '%{http_code}' -o "${tmp_dir}/manifest.tsv" "${BASE_URL}/manifest.tsv") || fail "获取发布清单失败"
binary=meperstudio
size=""
if [[ "$manifest_code" == 200 ]]; then
  IFS=$'\t' read -r schema version < "${tmp_dir}/manifest.tsv"
  [[ "$schema" == "meperstudio.cli/v1" && "$version" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]] || fail "发布清单版本无效"
  row=$(awk -F '\t' -v os="$os" -v arch="$arch" 'NR>1 && $1==os && $2==arch { if(NF!=7 || ++count>1) exit 2; print } END { if(count!=1) exit 2 }' "${tmp_dir}/manifest.tsv") || fail "发布清单中没有唯一的平台安装包"
  IFS=$'\t' read -r _ _ artifact_path archive binary expected size <<< "$row"
  [[ "$archive" =~ ^[A-Za-z0-9._-]+$ && "$archive" != '.' && "$archive" != '..' && "$binary" == meperstudio && "$expected" =~ ^[0-9a-f]{64}$ && "$size" =~ ^[1-9][0-9]*$ && "$artifact_path" == "releases/$version/$expected/$archive" ]] || fail "发布清单安装包格式无效"
elif [[ "$manifest_code" == 404 ]]; then
  curl -fsSL "${BASE_URL}/checksums.txt" -o "${tmp_dir}/checksums.txt"
  if ! awk -v file="$archive" '$2 == file { found=1 } END { exit !found }' "${tmp_dir}/checksums.txt"; then
    archive="silieco-${os}-${arch}.tar.gz"
    binary=silieco
  fi
  expected="$(awk -v file="$archive" '$2 == file { print $1 }' "${tmp_dir}/checksums.txt")"
  [[ "$expected" =~ ^[0-9a-f]{64}$ ]] || fail "校验文件中没有有效的 ${archive}"
  artifact_path="$archive"
else
  fail "获取发布清单失败：HTTP $manifest_code"
fi
curl -fsSL "${BASE_URL}/${artifact_path}" -o "${tmp_dir}/${archive}"
if [[ -n "$size" ]]; then
  actual_size=$(wc -c < "${tmp_dir}/${archive}" | tr -d '[:space:]')
  [[ "$actual_size" == "$size" ]] || fail "下载文件长度不匹配"
fi

if command -v shasum >/dev/null 2>&1; then
  actual="$(shasum -a 256 "${tmp_dir}/${archive}" | awk '{print $1}')"
elif command -v sha256sum >/dev/null 2>&1; then
  actual="$(sha256sum "${tmp_dir}/${archive}" | awk '{print $1}')"
else
  fail "缺少 SHA-256 校验工具（shasum 或 sha256sum）"
fi
[ "$actual" = "$expected" ] || fail "下载文件校验失败"

tar -xzf "${tmp_dir}/${archive}" -C "$tmp_dir" "$binary"
chmod +x "${tmp_dir}/${binary}"

bin_dir="${MEPERSTUDIO_BIN_DIR:-${SILIECO_BIN_DIR:-$HOME/.local/bin}}"
mkdir -p "$bin_dir"
[ -w "$bin_dir" ] || fail "安装目录不可写：$bin_dir"
staged=$(mktemp "${bin_dir}/.meperstudio-install-XXXXXX")
cp "${tmp_dir}/${binary}" "$staged" && chmod +x "$staged" && mv -f "$staged" "${bin_dir}/meperstudio" || { rm -f "$staged"; fail "无法替换 CLI，原版本已保留"; }
case ":$PATH:" in
  *":${bin_dir}:"*) ;;
  *)
    shell_rc="$HOME/.profile"
    case "${SHELL:-}" in */zsh) shell_rc="$HOME/.zprofile" ;; */bash) shell_rc="$HOME/.bash_profile" ;; esac
    printf '\n# Added by MEPERStudio CLI installer\nexport PATH="%s:$PATH"\n' "$bin_dir" >> "$shell_rc"
    export PATH="$bin_dir:$PATH"
    info "已更新 ${shell_rc}；当前终端可运行：export PATH=\"$bin_dir:\$PATH\""
    ;;
esac
info "MEPERStudio CLI 已安装到 ${bin_dir}/meperstudio"
"${bin_dir}/meperstudio" --version
printf '\n下一步：\n  meperstudio setup self-host --server-url %s --app-url %s\n' "$SERVER_URL" "$APP_URL"
