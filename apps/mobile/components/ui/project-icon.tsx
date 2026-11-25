/**
 * Mobile ProjectIcon — RN port of
 * `packages/views/projects/components/project-icon.tsx`. Renders the
 * project's circular image or emoji icon with a 📁 fallback.
 *
 * Why a square View wrapper + explicit lineHeight:
 *   Emoji glyphs do not respect the latin-text baseline metrics — on iOS
 *   they render with a visual extent ~10-15% larger than `fontSize`,
 *   centered on the baseline. `<Text>` clips content to `lineHeight`, so a
 *   `lineHeight: font-size` (the previous `leading-none` form) trimmed the
 *   top and bottom of the emoji. Setting `lineHeight = fontSize * 1.2`
 *   gives the glyph enough vertical room; the surrounding View pins a
 *   stable square footprint so flex parents using `items-center` /
 *   `items-start` align siblings against a predictable box instead of the
 *   emoji's drifting baseline.
 */
import { useState } from "react";
import { View } from "react-native";
import { Image as ExpoImage } from "expo-image";
import { resolveAttachmentUrl } from "@/lib/attachment-url";
import { Text } from "@/components/ui/text";

export type ProjectIconSize = "sm" | "md" | "lg";

const SIZE: Record<ProjectIconSize, { box: number; font: number }> = {
  sm: { box: 18, font: 14 },
  md: { box: 22, font: 16 },
  lg: { box: 28, font: 22 },
};

interface Props {
  icon?: string | null;
  size?: ProjectIconSize;
}

export function isProjectImageIcon(icon?: string | null): boolean {
  return !!icon && /^(https?:\/\/|\/uploads\/|\/api\/avatars\/)/i.test(icon);
}

export function ProjectIcon({ icon, size = "sm" }: Props) {
  const { box, font } = SIZE[size];
  const imageUrl = isProjectImageIcon(icon) ? resolveAttachmentUrl(icon) : null;
  const [failedUrl, setFailedUrl] = useState<string | null>(null);
  if (imageUrl && failedUrl !== imageUrl) {
    return (
      <View style={{ width: box, height: box, borderRadius: box / 2, overflow: "hidden" }}>
        <ExpoImage
          source={{ uri: imageUrl }}
          contentFit="cover"
          style={{ width: box, height: box }}
          onError={() => setFailedUrl(imageUrl)}
        />
      </View>
    );
  }
  return (
    <View
      style={{
        width: box,
        height: box,
        alignItems: "center",
        justifyContent: "center",
      }}
    >
      <Text style={{ fontSize: font, lineHeight: Math.round(font * 1.2) }}>
        {imageUrl ? "📁" : icon || "📁"}
      </Text>
    </View>
  );
}
