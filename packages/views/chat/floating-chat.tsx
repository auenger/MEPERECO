"use client";

import { createPortal } from "react-dom";
import { useWorkflowChatEditorStore } from "@silieco/core/workflows";
import { useWorkspaceId } from "@silieco/core/hooks";
import { useChatStore } from "@silieco/core/chat";
import { useWorkspacePaths } from "@silieco/core/paths";
import { browserSessionFromPath } from "@silieco/core/page-feedback/protocol";
import { useNavigation } from "../navigation";
import { ChatFab } from "./components/chat-fab";
import { ChatWindow } from "./components/chat-window";

/**
 * Mount point for the floating chat overlay (FAB + window). Rendered once in
 * each app shell's dashboard layout; owns the two gates that decide whether the
 * overlay exists at all:
 *
 *  1. The Settings → Chat preference (`floatingChatEnabled`). When a user turns
 *     the floating window off, Chat lives only in its dedicated tab.
 *  2. The Chat tab route itself. On `/:slug/chat` the full-page surface already
 *     owns the conversation, so a floating copy of the same `activeSessionId`
 *     would be pure duplication — hide it there.
 */
export function FloatingChat({ showLauncher = true }: { showLauncher?: boolean }) {
  const wsId = useWorkspaceId();
  const editor = useWorkflowChatEditorStore((s) => s.editor);
  const target = editor?.wsId === wsId ? document.getElementById(editor.portalId) : null;
  const enabled = useChatStore((s) => s.floatingChatEnabled);
  const { pathname } = useNavigation();
  const wsPaths = useWorkspacePaths();

  if (!enabled && !target) return null;
  // Suppress on the Chat tab — it renders the same conversation full-page.
  if (pathname === wsPaths.chat() || pathname.startsWith(`${wsPaths.chat()}/`)) {
    return null;
  }

  if (target) return createPortal(<ChatWindow />, target);

  return (
    <>
      <ChatWindow anchor={showLauncher ? "bottom" : "top"} />
      {showLauncher && !browserSessionFromPath(pathname) && <ChatFab />}
    </>
  );
}

/** Compact launcher for a platform-owned toolbar; the conversation stays shared. */
export function ChatToolbarButton() {
  const enabled = useChatStore((s) => s.floatingChatEnabled);
  const { pathname } = useNavigation();
  const p = useWorkspacePaths();
  if (!enabled || pathname === p.chat() || pathname.startsWith(`${p.chat()}/`)) return null;
  return <ChatFab placement="toolbar" />;
}
