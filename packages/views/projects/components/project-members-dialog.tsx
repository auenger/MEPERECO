"use client";

import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { api } from "@silieco/core/api";
import { useWorkspaceId } from "@silieco/core/hooks";
import type { MemberWithUser } from "@silieco/core/types";
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@silieco/ui/components/ui/dialog";
import { Checkbox } from "@silieco/ui/components/ui/checkbox";
import { Button } from "@silieco/ui/components/ui/button";
import { ActorAvatar } from "../../common/actor-avatar";
import { useT } from "../../i18n";
import { toast } from "sonner";

export function ProjectMembersDialog({ open, onOpenChange, projectId, members }: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  projectId: string;
  members: MemberWithUser[];
}) {
  const { t } = useT("projects");
  const wsId = useWorkspaceId();
  const qc = useQueryClient();
  const key = ["projects", wsId, "members", projectId] as const;
  const grants = useQuery({ queryKey: key, queryFn: () => api.listProjectMembers(projectId), enabled: open });
  const update = useMutation({
    mutationFn: ({ userId, granted }: { userId: string; granted: boolean }) => api.setProjectMember(projectId, userId, granted),
    onSuccess: (_, { userId, granted }) => {
      qc.setQueryData<{ user_ids: string[] }>(key, (prev) => ({
        user_ids: granted ? [...new Set([...(prev?.user_ids ?? []), userId])] : (prev?.user_ids ?? []).filter(id => id !== userId),
      }));
      void qc.invalidateQueries({ queryKey: key });
    },
    onError: () => toast.error(t(($) => $.access.failed)),
  });
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="flex max-h-[calc(100dvh-2rem)] flex-col sm:max-w-lg">
        <DialogHeader>
          <DialogTitle>{t(($) => $.access.manage)}</DialogTitle>
          <DialogDescription>{t(($) => $.access.description)}</DialogDescription>
        </DialogHeader>
        <div className="min-h-0 overflow-y-auto" aria-busy={grants.isLoading || update.isPending}>
          {grants.isLoading ? <p className="p-3 text-muted-foreground">{t(($) => $.access.loading)}</p> :
            grants.isError ? <p className="p-3 text-destructive">{t(($) => $.access.load_failed)}</p> :
            members.map((member) => {
              const admin = member.role === "owner" || member.role === "admin";
              return (
                <label key={member.user_id} className="flex min-h-12 cursor-pointer items-center gap-3 rounded-md px-2 py-1.5 hover:bg-accent/50">
                  <Checkbox checked={admin || !!grants.data?.user_ids.includes(member.user_id)} disabled={admin || update.isPending}
                    onCheckedChange={(checked) => update.mutate({ userId: member.user_id, granted: checked === true })} />
                  <ActorAvatar actorType="member" actorId={member.user_id} size="md" profileLink={false} />
                  <span className="min-w-0 flex-1">
                    <span className="block truncate text-body font-medium">{member.name || member.email}</span>
                    <span className="block truncate text-caption text-muted-foreground">{admin ? t(($) => $.access.admin) : member.email}</span>
                  </span>
                </label>
              );
            })}
        </div>
        <DialogFooter><Button onClick={() => onOpenChange(false)}>{t(($) => $.access.done)}</Button></DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
