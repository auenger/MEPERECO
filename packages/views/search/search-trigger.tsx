"use client";

import { Search } from "lucide-react";
import { SidebarMenuButton } from "@silieco/ui/components/ui/sidebar";
import {
  useShortcut,
} from "@silieco/core/shortcuts";
import { useSearchStore } from "./search-store";
import { useT } from "../i18n";
import { ShortcutKeycaps } from "../common/shortcut-keycaps";

export function SearchTrigger() {
  const { t } = useT("search");
  const shortcut = useShortcut("openSearch");
  return (
    <SidebarMenuButton
      tooltip={t(($) => $.trigger.label)}
      aria-label={t(($) => $.trigger.label)}
      className="text-muted-foreground"
      onClick={() => useSearchStore.getState().setOpen(true)}
    >
      <Search />
      <span className="group-data-[collapsible=icon]:hidden">{t(($) => $.trigger.label)}</span>
      {shortcut ? (
        <ShortcutKeycaps shortcut={shortcut} decorative className="pointer-events-none ml-auto group-data-[collapsible=icon]:hidden" />
      ) : null}
    </SidebarMenuButton>
  );
}
