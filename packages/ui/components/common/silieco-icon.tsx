import { useEffect, useState } from "react";
import { cn } from "../../lib/utils";
import { MEPER_APP_ICON_DATA_URL, MEPER_MARK_DATA_URL, MEPER_SMALL_MARK_DATA_URL } from "@silieco/core/brand-assets";

interface SiliecoIconProps extends React.ComponentProps<"span"> {
  animate?: boolean;
  noSpin?: boolean;
  bordered?: boolean;
  size?: "sm" | "md" | "lg";
  variant?: "mark" | "small" | "app";
}

const borderedSizes = {
  sm: { wrapper: "p-1.5", icon: "size-3.5" },
  md: { wrapper: "p-2", icon: "size-4" },
  lg: { wrapper: "p-2.5", icon: "size-5" },
};

function MEPERMark({ className, variant }: { className?: string; variant: "mark" | "small" | "app" }) {
  const src = variant === "app" ? MEPER_APP_ICON_DATA_URL : variant === "small" ? MEPER_SMALL_MARK_DATA_URL : MEPER_MARK_DATA_URL;
  return (
    <img
      aria-hidden="true"
      alt=""
      draggable={false}
      src={src}
      className={cn("object-contain", variant === "small" && "dark:brightness-0 dark:invert", className)}
    />
  );
}

export function SiliecoIcon({
  className,
  animate = false,
  noSpin = false,
  bordered = false,
  size = "sm",
  variant = bordered ? "app" : "mark",
  ...props
}: SiliecoIconProps) {
  const [entranceDone, setEntranceDone] = useState(!animate);

  useEffect(() => {
    if (!animate) return;
    const timer = setTimeout(() => setEntranceDone(true), 600);
    return () => clearTimeout(timer);
  }, [animate]);

  const animationClass = cn(
    !entranceDone && "animate-entrance-spin",
    entranceDone && !noSpin && "hover:animate-spin",
  );

  if (bordered) {
    const sizeConfig = borderedSizes[size];
    return (
      <span
        aria-hidden="true"
        className={cn(
          "inline-flex items-center justify-center rounded-md border border-border",
          sizeConfig.wrapper,
          className,
        )}
        {...props}
      >
        <MEPERMark variant={variant} className={cn(sizeConfig.icon, animationClass)} />
      </span>
    );
  }

  return (
    <span
      aria-hidden="true"
      className={cn("inline-block size-[1em]", className)}
      {...props}
    >
      <MEPERMark variant={variant} className={cn("size-full", animationClass)} />
    </span>
  );
}
