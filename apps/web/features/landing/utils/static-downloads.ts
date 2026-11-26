import { existsSync } from "node:fs";
import { join } from "node:path";
import { parseReleaseAssets, type DownloadAssets } from "./parse-release-assets";
import packageJson from "../../../package.json";

/**
 * Initial installers and automatic updates use the same release directory.
 * Keep historical static aliases as a fallback for older deployments.
 */
export interface StaticDownloadRelease {
  version: string;
  publishedAt: string;
  htmlUrl: string;
  assets: DownloadAssets;
}

const DOWNLOAD_BASE = "/downloads";
const assetFiles = {
  macArm64Dmg: "meperstudio-desktop-mac-arm64.dmg",
  macArm64Zip: "meperstudio-desktop-mac-arm64.zip",
  macX64Dmg: "meperstudio-desktop-mac-x64.dmg",
  macX64Zip: "meperstudio-desktop-mac-x64.zip",
  winX64Zip: "meperstudio-desktop-windows-x64.zip",
  winX64Exe: "meperstudio-desktop-windows-x64.exe",
  winArm64Exe: "meperstudio-desktop-windows-arm64.exe",
} satisfies Partial<Record<keyof DownloadAssets, string>>;

export function getStaticDownloadRelease(): StaticDownloadRelease {
  const directory = process.env.MEPERSTUDIO_DOWNLOAD_DIR || process.env.SILIECO_DOWNLOAD_DIR || join(process.cwd(), "public/downloads");
  const assets: DownloadAssets = {};
  for (const [key, filename] of Object.entries(assetFiles)) {
    const legacy = filename.replace("meperstudio-desktop-", "silieco-desktop-");
    const existing = existsSync(join(directory,filename)) ? filename : existsSync(join(directory,legacy)) ? legacy : null;
    if (existing) assets[key as keyof DownloadAssets] = `${DOWNLOAD_BASE}/${existing}`;
  }
  const version = process.env.MEPERSTUDIO_DESKTOP_VERSION?.trim()
    || process.env.MEPERSTUDIO_DESKTOP_VERSION?.trim() || process.env.SILIECO_DESKTOP_VERSION?.trim()
    || packageJson.version;
  const updateDir = process.env.MEPERSTUDIO_DESKTOP_UPDATE_DIR;
  if (updateDir && /^[0-9]+\.[0-9]+\.[0-9]+$/.test(version)) {
    const releaseFiles = [
      ...["arm64", "x64"].flatMap(arch => ["dmg", "zip"].map(ext =>
        `meperstudio-desktop-${version}-mac-${arch}.${ext}`)),
      ...["arm64", "x64"].map(arch => `meperstudio-desktop-${version}-windows-${arch}.exe`),
      ...["arm64", "amd64", "x86_64", "x64", "aarch64"].flatMap(arch => ["AppImage", "deb", "rpm"].map(ext =>
        `meperstudio-desktop-${version}-linux-${arch}.${ext}`)),
    ];
    Object.assign(assets, parseReleaseAssets(releaseFiles
      .filter(name => existsSync(join(updateDir, name)))
      .map(name => ({ name, browser_download_url: `/updates/desktop/${name}` }))));
  }
  return {
    version,
    publishedAt: new Date().toISOString(),
    htmlUrl: `${DOWNLOAD_BASE}/`,
    assets,
  };
}
