$ErrorActionPreference = "Stop"
$baseUrl = if ($env:MEPERSTUDIO_CLI_DOWNLOAD_BASE_URL) { $env:MEPERSTUDIO_CLI_DOWNLOAD_BASE_URL.TrimEnd('/') } elseif ($env:MEPERSTUDIO_DOWNLOAD_BASE_URL) { $env:MEPERSTUDIO_DOWNLOAD_BASE_URL.TrimEnd('/') } elseif ($env:SILIECO_DOWNLOAD_BASE_URL) { $env:SILIECO_DOWNLOAD_BASE_URL.TrimEnd('/') } else { "https://studio.meper.cn/cli" }
$appUrl = if ($env:MEPERSTUDIO_APP_URL) { $env:MEPERSTUDIO_APP_URL } elseif ($env:SILIECO_APP_URL) { $env:SILIECO_APP_URL } else { $baseUrl -replace "/cli$", "" }
$serverUrl = if ($env:MEPERSTUDIO_SERVER_URL) { $env:MEPERSTUDIO_SERVER_URL } elseif ($env:SILIECO_SERVER_URL) { $env:SILIECO_SERVER_URL } else { $appUrl }

$arch = if ([System.Runtime.InteropServices.RuntimeInformation]::OSArchitecture -eq [System.Runtime.InteropServices.Architecture]::Arm64) { "arm64" } else { "amd64" }
$archive = "meperstudio-windows-$arch.zip"
$tempDir = Join-Path ([System.IO.Path]::GetTempPath()) ("meperstudio-cli-" + [guid]::NewGuid().ToString("N"))
$binDir = if ($env:MEPERSTUDIO_BIN_DIR) { $env:MEPERSTUDIO_BIN_DIR } elseif ($env:SILIECO_BIN_DIR) { $env:SILIECO_BIN_DIR } else { Join-Path $env:LOCALAPPDATA "MEPERStudio\bin" }

New-Item -ItemType Directory -Force -Path $tempDir, $binDir | Out-Null
try {
  Write-Host "==> 正在从 $baseUrl 下载 MEPERStudio CLI (windows/$arch)..."
  $declaredBinary = $null
  $expectedSize = $null
  try { $hasManifest = $true; $release = Invoke-RestMethod -UseBasicParsing "$baseUrl/manifest.json" }
  catch {
    if ([int]$_.Exception.Response.StatusCode -ne 404) { throw }
    $hasManifest = $false; $release = $null
  }
  if ($hasManifest) {
    if ($release.schema_version -ne "meperstudio.cli/v1" -or $release.version -notmatch '^\d+\.\d+\.\d+$') { throw "发布清单版本无效" }
    $entries = @($release.artifacts | Where-Object { $_.os -eq "windows" -and $_.arch -eq $arch })
    if ($entries.Count -ne 1) { throw "发布清单中没有唯一的 Windows/$arch 安装包" }
    $entry = $entries[0]
    $archive = $entry.archive_name; $expected = $entry.sha256; $declaredBinary = $entry.binary
    $expectedSize = $entry.size_bytes; $artifactPath = $entry.path
    if ($archive -notmatch '^[A-Za-z0-9._-]+$' -or $archive -in @('.','..') -or $declaredBinary -ne "meperstudio.exe" -or $expected -cnotmatch '^[0-9a-f]{64}$' -or $expectedSize -le 0 -or $artifactPath -cne "releases/$($release.version)/$expected/$archive") { throw "发布清单安装包格式无效" }
  } else {
    Invoke-WebRequest -UseBasicParsing "$baseUrl/checksums.txt" -OutFile (Join-Path $tempDir "checksums.txt")
    $manifest = Get-Content (Join-Path $tempDir "checksums.txt")
    if (-not ($manifest | Where-Object { $_ -match "\s+$([regex]::Escape($archive))$" })) { $archive = "silieco-windows-$arch.zip" }
    $line = $manifest | Where-Object { $_ -match "\s+$([regex]::Escape($archive))$" } | Select-Object -First 1
    if (-not $line) { throw "校验文件中没有 $archive" }
    $expected = ($line -split '\s+')[0].ToLowerInvariant()
    $artifactPath = $archive
  }
  $archivePath = Join-Path $tempDir $archive
  Invoke-WebRequest -UseBasicParsing "$baseUrl/$artifactPath" -OutFile $archivePath
  if ($null -ne $expectedSize -and (Get-Item $archivePath).Length -ne $expectedSize) { throw "下载文件长度不匹配" }

  $actual = (Get-FileHash -Algorithm SHA256 $archivePath).Hash.ToLowerInvariant()
  if ($actual -ne $expected) { throw "下载文件校验失败" }

  Expand-Archive -Force $archivePath $tempDir
  $binary = if ($declaredBinary) { $declaredBinary } elseif (Test-Path (Join-Path $tempDir "meperstudio.exe")) { "meperstudio.exe" } else { "silieco.exe" }
  $target = Join-Path $binDir "meperstudio.exe"
  $staged = "$target.new-$([guid]::NewGuid().ToString('N'))"
  $backup = "$target.previous-$([guid]::NewGuid().ToString('N'))"
  try {
    Copy-Item -Force (Join-Path $tempDir $binary) $staged
    if (Test-Path $target) { [IO.File]::Replace($staged, $target, $backup); Remove-Item -Force $backup -ErrorAction SilentlyContinue }
    else { [IO.File]::Move($staged, $target) }
  } finally { Remove-Item -Force $staged -ErrorAction SilentlyContinue }

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
