import { existsSync, readFileSync, statSync } from "node:fs";
import { join } from "node:path";
import { getStaticDownloadRelease } from "@/features/landing/utils/static-downloads";
import { parseCliManifest, type CliReleaseArtifact } from "@silieco/core/release/cli-manifest";
import { DownloadClient } from "./download-client";

export const dynamic = "force-dynamic";

export default function DownloadPage() {
  const root = join(process.cwd(), "public/cli");
  let artifacts: CliReleaseArtifact[] | undefined;
  if (existsSync(join(root, "manifest.json"))) {
    try {
      const manifest = parseCliManifest(JSON.parse(readFileSync(join(root, "manifest.json"), "utf8")));
      artifacts = manifest.artifacts.every(a => { try { return statSync(join(root, a.path)).size === a.size_bytes; } catch { return false; } }) ? manifest.artifacts : [];
    } catch { artifacts = []; }
  }
  return <DownloadClient release={getStaticDownloadRelease()} cliAvailable={artifacts ? artifacts.length > 0 : existsSync(join(root, "checksums.txt"))} cliArtifacts={artifacts} />;
}
