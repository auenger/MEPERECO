export const WORKSPACE_SLUG_REGEX = /^[a-z0-9]+(?:-[a-z0-9]+)*$/;

/** Generate one stable fallback per creation form, including on HTTP self-hosts. */
export function createWorkspaceSlugFallback(): string {
  const bytes = crypto.getRandomValues(new Uint8Array(6));
  return `workspace-${Array.from(bytes, (byte) => byte.toString(16).padStart(2, "0")).join("")}`;
}

/** Keep display names unrestricted while generating an ASCII URL identifier. */
export function nameToWorkspaceSlug(name: string, fallback: string): string {
  if (!name.trim()) return "";
  const slug = name
    .normalize("NFKC")
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, "-")
    .replace(/^-|-$/g, "");
  return slug || fallback;
}

export function isWorkspaceSlugConflict(error: unknown): boolean {
  return (
    typeof error === "object" &&
    error !== null &&
    "status" in error &&
    (error as { status?: unknown }).status === 409
  );
}
