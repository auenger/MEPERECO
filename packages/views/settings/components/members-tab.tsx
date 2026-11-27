"use client";

import { useState } from "react";
import { Crown, Shield, User, Plus, MoreHorizontal, UserMinus, Clock, X, Mail, Search, ChevronLeft, ChevronRight } from "lucide-react";
import { ActorAvatar } from "../../common/actor-avatar";
import { MemberProjectAccessDialog } from "../../members/member-project-access-dialog";
import type { MemberWithUser, MemberRole, Invitation } from "@silieco/core/types";
import { Input } from "@silieco/ui/components/ui/input";
import { Button } from "@silieco/ui/components/ui/button";
import { Card, CardContent } from "@silieco/ui/components/ui/card";
import { Badge } from "@silieco/ui/components/ui/badge";
import { Avatar, AvatarFallback, AvatarImage } from "@silieco/ui/components/ui/avatar";
import {
  AlertDialog,
  AlertDialogContent,
  AlertDialogHeader,
  AlertDialogTitle,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogCancel,
  AlertDialogAction,
} from "@silieco/ui/components/ui/alert-dialog";
import {
  Select,
  SelectTrigger,
  SelectValue,
  SelectContent,
  SelectItem,
} from "@silieco/ui/components/ui/select";
import {
  DropdownMenu,
  DropdownMenuTrigger,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuSub,
  DropdownMenuSubTrigger,
  DropdownMenuSubContent,
} from "@silieco/ui/components/ui/dropdown-menu";
import { toast } from "sonner";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { useAuthStore } from "@silieco/core/auth";
import { useWorkspaceId } from "@silieco/core/hooks";
import { useCurrentWorkspace } from "@silieco/core/paths";
import { memberListOptions, invitationListOptions, inviteCandidatesOptions, workspaceKeys } from "@silieco/core/workspace/queries";
import { resolvePublicFileUrl } from "@silieco/core/workspace/avatar-url";
import { api } from "@silieco/core/api";
import { useT } from "../../i18n";
import { SettingsCard, SettingsSection, SettingsTab } from "./settings-layout";
import { useDebouncedValue } from "../../common/use-debounced-value";

const ROLE_ICONS: Record<MemberRole, typeof Crown> = {
  owner: Crown,
  admin: Shield,
  member: User,
};

function useRoleLabels() {
  const { t } = useT("settings");
  return {
    owner: {
      label: t(($) => $.members.roles.owner.label),
      description: t(($) => $.members.roles.owner.description),
      icon: ROLE_ICONS.owner,
    },
    admin: {
      label: t(($) => $.members.roles.admin.label),
      description: t(($) => $.members.roles.admin.description),
      icon: ROLE_ICONS.admin,
    },
    member: {
      label: t(($) => $.members.roles.member.label),
      description: t(($) => $.members.roles.member.description),
      icon: ROLE_ICONS.member,
    },
  } as const;
}

function MemberRow({
  member,
  canManage,
  canManageOwners,
  ownerCount,
  isSelf,
  busy,
  onRoleChange,
  onRemove,
}: {
  member: MemberWithUser;
  canManage: boolean;
  canManageOwners: boolean;
  /** Total number of owners in this workspace — needed to gate demoting the
   *  last owner per `workspace.go:497-507`. */
  ownerCount: number;
  isSelf: boolean;
  busy: boolean;
  onRoleChange: (role: MemberRole) => void;
  onRemove: () => void;
}) {
  const { t } = useT("settings");
  const roleConfig = useRoleLabels();
  const rc = roleConfig[member.role];
  const RoleIcon = rc.icon;
  const canEditRole = canManage && !isSelf && (member.role !== "owner" || canManageOwners);
  const canRemove = canManage && !isSelf && (member.role !== "owner" || canManageOwners);
  const isLastOwner = member.role === "owner" && ownerCount <= 1;
  const showMenu = canEditRole || canRemove;
  const canManageProjectAccess = canManage && member.role === "member";
  const [projectAccessOpen, setProjectAccessOpen] = useState(false);

  return (
    <div className="flex items-center gap-3 px-4 py-3">
      <ActorAvatar actorType="member" actorId={member.user_id} size="lg" />
      <div className="min-w-0 flex-1">
        <div className="text-body font-medium truncate">{member.name}</div>
        <div className="text-caption text-muted-foreground truncate">{member.email}</div>
      </div>
      {canManageProjectAccess && (
        <Button variant="outline" size="sm" disabled={busy} onClick={() => setProjectAccessOpen(true)}>
          <Shield className="h-3.5 w-3.5" />
          {t(($) => $.members.project_access)}
        </Button>
      )}
      {canManageProjectAccess && projectAccessOpen && (
        <MemberProjectAccessDialog
          open={projectAccessOpen}
          onOpenChange={setProjectAccessOpen}
          userId={member.user_id}
          memberName={member.name}
        />
      )}
      {showMenu && (
        <DropdownMenu>
          <DropdownMenuTrigger
            render={
              <Button variant="ghost" size="icon-sm" disabled={busy}>
                <MoreHorizontal className="h-4 w-4 text-muted-foreground" />
              </Button>
            }
          />
          <DropdownMenuContent align="end" className="w-auto">
            {canEditRole && (
              <DropdownMenuSub>
                <DropdownMenuSubTrigger>
                  <Shield className="h-3.5 w-3.5" />
                  {t(($) => $.members.change_role)}
                </DropdownMenuSubTrigger>
                <DropdownMenuSubContent className="w-auto">
                  {(Object.entries(roleConfig) as [MemberRole, (typeof roleConfig)[MemberRole]][]).map(
                    ([role, config]) => {
                      if (role === "owner" && !canManageOwners) return null;
                      const Icon = config.icon;
                      const wouldDemoteLastOwner =
                        isLastOwner && role !== "owner";
                      return (
                        <DropdownMenuItem
                          key={role}
                          onClick={() =>
                            wouldDemoteLastOwner ? undefined : onRoleChange(role)
                          }
                          disabled={wouldDemoteLastOwner}
                          title={
                            wouldDemoteLastOwner
                              ? t(($) => $.members.cannot_demote_last_owner_title)
                              : undefined
                          }
                        >
                          <Icon className="h-3.5 w-3.5" />
                          <div className="flex flex-col">
                            <span>{config.label}</span>
                            <span className="text-caption text-muted-foreground font-normal">
                              {wouldDemoteLastOwner
                                ? t(($) => $.members.cannot_demote_last_owner)
                                : config.description}
                            </span>
                          </div>
                          {member.role === role && (
                            <span className="ml-auto text-caption text-muted-foreground">{"✓"}</span>
                          )}
                        </DropdownMenuItem>
                      );
                    }
                  )}
                </DropdownMenuSubContent>
              </DropdownMenuSub>
            )}
            {canEditRole && canRemove && <DropdownMenuSeparator />}
            {canRemove && (
              <DropdownMenuItem variant="destructive" onClick={onRemove}>
                <UserMinus className="h-3.5 w-3.5" />
                {t(($) => $.members.remove_action)}
              </DropdownMenuItem>
            )}
          </DropdownMenuContent>
        </DropdownMenu>
      )}
      <Badge variant="secondary">
        <RoleIcon className="h-3 w-3" />
        {rc.label}
      </Badge>
    </div>
  );
}

function InvitationRow({
  invitation,
  canManage,
  onRevoke,
  busy,
}: {
  invitation: Invitation;
  canManage: boolean;
  onRevoke: () => void;
  busy: boolean;
}) {
  const { t } = useT("settings");
  const roleConfig = useRoleLabels();
  const rc = roleConfig[invitation.role];

  return (
    <div className="flex items-center gap-3 px-4 py-3">
      <div className="flex h-8 w-8 items-center justify-center rounded-full bg-muted">
        <Mail className="h-4 w-4 text-muted-foreground" />
      </div>
      <div className="min-w-0 flex-1">
        <div className="text-body font-medium truncate">{invitation.invitee_email}</div>
        <div className="flex items-center gap-1 text-caption text-muted-foreground">
          <Clock className="h-3 w-3" />
          <span>{t(($) => $.members.pending_status)}</span>
        </div>
      </div>
      {canManage && (
        <Button
          variant="ghost"
          size="icon-sm"
          disabled={busy}
          onClick={onRevoke}
          title={t(($) => $.members.revoke_invitation_tooltip)}
        >
          <X className="h-4 w-4 text-muted-foreground" />
        </Button>
      )}
      <Badge variant="outline">
        {rc.label}
      </Badge>
    </div>
  );
}

export function MembersTab() {
  const { t } = useT("settings");
  const roleConfig = useRoleLabels();
  const user = useAuthStore((s) => s.user);
  const workspace = useCurrentWorkspace();
  const qc = useQueryClient();
  const wsId = useWorkspaceId();
  const { data: members = [] } = useQuery(memberListOptions(wsId));
  const { data: invitations = [] } = useQuery(invitationListOptions(wsId));

  const [inviteSearch, setInviteSearch] = useState("");
  const [invitePage, setInvitePage] = useState(0);
  const [inviteRole, setInviteRole] = useState<MemberRole>("member");
  const [invitingEmail, setInvitingEmail] = useState<string | null>(null);
  const [memberActionId, setMemberActionId] = useState<string | null>(null);
  const [invitationActionId, setInvitationActionId] = useState<string | null>(null);
  const [confirmAction, setConfirmAction] = useState<{
    title: string;
    description: string;
    variant?: "destructive";
    onConfirm: () => Promise<void>;
  } | null>(null);

  const currentMember = members.find((m) => m.user_id === user?.id) ?? null;
  const canManageWorkspace = currentMember?.role === "owner" || currentMember?.role === "admin";
  const isOwner = currentMember?.role === "owner";
  const ownerCount = members.filter((m) => m.role === "owner").length;
  const searchTerm = inviteSearch.trim().toLowerCase();
  const debouncedSearch = useDebouncedValue(searchTerm, 250);
  const searchReady = Array.from(searchTerm.replace(/^@/, "")).length >= 2;
  const searchSettled = searchTerm === debouncedSearch;
  const candidates = useQuery({
    ...inviteCandidatesOptions(wsId, debouncedSearch, invitePage),
    enabled: !!wsId && canManageWorkspace && searchReady && searchSettled,
  });
  const unavailableEmails = new Set([
    ...members.map((member) => member.email.toLowerCase()),
    ...invitations.map((invitation) => invitation.invitee_email.toLowerCase()),
  ]);
  const matchingCandidates = (candidates.data?.items ?? []).filter(
    (candidate) => !unavailableEmails.has(candidate.email.toLowerCase()),
  );
  const isCompleteEmail = /^[^\s@<>]+@[^\s@<>]+$/.test(searchTerm) && searchTerm.length <= 254;
  const canInviteEmail = isCompleteEmail && !unavailableEmails.has(searchTerm);

  const handleInviteMember = async (email: string, clearSearch = false) => {
    if (!workspace || invitingEmail || unavailableEmails.has(email.toLowerCase())) return;
    setInvitingEmail(email);
    try {
      const invitation = await api.createMember(workspace.id, {
        email,
        role: inviteRole,
      });
      qc.setQueryData<Invitation[]>(workspaceKeys.invitations(wsId), (previous = []) => [invitation, ...previous.filter((item) => item.id !== invitation.id)]);
      if (clearSearch) setInviteSearch("");
      setInvitePage(0);
      await Promise.all([
        qc.invalidateQueries({ queryKey: workspaceKeys.invitations(wsId) }),
        qc.invalidateQueries({ queryKey: workspaceKeys.inviteCandidates(wsId) }),
      ]);
      toast.success(t(($) => $.members.toast_invitation_sent));
    } catch (e) {
      toast.error(e instanceof Error ? e.message : t(($) => $.members.toast_invitation_failed));
    } finally {
      setInvitingEmail(null);
    }
  };

  const handleRevokeInvitation = (invitation: Invitation) => {
    if (!workspace) return;
    setConfirmAction({
      title: t(($) => $.members.revoke_invitation_title),
      description: t(($) => $.members.revoke_invitation_description, { email: invitation.invitee_email }),
      variant: "destructive",
      onConfirm: async () => {
        setInvitationActionId(invitation.id);
        try {
          await api.revokeInvitation(workspace.id, invitation.id);
          qc.invalidateQueries({ queryKey: workspaceKeys.invitations(wsId) });
          qc.invalidateQueries({ queryKey: workspaceKeys.inviteCandidates(wsId) });
          toast.success(t(($) => $.members.toast_invitation_revoked));
        } catch (e) {
          toast.error(e instanceof Error ? e.message : t(($) => $.members.toast_invitation_revoke_failed));
        } finally {
          setInvitationActionId(null);
        }
      },
    });
  };

  const handleRoleChange = async (memberId: string, role: MemberRole) => {
    if (!workspace) return;
    setMemberActionId(memberId);
    try {
      await api.updateMember(workspace.id, memberId, { role });
      qc.invalidateQueries({ queryKey: workspaceKeys.members(wsId) });
      toast.success(t(($) => $.members.toast_role_updated));
    } catch (e) {
      toast.error(e instanceof Error ? e.message : t(($) => $.members.toast_role_failed));
    } finally {
      setMemberActionId(null);
    }
  };

  const handleRemoveMember = (member: MemberWithUser) => {
    if (!workspace) return;
    setConfirmAction({
      title: t(($) => $.members.remove_member_title, { name: member.name }),
      description: t(($) => $.members.remove_member_description, { name: member.name, workspace: workspace.name }),
      variant: "destructive",
      onConfirm: async () => {
        setMemberActionId(member.id);
        try {
          await api.deleteMember(workspace.id, member.id);
          qc.invalidateQueries({ queryKey: workspaceKeys.members(wsId) });
          qc.invalidateQueries({ queryKey: workspaceKeys.inviteCandidates(wsId) });
          toast.success(t(($) => $.members.toast_member_removed));
        } catch (e) {
          toast.error(e instanceof Error ? e.message : t(($) => $.members.toast_member_remove_failed));
        } finally {
          setMemberActionId(null);
        }
      },
    });
  };

  if (!workspace) return null;

  return (
    <SettingsTab title={t(($) => $.page.tabs.members)}>
      <SettingsSection title={t(($) => $.members.section_title, { count: members.length })}>

        {canManageWorkspace && (
          <Card>
            <CardContent className="space-y-3">
              <div className="flex items-center gap-2">
                <Plus className="h-4 w-4 text-muted-foreground" />
                <h3 className="text-body font-medium">{t(($) => $.members.invite_title)}</h3>
              </div>
              <div className="grid gap-3 sm:grid-cols-[1fr_120px_auto]">
                <div className="relative min-w-0">
                  <Search aria-hidden className="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
                  <Input
                    type="search"
                    className="pl-9"
                    name="invite-search"
                    autoComplete="off"
                    spellCheck={false}
                    maxLength={254}
                    aria-label={t(($) => $.members.invite_search_placeholder)}
                    value={inviteSearch}
                    disabled={!!invitingEmail}
                    onChange={(e) => { setInviteSearch(e.target.value); setInvitePage(0); }}
                    placeholder={t(($) => $.members.invite_search_placeholder)}
                    onKeyDown={(e) => {
                      if (e.key === "Enter" && !e.nativeEvent.isComposing && canInviteEmail) {
                        e.preventDefault();
                        void handleInviteMember(searchTerm, true);
                      }
                    }}
                  />
                </div>
                <Select
                  items={(["member", "admin"] as const).map((value) => ({
                    value,
                    label: roleConfig[value].label,
                  }))}
                  value={inviteRole}
                  disabled={!!invitingEmail}
                  onValueChange={(value) => setInviteRole(value as MemberRole)}
                >
                  <SelectTrigger size="default" className="w-full" aria-label={t(($) => $.members.invite_role_label)}>
                    <SelectValue>{() => roleConfig[inviteRole].label}</SelectValue>
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="member">{roleConfig.member.label}</SelectItem>
                    <SelectItem value="admin">{roleConfig.admin.label}</SelectItem>
                  </SelectContent>
                </Select>
                <Button
                  onClick={() => handleInviteMember(searchTerm, true)}
                  disabled={!!invitingEmail || !canInviteEmail}
                >
                  {invitingEmail === searchTerm ? t(($) => $.members.inviting) : t(($) => $.members.invite_button)}
                </Button>
              </div>
              <p className="text-caption text-muted-foreground">{t(($) => $.members.invite_search_hint)}</p>
              {isCompleteEmail && !canInviteEmail && (
                <p role="status" className="text-caption text-muted-foreground">{t(($) => $.members.invite_unavailable)}</p>
              )}
              {searchReady && (
                <div className="overflow-hidden rounded-md border" aria-busy={!searchSettled || candidates.isFetching}>
                  <div className="bg-muted/40 px-3 py-2 text-caption font-medium text-muted-foreground">
                    {t(($) => $.members.invite_search_results)}
                  </div>
                  {!searchSettled || candidates.isPending ? (
                    <p role="status" className="px-3 py-3 text-body text-muted-foreground">{t(($) => $.members.invite_searching)}</p>
                  ) : candidates.isError ? (
                    <div className="flex flex-wrap items-center justify-between gap-2 px-3 py-3">
                      <p role="status" className="text-body text-muted-foreground">{t(($) => $.members.invite_search_error)}</p>
                      <Button variant="outline" size="sm" onClick={() => candidates.refetch()}>{t(($) => $.members.invite_search_retry)}</Button>
                    </div>
                  ) : matchingCandidates.length === 0 ? (
                    <p role="status" className="px-3 py-3 text-body text-muted-foreground">{t(($) => $.members.invite_search_empty)}</p>
                  ) : (
                    <div className="max-h-80 divide-y overflow-y-auto">
                      {matchingCandidates.map((candidate) => (
                        <div key={candidate.id} className="flex min-w-0 items-center gap-3 px-3 py-2">
                          <Avatar>
                            <AvatarImage src={resolvePublicFileUrl(candidate.avatar_url) ?? undefined} alt="" />
                            <AvatarFallback>{Array.from(candidate.name.trim() || candidate.email)[0]?.toUpperCase()}</AvatarFallback>
                          </Avatar>
                          <div className="min-w-0 flex-1">
                            <div className="truncate text-body font-medium">{candidate.name || candidate.email}</div>
                            <div className="truncate text-caption text-muted-foreground" title={candidate.email}>{candidate.email}</div>
                          </div>
                          <Button
                            variant="outline"
                            size="sm"
                            disabled={!!invitingEmail}
                            aria-label={t(($) => $.members.invite_candidate_action, { email: candidate.email })}
                            onClick={() => handleInviteMember(candidate.email)}
                          >
                            {invitingEmail === candidate.email ? t(($) => $.members.inviting) : t(($) => $.members.invite_button)}
                          </Button>
                        </div>
                      ))}
                    </div>
                  )}
                  {searchSettled && !candidates.isPending && !candidates.isError && (invitePage > 0 || candidates.data?.has_more) && (
                    <div className="flex items-center justify-end gap-2 border-t px-3 py-2">
                      <Button variant="ghost" size="sm" disabled={!!invitingEmail || candidates.isFetching || invitePage === 0} onClick={() => setInvitePage((page) => page - 1)}>
                        <ChevronLeft aria-hidden className="h-4 w-4" />{t(($) => $.members.invite_previous_page)}
                      </Button>
                      <span className="text-caption text-muted-foreground">{invitePage + 1}</span>
                      <Button variant="ghost" size="sm" disabled={!!invitingEmail || candidates.isFetching || !candidates.data?.has_more} onClick={() => setInvitePage((page) => page + 1)}>
                        {t(($) => $.members.invite_next_page)}<ChevronRight aria-hidden className="h-4 w-4" />
                      </Button>
                    </div>
                  )}
                </div>
              )}
            </CardContent>
          </Card>
        )}

        {members.length > 0 ? (
          <SettingsCard>
            {members.map((m) => (
              <div key={m.id}>
                <MemberRow
                  member={m}
                  canManage={canManageWorkspace}
                  canManageOwners={isOwner}
                  ownerCount={ownerCount}
                  isSelf={m.user_id === user?.id}
                  busy={memberActionId === m.id}
                  onRoleChange={(role) => handleRoleChange(m.id, role)}
                  onRemove={() => handleRemoveMember(m)}
                />
              </div>
            ))}
          </SettingsCard>
        ) : (
          <p className="text-body text-muted-foreground">{t(($) => $.members.no_members)}</p>
        )}
      </SettingsSection>

      {invitations.length > 0 && (
        <SettingsSection title={t(($) => $.members.pending_title, { count: invitations.length })}>
          <SettingsCard>
            {invitations.map((inv) => (
              <div key={inv.id}>
                <InvitationRow
                  invitation={inv}
                  canManage={canManageWorkspace}
                  onRevoke={() => handleRevokeInvitation(inv)}
                  busy={invitationActionId === inv.id}
                />
              </div>
            ))}
          </SettingsCard>
        </SettingsSection>
      )}

      <AlertDialog open={!!confirmAction} onOpenChange={(v) => { if (!v) setConfirmAction(null); }}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{confirmAction?.title}</AlertDialogTitle>
            <AlertDialogDescription>{confirmAction?.description}</AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>{t(($) => $.members.confirm_cancel)}</AlertDialogCancel>
            <AlertDialogAction
              variant={confirmAction?.variant === "destructive" ? "destructive" : "default"}
              onClick={async () => {
                await confirmAction?.onConfirm();
                setConfirmAction(null);
              }}
            >
              {t(($) => $.members.confirm_action)}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </SettingsTab>
  );
}
