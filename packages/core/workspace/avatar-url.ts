import { api } from "../api";
import { replaceLegacyBrandAvatar } from "../brand-assets";

export function resolvePublicFileUrlWithBase(rawUrl: string | null | undefined, baseUrl: string): string | null {
  if (!rawUrl) return null;
  if (!rawUrl.startsWith("/")) return replaceLegacyBrandAvatar(rawUrl);
  const trimmedBaseUrl = baseUrl.replace(/\/+$/, "");
  return `${trimmedBaseUrl}${rawUrl}`;
}

export function resolvePublicFileUrl(rawUrl: string | null | undefined): string | null {
  return resolvePublicFileUrlWithBase(rawUrl, api.getBaseUrl());
}
