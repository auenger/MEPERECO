/** Public, content-addressed CLI distribution manifest. The build script is
 * the sole producer; clients must validate before using a path or checksum. */
export interface CliReleaseArtifact {
  os: string; arch: string; path: string; archive_name: string;
  binary: string; sha256: string; size_bytes: number;
}
export interface CliReleaseManifest {
  schema_version: "meperstudio.cli/v1";
  version: string;
  artifacts: CliReleaseArtifact[];
}
export function parseCliManifest(value: unknown): CliReleaseManifest {
  if (!value || typeof value !== "object") throw new Error("Invalid CLI manifest");
  const m = value as Partial<CliReleaseManifest>;
  if (m.schema_version !== "meperstudio.cli/v1" || typeof m.version !== "string" || !/^\d+\.\d+\.\d+$/.test(m.version) || !Array.isArray(m.artifacts) || !m.artifacts.length) throw new Error("Invalid CLI manifest version");
  const seen = new Set<string>();
  for (const a of m.artifacts) {
    if (!a || typeof a.os !== "string" || !a.os || typeof a.arch !== "string" || !a.arch) throw new Error("Missing CLI platform");
    const key = `${a.os}/${a.arch}`;
    if (seen.has(key) || typeof a.sha256 !== "string" || !/^[0-9a-f]{64}$/.test(a.sha256) || !Number.isSafeInteger(a.size_bytes) || a.size_bytes <= 0 || typeof a.archive_name !== "string" || !/^[A-Za-z0-9._-]+$/.test(a.archive_name) || [".",".."].includes(a.archive_name) || a.binary !== (a.os === "windows" ? "meperstudio.exe" : "meperstudio") || a.path !== `releases/${m.version}/${a.sha256}/${a.archive_name}`) throw new Error(`Invalid CLI artifact: ${key}`);
    seen.add(key);
  }
  return m as CliReleaseManifest;
}
export function cliArtifactFor(m: CliReleaseManifest, platform: string, architecture: string): CliReleaseArtifact {
  const os = platform === "win32" ? "windows" : platform;
  const arch = architecture === "x64" ? "amd64" : architecture;
  const artifact = m.artifacts.find(a => a.os === os && a.arch === arch);
  if (!artifact) throw new Error(`No CLI artifact for ${os}/${arch}`);
  return artifact;
}
