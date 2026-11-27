"use client";

import { create } from "zustand";
import { createJSONStorage, persist } from "zustand/middleware";
import { defaultStorage } from "../platform/storage";
import { parseProjectTabSurface, parseTabSubject } from "../paths/tab-subject";

interface SopVisibilityState {
  enabled: boolean;
  setEnabled: (enabled: boolean) => void;
}

/** Device-local display preference, not a permission or workflow execution flag. */
export const useSopVisibilityStore = create<SopVisibilityState>()(
  persist(
    (set) => ({ enabled: false, setEnabled: (enabled) => set({ enabled }) }),
    {
      name: "silieco_tr1_sop_visibility",
      storage: createJSONStorage(() => defaultStorage),
      partialize: (state) => ({ enabled: state.enabled }),
    },
  ),
);

/** Identify SOP-only tabs without hiding the ordinary project Task or Resources tabs. */
export function isSopRoute(url: string): boolean {
  const subject = parseTabSubject(url);
  if (subject.kind === "page" && subject.page === "workflows") return true;
  const surface = parseProjectTabSurface(url);
  return surface?.kind === "sop" || surface?.kind === "run";
}
