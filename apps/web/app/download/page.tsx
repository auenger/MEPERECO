import { existsSync } from "node:fs";
import { join } from "node:path";
import { getStaticDownloadRelease } from "@/features/landing/utils/static-downloads";
import { DownloadClient } from "./download-client";

export const dynamic = "force-dynamic";

export default function DownloadPage() {
  return <DownloadClient release={getStaticDownloadRelease()} cliAvailable={existsSync(join(process.cwd(), "public/cli/checksums.txt"))} />;
}
