import { existsSync } from "node:fs";
import { homedir } from "node:os";
import { join } from "node:path";

// Must match core/internal/branding.StateDir; do not move live legacy profiles.
export function stateDir(home = homedir()): string {
  const current = join(home, ".meperstudio");
  const legacy = join(home, ".silieco");
  return existsSync(current) || !existsSync(legacy) ? current : legacy;
}
