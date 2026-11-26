$ErrorActionPreference = "Stop"
$baseUrl = if ($env:MEPERSTUDIO_DOWNLOAD_BASE_URL) { $env:MEPERSTUDIO_DOWNLOAD_BASE_URL.TrimEnd('/') } elseif ($env:SILIECO_DOWNLOAD_BASE_URL) { $env:SILIECO_DOWNLOAD_BASE_URL.TrimEnd('/') } else { "https://studio.meper.cn/cli" }
$appUrl = if ($env:MEPERSTUDIO_APP_URL) { $env:MEPERSTUDIO_APP_URL } elseif ($env:SILIECO_APP_URL) { $env:SILIECO_APP_URL } else { $baseUrl -replace "/cli$", "" }
$serverUrl = if ($env:MEPERSTUDIO_SERVER_URL) { $env:MEPERSTUDIO_SERVER_URL } elseif ($env:SILIECO_SERVER_URL) { $env:SILIECO_SERVER_URL } else { $appUrl }

$arch = if ([System.Runtime.InteropServices.RuntimeInformation]::OSArchitecture -eq [System.Runtime.InteropServices.Architecture]::Arm64) { "arm64" } else { "amd64" }
$archive = "meperstudio-windows-$arch.zip"
$tempDir = Join-Path ([System.IO.Path]::GetTempPath()) ("meperstudio-cli-" + [guid]::NewGuid().ToString("N"))
$binDir = if ($env:MEPERSTUDIO_BIN_DIR) { $env:MEPERSTUDIO_BIN_DIR } elseif ($env:SILIECO_BIN_DIR) { $env:SILIECO_BIN_DIR } else { Join-Path $env:LOCALAPPDATA "MEPERStudio\bin" }

New-Item -ItemType Directory -Force -Path $tempDir, $binDir | Out-Null
try {
  Write-Host "==> 正在从 $baseUrl 下载 MEPERStudio CLI (windows/$arch)..."
  Invoke-WebRequest -UseBasicParsing "$baseUrl/checksums.txt" -OutFile (Join-Path $tempDir "checksums.txt")
  $manifest = Get-Content (Join-Path $tempDir "checksums.txt")
  if (-not ($manifest | Where-Object { $_ -match "\s+$([regex]::Escape($archive))$" })) { $archive = "silieco-windows-$arch.zip" }
  $archivePath = Join-Path $tempDir $archive
  Invoke-WebRequest -UseBasicParsing "$baseUrl/$archive" -OutFile $archivePath

  $line = Get-Content (Join-Path $tempDir "checksums.txt") | Where-Object { $_ -match "\s+$([regex]::Escape($archive))$" } | Select-Object -First 1
  if (-not $line) { throw "校验文件中没有 $archive" }
  $expected = ($line -split '\s+')[0].ToLowerInvariant()
  $actual = (Get-FileHash -Algorithm SHA256 $archivePath).Hash.ToLowerInvariant()
  if ($actual -ne $expected) { throw "下载文件校验失败" }

  Expand-Archive -Force $archivePath $tempDir
  $binary = if (Test-Path (Join-Path $tempDir "meperstudio.exe")) { "meperstudio.exe" } else { "silieco.exe" }
  Copy-Item -Force (Join-Path $tempDir $binary) (Join-Path $binDir "meperstudio.exe")

  $userPath = [Environment]::GetEnvironmentVariable("Path", "User")
  if (($userPath -split ';') -notcontains $binDir) {
    [Environment]::SetEnvironmentVariable("Path", (($userPath.TrimEnd(';') + ';' + $binDir).TrimStart(';')), "User")
  }
  $env:Path = "$binDir;$env:Path"
  Write-Host "==> MEPERStudio CLI 已安装到 $binDir\meperstudio.exe"
  & (Join-Path $binDir "meperstudio.exe") --version
  Write-Host "`n下一步："
  Write-Host "  meperstudio setup self-host --server-url $serverUrl --app-url $appUrl"
} finally {
  Remove-Item -Recurse -Force $tempDir -ErrorAction SilentlyContinue
}
