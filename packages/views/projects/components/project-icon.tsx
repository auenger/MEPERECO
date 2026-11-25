"use client";

import { useState } from "react";
import type { Project } from "@silieco/core/types";
import { resolvePublicFileUrl } from "@silieco/core/workspace/avatar-url";
import { cn } from "@silieco/ui/lib/utils";

export type ProjectIconSize = "sm" | "md" | "lg" | "xl";

export interface ProjectIconProps {
  project?: Pick<Project, "icon"> | null;
  size?: ProjectIconSize;
  className?: string;
}

const SIZE_CLASS: Record<ProjectIconSize, string> = {
  sm: "size-3.5 text-caption leading-none",
  md: "size-4 text-body leading-none",
  lg: "size-6 text-display-sm leading-none",
  xl: "size-10 text-display-sm leading-none",
};

export function ProjectIcon({ project, size = "sm", className }: ProjectIconProps) {
  const icon = project?.icon;
  const imageUrl = icon && /^(https?:\/\/|\/uploads\/|\/api\/avatars\/)/i.test(icon)
    ? resolvePublicFileUrl(icon)
    : null;
  const [failedUrl, setFailedUrl] = useState<string | null>(null);
  const showImage = imageUrl && imageUrl !== failedUrl;

  return (
    <span
      aria-hidden="true"
      className={cn(
        "inline-flex shrink-0 items-center justify-center",
        SIZE_CLASS[size],
        imageUrl && "overflow-hidden rounded-full",
        className,
      )}
    >
      {showImage ? (
        <img
          src={imageUrl}
          alt=""
          className="size-full rounded-full object-cover"
          onError={() => setFailedUrl(imageUrl)}
        />
      ) : imageUrl ? "📁" : icon || "📁"}
    </span>
  );
}
