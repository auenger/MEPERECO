"use client";

import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { ExternalLink, FileDiff, FileText, RefreshCw } from "lucide-react";
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle } from "@silieco/ui/components/ui/dialog";
import { Button } from "@silieco/ui/components/ui/button";
import { Input } from "@silieco/ui/components/ui/input";
import { api } from "@silieco/core/api";
import type { GitChangedFile, GitPullRequestItem } from "@silieco/core/types";
import { cn } from "@silieco/ui/lib/utils";
import { useT } from "../i18n";

function FileStatus({ file }: { file: GitChangedFile }) {
  const { t } = useT("git");
  const label = file.status === "added" ? t($ => $.file_added) : file.status === "removed" ? t($ => $.file_removed) : file.status === "renamed" ? t($ => $.file_renamed) : t($ => $.file_modified);
  return <span className={cn("rounded px-1.5 py-0.5 text-micro", file.status === "added" ? "bg-emerald-500/10 text-emerald-600 dark:text-emerald-400" : file.status === "removed" ? "bg-rose-500/10 text-rose-600 dark:text-rose-400" : "bg-muted text-muted-foreground")}>{label}</span>;
}

function PatchViewer({ file }: { file: GitChangedFile }) {
  const { t } = useT("git");
  return <div className="flex min-h-0 min-w-0 flex-col overflow-hidden rounded-lg border border-border">
    <div className="shrink-0 border-b bg-muted/30 px-3 py-2">
      <div className="flex flex-wrap items-center gap-2"><FileStatus file={file} /><span className="min-w-0 break-all font-mono text-caption">{file.path}</span></div>
      {file.previous_path && file.previous_path !== file.path && <p className="mt-1 break-all text-micro text-muted-foreground">{t($ => $.previous_path)}: {file.previous_path}</p>}
      {(file.additions != null || file.deletions != null) && <div className="mt-1 flex gap-2 font-mono text-caption"><span className="text-emerald-600 dark:text-emerald-400">+{file.additions ?? 0}</span><span className="text-rose-600 dark:text-rose-400">−{file.deletions ?? 0}</span></div>}
    </div>
    {file.patch_truncated && <p className="shrink-0 border-b bg-muted/30 px-3 py-2 text-caption text-muted-foreground">{t($ => $.patch_truncated)}</p>}
    {file.patch ? <div className="min-h-0 flex-1 overflow-auto select-text" tabIndex={0} aria-label={t($ => $.diff_title)}><pre className="min-w-max py-2 font-mono text-caption leading-5">{file.patch.split("\n").map((line, index) => <span key={index} className={cn("block whitespace-pre px-3", line.startsWith("@@") ? "bg-muted text-muted-foreground" : line.startsWith("+") && !line.startsWith("+++") ? "bg-emerald-500/10 text-emerald-700 dark:text-emerald-300" : line.startsWith("-") && !line.startsWith("---") ? "bg-rose-500/10 text-rose-700 dark:text-rose-300" : "text-foreground")}>{line || " "}</span>)}</pre></div> : <p className="p-4 text-caption text-muted-foreground">{t($ => $.patch_unavailable)}</p>}
  </div>;
}

export function GitChangesDialog({ wsId, projectId, projectFilter, pr, onClose }: {
  wsId: string; projectId?: string; projectFilter?: string; pr: GitPullRequestItem; onClose: () => void;
}) {
  const { t } = useT("git");
  const [page, setPage] = useState(1);
  const [search, setSearch] = useState("");
  const [selected, setSelected] = useState<string | null>(null);
  const supported = pr.provider === "gitlab" || pr.provider === "github";
  const q = useQuery({ queryKey: ["git", wsId, projectId ?? projectFilter ?? "workspace", "files", pr.id, page], queryFn: () => api.listGitPullRequestFiles(wsId, projectId, pr.id, page, projectFilter), enabled: supported, retry: false, staleTime: 30_000, gcTime: 0 });
  const files = (q.data?.files ?? []).filter(file => `${file.path} ${file.previous_path ?? ""}`.toLowerCase().includes(search.trim().toLowerCase()));
  const current = files.find(file => file.path === selected) ?? files[0];
  const changePage = (next: number) => { setSelected(null); setSearch(""); setPage(next); };
  return <Dialog open onOpenChange={open => { if (!open) onClose(); }}>
    <DialogContent className="flex h-[min(760px,85vh)] max-w-[calc(100%-2rem)] flex-col gap-3 overflow-hidden sm:max-w-[1100px]">
      <DialogHeader className="shrink-0 pr-9"><DialogTitle className="flex items-center gap-2"><FileDiff className="size-5" />{t($ => $.view_changes)}</DialogTitle><DialogDescription className="break-all">{pr.repo_owner}/{pr.repo_name} #{pr.number} · {pr.title}</DialogDescription></DialogHeader>
      <div className="flex shrink-0 flex-wrap items-center justify-between gap-2 border-b pb-3">
        <p className="text-caption text-muted-foreground">{t($ => $.live_changes_hint)}</p>
        <a href={pr.html_url} target="_blank" rel="noreferrer noopener" className="inline-flex items-center gap-1.5 text-caption text-primary hover:underline">{t($ => $.open_platform)}<ExternalLink className="size-3.5" /></a>
      </div>
      {!supported ? <p className="rounded-lg border border-dashed p-4 text-caption text-muted-foreground">{t($ => $.unsupported_provider)}</p> : q.isPending ? <p className="p-4 text-caption text-muted-foreground">{t($ => $.loading_changes)}</p> : q.isError ? <div role="alert" className="rounded-lg border p-4"><p className="text-caption text-muted-foreground">{t($ => $.changes_error)}</p><Button className="mt-3" variant="outline" size="sm" onClick={() => void q.refetch()}><RefreshCw className="size-3.5" />{t($ => $.retry)}</Button></div> : <>
        <div className="flex shrink-0 flex-wrap items-center justify-between gap-2"><Input aria-label={t($ => $.file_filter)} placeholder={t($ => $.file_filter)} className="w-full sm:w-64" value={search} onChange={e => setSearch(e.target.value)} /><span className="text-caption text-muted-foreground">{t($ => $.files_on_page, { count: files.length })}</span></div>
        <div className="grid min-h-0 flex-1 grid-rows-[minmax(0,1fr)_minmax(0,2fr)] gap-3 md:grid-cols-[minmax(0,260px)_minmax(0,1fr)] md:grid-rows-1">
          <div className="min-h-0 overflow-auto rounded-lg border p-1" aria-label={t($ => $.files_title)}>
            {files.length ? files.map(file => <button type="button" key={file.path} aria-pressed={current?.path === file.path} onClick={() => setSelected(file.path)} className={cn("flex w-full items-start gap-2 rounded-md p-2 text-left hover:bg-accent focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring", current?.path === file.path && "bg-accent")}><FileText className="mt-0.5 size-3.5 shrink-0 text-muted-foreground" /><span className="min-w-0 flex-1"><span className="block truncate text-caption" title={file.path}>{file.path.split("/").at(-1)}</span><span className="block truncate text-micro text-muted-foreground" title={file.path}>{file.path}</span></span><FileStatus file={file} /></button>) : <p className="p-3 text-caption text-muted-foreground">{t($ => $.no_files)}</p>}
          </div>
          {current ? <PatchViewer file={current} /> : <div className="grid place-items-center rounded-lg border border-dashed text-caption text-muted-foreground">{t($ => $.no_files)}</div>}
        </div>
        <div className="flex shrink-0 items-center justify-end gap-2"><Button size="sm" variant="outline" disabled={page === 1} onClick={() => changePage(page - 1)}>{t($ => $.previous)}</Button><span className="text-caption tabular-nums text-muted-foreground">{page}</span><Button size="sm" variant="outline" disabled={!q.data.next_page} onClick={() => q.data.next_page && changePage(q.data.next_page)}>{t($ => $.next)}</Button></div>
      </>}
    </DialogContent>
  </Dialog>;
}
