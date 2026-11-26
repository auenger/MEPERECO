"use client";

import { ExternalLink, Info } from "lucide-react";
import { cn } from "@silieco/ui/lib/utils";
import { useT } from "../../i18n";

/** ZCode's desktop session model is not the headless CLI's model selection. */
export function ZCodeCliNotice({
  provider,
  className,
}: {
  provider?: string | null;
  className?: string;
}) {
  const { t } = useT("runtimes");
  if (provider !== "zcode") return null;

  return (
    <div
      className={cn(
        "flex min-w-0 items-start gap-2 rounded-md border border-info/20 bg-info/5 px-3 py-2 text-caption leading-relaxed text-muted-foreground",
        className,
      )}
      aria-live="polite"
    >
      <Info className="mt-0.5 size-4 shrink-0 text-info" aria-hidden="true" />
      <div className="min-w-0">
        <p className="font-medium text-foreground">{t(($) => $.zcode_cli.title)}</p>
        <p className="mt-0.5">{t(($) => $.zcode_cli.login)}</p>
        <p className="mt-0.5">{t(($) => $.zcode_cli.model)}</p>
        <a
          href="https://usemagpie.ai/"
          target="_blank"
          rel="noopener noreferrer"
          className="mt-1 inline-flex items-center gap-1 font-medium text-primary hover:underline"
        >
          {t(($) => $.zcode_cli.magpie)}
          <ExternalLink className="size-3" aria-hidden="true" />
        </a>
      </div>
    </div>
  );
}
