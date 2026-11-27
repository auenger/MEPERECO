"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { api } from "@silieco/core/api";
import { useWorkspaceId } from "@silieco/core/hooks";
import { projectListOptions } from "@silieco/core/projects/queries";
import { Button } from "@silieco/ui/components/ui/button";
import { Checkbox } from "@silieco/ui/components/ui/checkbox";
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogDescription, DialogFooter } from "@silieco/ui/components/ui/dialog";
import { toast } from "sonner";
import { useT } from "../i18n";

export function MemberProjectAccessDialog({ open, onOpenChange, userId, memberName }: {
  open: boolean;
  onOpenChange: (value: boolean) => void;
  userId: string;
  memberName: string;
}) {
  const { t } = useT("members");
  const wsId = useWorkspaceId();
  const qc = useQueryClient();
  const key = ["projects", wsId, "member-access", userId] as const;
  const projects = useQuery({ ...projectListOptions(wsId), enabled: open });
  const grants = useQuery({ queryKey: key, queryFn: () => api.listMemberProjectAccess(userId), enabled: open });
  const update = useMutation({
    mutationFn: ({ projectId, granted }: { projectId: string; granted: boolean }) => api.setProjectMember(projectId, userId, granted),
    onSuccess: (_, { projectId, granted }) => {
      qc.setQueryData<{ project_ids: string[] }>(key, prev => ({
        project_ids: granted ? [...new Set([...(prev?.project_ids ?? []), projectId])] : (prev?.project_ids ?? []).filter(id => id !== projectId),
      }));
      void qc.invalidateQueries({ queryKey: ["projects", wsId, "members", projectId] });
      void qc.invalidateQueries({ queryKey: key });
    },
    onError: () => toast.error(t(($) => $.project_access.failed)),
  });
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="flex max-h-[calc(100dvh-2rem)] flex-col sm:max-w-lg">
        <DialogHeader>
          <DialogTitle>{memberName} · {t(($) => $.project_access.manage)}</DialogTitle>
          <DialogDescription>{t(($) => $.project_access.description)}</DialogDescription>
        </DialogHeader>
        <div className="min-h-0 overflow-y-auto" aria-busy={projects.isLoading || grants.isLoading || update.isPending}>
          {projects.isLoading || grants.isLoading ? <p className="p-3 text-muted-foreground">{t(($) => $.project_access.loading)}</p>
            : projects.isError || grants.isError ? <p className="p-3 text-destructive">{t(($) => $.project_access.load_failed)}</p>
            : !projects.data?.length ? <p className="p-3 text-muted-foreground">{t(($) => $.project_access.empty)}</p>
            : projects.data.map(project => (
              <label key={project.id} className="flex min-h-12 items-center gap-3 rounded-md px-2 py-1.5 hover:bg-accent/50">
                <Checkbox checked={!!grants.data?.project_ids.includes(project.id)} disabled={update.isPending}
                  onCheckedChange={checked => update.mutate({ projectId: project.id, granted: checked === true })} />
                <span className="min-w-0 flex-1 truncate text-body font-medium">{project.title}</span>
              </label>
            ))}
        </div>
        <DialogFooter><Button onClick={() => onOpenChange(false)}>{t(($) => $.project_access.done)}</Button></DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
