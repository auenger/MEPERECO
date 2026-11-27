/** Shared client-side policy for registration, password reset and password changes. */
export const PASSWORD_POLICY = Object.freeze({ min: 6, maxBytes: 72 });

export function validNewPassword(password: string): boolean {
  let characters = 0;
  let bytes = 0;
  // Count Unicode code points and UTF-8 bytes without depending on TextEncoder
  // availability in React Native. Match the server's bcrypt input boundary.
  for (const character of password) {
    const codePoint = character.codePointAt(0)!;
    if (codePoint === 0 || (codePoint >= 0xd800 && codePoint <= 0xdfff)) return false;
    characters++;
    bytes += codePoint <= 0x7f ? 1 : codePoint <= 0x7ff ? 2 : codePoint <= 0xffff ? 3 : 4;
    if (bytes > PASSWORD_POLICY.maxBytes) return false;
  }
  return characters >= PASSWORD_POLICY.min;
}
