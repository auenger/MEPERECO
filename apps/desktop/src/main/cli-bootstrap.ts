import { app } from "electron";
import { execFile } from "child_process";
import { createHash } from "crypto";
import { createReadStream, createWriteStream, existsSync } from "fs";
import { chmod, copyFile, mkdir, rename, rm, stat } from "fs/promises";
import { join, dirname } from "path";
import { pipeline } from "stream/promises";
import { tmpdir } from "os";
import { Readable } from "stream";

import { parseCliManifest, cliArtifactFor } from "@silieco/core/release/cli-manifest";
import { selectPlatformReleaseAssetName } from "./cli-release-asset";

// Desktop prefers the bundled `silieco` CLI shipped inside the app for
// same-repo builds, but it can also repair or bootstrap a managed copy in
// userData on first launch when the bundled binary is missing or unusable.

const CLI_DOWNLOAD_BASE = (process.env.MEPERSTUDIO_CLI_DOWNLOAD_BASE_URL || process.env.MEPERSTUDIO_DOWNLOAD_BASE_URL || process.env.SILIECO_DOWNLOAD_BASE_URL || "https://studio.meper.cn/cli").replace(/\/+$/, "");

function binaryName(): string {
  return process.platform === "win32" ? "meperstudio.exe" : "meperstudio";
}

export function managedCliPath(): string {
  return join(app.getPath("userData"), "bin", binaryName());
}

function run(cmd: string, args: string[], cwd?: string): Promise<void> {
  return new Promise((resolve, reject) => {
    execFile(cmd, args, { cwd }, (err) => (err ? reject(err) : resolve()));
  });
}

async function downloadToFile(url: string, dest: string): Promise<void> {
  const res = await fetch(url, { redirect: "follow" });
  if (!res.ok || !res.body) {
    throw new Error(`download failed: ${res.status} ${res.statusText}`);
  }
  await mkdir(dirname(dest), { recursive: true });
  // Node's fetch returns a web ReadableStream; adapt to a Node stream for pipeline.
  const nodeStream = Readable.fromWeb(res.body as Parameters<typeof Readable.fromWeb>[0]);
  await pipeline(nodeStream, createWriteStream(dest));
}

// Fetch goreleaser's published checksums.txt and parse it into a
// filename → sha256 lookup. Format is `<hex>  <filename>` per line.
async function fetchChecksums(): Promise<Map<string, string>> {
  const url = `${CLI_DOWNLOAD_BASE}/checksums.txt`;
  const res = await fetch(url, { redirect: "follow" });
  if (!res.ok) {
    throw new Error(
      `checksums.txt fetch failed: ${res.status} ${res.statusText}`,
    );
  }
  const text = await res.text();
  const map = new Map<string, string>();
  for (const rawLine of text.split("\n")) {
    const line = rawLine.trim();
    if (!line) continue;
    const match = line.match(/^([a-f0-9]{64})\s+\*?(\S+)$/i);
    if (match) map.set(match[2], match[1].toLowerCase());
  }
  return map;
}

async function sha256OfFile(path: string): Promise<string> {
  const hash = createHash("sha256");
  await pipeline(createReadStream(path), hash);
  return hash.digest("hex");
}

async function verifyChecksum(
  archivePath: string,
  assetName: string,
  expected: string,
): Promise<void> {
  const actual = await sha256OfFile(archivePath);
  if (actual.toLowerCase() !== expected) {
    throw new Error(
      `checksum mismatch for ${assetName}: expected ${expected}, got ${actual}`,
    );
  }
}

async function extractArchive(archive: string, dest: string): Promise<void> {
  await mkdir(dest, { recursive: true });
  // Modern OSes all ship a `tar` that auto-detects tar.gz and zip:
  // - macOS/Linux: GNU tar or bsdtar
  // - Windows 10+: bsdtar is bundled as `tar.exe` since build 17063
  await run("tar", ["-xf", archive, "-C", dest]);
}

async function installFresh(): Promise<string> {
  const target = managedCliPath();
  const response = await fetch(`${CLI_DOWNLOAD_BASE}/manifest.json`);
  let assetName: string, expectedChecksum: string, url: string;
  let expectedSize: number | undefined, declaredBinary: string | undefined;
  if (response.ok) {
    const manifest = parseCliManifest(await response.json());
    const artifact = cliArtifactFor(manifest, process.platform, process.arch);
    assetName = artifact.archive_name; expectedChecksum = artifact.sha256;
    url = `${CLI_DOWNLOAD_BASE}/${artifact.path}`;
    expectedSize = artifact.size_bytes; declaredBinary = artifact.binary;
  } else if (response.status === 404) {
    const checksums = await fetchChecksums();
    assetName = selectPlatformReleaseAssetName(checksums.keys());
    expectedChecksum = checksums.get(assetName) ?? "";
    if (!expectedChecksum) throw new Error(`No checksum for ${assetName}`);
    url = `${CLI_DOWNLOAD_BASE}/${assetName}`;
  } else { throw new Error(`CLI manifest HTTP ${response.status}`); }

  const workDir = join(tmpdir(), `meperstudio-cli-${Date.now()}`);
  await mkdir(workDir, { recursive: true });

  try {
    const archivePath = join(workDir, assetName);
    console.log(`[cli-bootstrap] downloading ${url}`);
    await downloadToFile(url, archivePath);

    console.log(`[cli-bootstrap] verifying ${assetName} against checksums.txt`);
    if (expectedSize !== undefined && (await stat(archivePath)).size !== expectedSize) throw new Error(`CLI archive size mismatch: ${assetName}`);
    await verifyChecksum(archivePath, assetName, expectedChecksum);

    console.log(`[cli-bootstrap] extracting ${assetName}`);
    await extractArchive(archivePath, workDir);

    const branded = join(workDir, binaryName());
    const legacy = join(workDir, process.platform === "win32" ? "silieco.exe" : "silieco");
    const extractedBin = declaredBinary ? join(workDir, declaredBinary) : existsSync(branded) ? branded : legacy;
    if (!existsSync(extractedBin)) {
      throw new Error(
        `archive ${assetName} did not contain ${binaryName()} at its root`,
      );
    }

    await mkdir(dirname(target), { recursive: true });
    const staged = `${target}.new-${Date.now()}`;
    const backup = `${target}.previous-${Date.now()}`;
    try {
      await copyFile(extractedBin, staged);
      await chmod(staged, 0o755);
      if (process.platform === "win32" && existsSync(target)) {
        await rename(target, backup);
        try { await rename(staged, target); } catch (error) { await rename(backup, target); throw error; }
        await rm(backup, { force: true }).catch(() => {});
      } else { await rename(staged, target); }
    } finally { await rm(staged, { force: true }).catch(() => {}); }

    // macOS: ad-hoc sign so spawning the child never hits a gatekeeper quirk.
    // Non-fatal: unsigned binaries still execute when the parent app is trusted.
    if (process.platform === "darwin") {
      await run("codesign", ["-s", "-", "--force", target]).catch((err) => {
        console.warn("[cli-bootstrap] ad-hoc codesign failed:", err);
      });
    }

    console.log(`[cli-bootstrap] installed CLI at ${target}`);
    return target;
  } finally {
    await rm(workDir, { recursive: true, force: true }).catch(() => {});
  }
}

/**
 * Returns the path to a usable `silieco` binary. If one is already present at
 * the managed userData location, returns it immediately. Otherwise downloads
 * the latest release asset for the current platform and installs it.
 */
export async function ensureManagedCli(
  options: { forceInstall?: boolean } = {},
): Promise<string> {
  const target = managedCliPath();
  if (existsSync(target) && !options.forceInstall) return target;
  return installFresh();
}
