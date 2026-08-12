import { useMemo, useState } from "react";
import { Users, EnvelopeSimple } from "@phosphor-icons/react";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Badge } from "@/components/ui/badge";
import { Skeleton } from "@/components/ui/skeleton";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { RowActions } from "@/components/common/RowActions";
import { api } from "@/lib/api";
import { apiErrorMessage } from "@/lib/apiErrors";
import { getCachedAccountEmail } from "@/lib/authAccount";
import { getInitials } from "@/lib/formatters";
import { useTranslation } from "react-i18next";
import { useAsyncData } from "@/hooks/useAsyncData";
import { toast } from "sonner";
import type { WorkspaceMember } from "@/types";

const isValidEmail = (email: string) => /^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(email);

type InviteRole = Exclude<WorkspaceMember["role"], "owner">;

const INVITE_ROLES: InviteRole[] = ["admin", "member", "guest"];

function canManageTargetRole(actorRole: WorkspaceMember["role"] | undefined, targetRole: WorkspaceMember["role"]): boolean {
  if (!actorRole || targetRole === "owner") return false;
  if (actorRole === "owner") return true;
  return actorRole === "admin" && (targetRole === "member" || targetRole === "guest");
}

function assignableInviteRoles(actorRole: WorkspaceMember["role"] | undefined): InviteRole[] {
  return INVITE_ROLES.filter((role) => canManageTargetRole(actorRole, role));
}

export function SettingsMembersPage() {
  const { t } = useTranslation("settings");
  const { t: tc } = useTranslation("common");
  const { data: members = [], loading, error, refetch } = useAsyncData(
    () => api.getWorkspaceMembers().then((res) => res.data),
    []
  );
  const [email, setEmail] = useState("");
  const [role, setRole] = useState<InviteRole>("member");
  const [inviting, setInviting] = useState(false);
  const [editTarget, setEditTarget] = useState<WorkspaceMember | null>(null);
  const [editRole, setEditRole] = useState<InviteRole>("member");
  const [removeTarget, setRemoveTarget] = useState<WorkspaceMember | null>(null);
  const [saving, setSaving] = useState(false);

  const selfEmail = getCachedAccountEmail()?.toLowerCase();
  const actor = useMemo(
    () => (members ?? []).find((member) => member.email.toLowerCase() === selfEmail),
    [members, selfEmail],
  );
  const actorRole = actor?.role ?? (selfEmail ? undefined : "owner");
  const inviteRoles = assignableInviteRoles(actorRole);

  const trimmedEmail = email.trim();
  const canInvite = isValidEmail(trimmedEmail) && !inviting && canManageTargetRole(actorRole, role);

  const handleInvite = async () => {
    if (!trimmedEmail) return;
    if (!isValidEmail(trimmedEmail)) {
      toast.error(t("members.invalidEmail"));
      return;
    }
    if (!canManageTargetRole(actorRole, role)) {
      toast.error(t("members.cannotManageMember"));
      return;
    }

    setInviting(true);
    try {
      const normalizedEmail = trimmedEmail.toLowerCase();
      await api.inviteWorkspaceMember(normalizedEmail, role);
      toast.success(t("members.invited", { email: normalizedEmail }));
      setEmail("");
      setRole("member");
      refetch();
    } catch (err) {
      toast.error(
        apiErrorMessage(err, {
          fallback: "saveFailed",
          messageKey: "settings:members.inviteFailed",
        }),
      );
    } finally {
      setInviting(false);
    }
  };

  const openEdit = (member: WorkspaceMember) => {
    setEditTarget(member);
    setEditRole(member.role === "owner" ? "member" : member.role);
  };

  const handleSaveRole = async () => {
    if (!editTarget) return;
    if (!canManageTargetRole(actorRole, editRole)) {
      toast.error(t("members.cannotManageMember"));
      return;
    }
    setSaving(true);
    try {
      if (editTarget.status === "pending") {
        await api.updateWorkspaceInvitationRole(editTarget.id, editRole);
      } else {
        await api.updateWorkspaceMemberRole(editTarget.userId || editTarget.id, editRole);
      }
      toast.success(t("members.roleUpdated", { email: editTarget.email }));
      setEditTarget(null);
      refetch();
    } catch (err) {
      toast.error(
        apiErrorMessage(err, {
          fallback: "saveFailed",
          messageKey: "settings:members.roleUpdateFailed",
        }),
      );
    } finally {
      setSaving(false);
    }
  };

  const handleRemove = async () => {
    if (!removeTarget) return;
    setSaving(true);
    try {
      if (removeTarget.status === "pending") {
        await api.revokeWorkspaceInvitation(removeTarget.id);
        toast.success(t("members.inviteRevoked", { email: removeTarget.email }));
      } else {
        await api.removeWorkspaceMember(removeTarget.userId || removeTarget.id);
        toast.success(t("members.removed", { email: removeTarget.email }));
      }
      setRemoveTarget(null);
      refetch();
    } catch (err) {
      toast.error(
        apiErrorMessage(err, {
          fallback: "deleteFailed",
          messageKey: "settings:members.removeFailed",
        }),
      );
    } finally {
      setSaving(false);
    }
  };

  return (
    <div className="space-y-6">
      <Card>
        <CardHeader>
          <CardTitle className="text-h2 flex items-center gap-2">
            <Users size={20} />
            {t("members.title")}
          </CardTitle>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="flex flex-col gap-2 sm:flex-row sm:items-center">
            <Input
              type="email"
              autoComplete="email"
              placeholder={t("members.emailPlaceholder")}
              className="max-w-sm"
              value={email}
              onChange={(e) => setEmail(e.target.value)}
              onKeyDown={(e) => {
                if (e.key === "Enter") {
                  e.preventDefault();
                  void handleInvite();
                }
              }}
              aria-label={t("members.emailPlaceholder")}
            />
            <Select value={role} onValueChange={(value) => setRole(value as InviteRole)}>
              <SelectTrigger className="w-[140px]" aria-label={t("members.roleLabel")}>
                <SelectValue>{t(`members.roles.${role}`)}</SelectValue>
              </SelectTrigger>
              <SelectContent>
                {inviteRoles.map((inviteRole) => (
                  <SelectItem key={inviteRole} value={inviteRole}>
                    {t(`members.roles.${inviteRole}`)}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
            <Button onClick={() => void handleInvite()} disabled={!canInvite}>
              <EnvelopeSimple size={16} className="mr-1.5" />
              {inviting ? t("members.inviting") : t("members.invite")}
            </Button>
          </div>

          {error ? (
            <div className="rounded-lg border border-error-500/20 bg-error-100 p-4">
              <p className="text-sm font-medium text-error-500">{t("members.loadFailed")}</p>
              <p className="text-caption mt-1 text-error-500/80">{error}</p>
              <Button variant="outline" size="sm" className="mt-3" onClick={refetch}>
                {t("members.retry")}
              </Button>
            </div>
          ) : loading ? (
            <Skeleton className="h-40" />
          ) : (
            <ul className="divide-y divide-border">
              {(members ?? []).map((member) => {
                const displayName = member.name?.trim() || member.email;
                const showEmailSecondary = Boolean(member.name?.trim()) && member.name.trim() !== member.email;
                const isSelf = Boolean(selfEmail && member.email.toLowerCase() === selfEmail);
                const canManage = !isSelf && canManageTargetRole(actorRole, member.role);
                const disabledReason = member.role === "owner"
                  ? t("members.cannotModifyOwner")
                  : isSelf
                    ? t("members.cannotModifySelf")
                    : t("members.cannotManageMember");
                return (
                  <li key={member.id} className="flex items-center justify-between py-3">
                    <div className="flex items-center gap-3">
                      <div className="flex h-9 w-9 items-center justify-center rounded-full bg-muted text-xs font-medium">
                        {getInitials(displayName)}
                      </div>
                      <div>
                        <p className="text-sm font-medium">{displayName}</p>
                        {showEmailSecondary ? (
                          <p className="text-caption text-muted-foreground">{member.email}</p>
                        ) : null}
                      </div>
                    </div>
                    <div className="flex items-center gap-3">
                      {member.status === "pending" ? (
                        <Badge variant="secondary">{t("members.status.pending")}</Badge>
                      ) : null}
                      <Badge variant={member.status === "active" ? "default" : "secondary"}>
                        {t(`members.roles.${member.role}`, { defaultValue: member.role })}
                      </Badge>
                      <RowActions
                        actions={[
                          {
                            label: t("members.editRole"),
                            onClick: () => openEdit(member),
                            disabled: !canManage,
                            title: canManage ? undefined : disabledReason,
                          },
                          {
                            label: t("members.remove"),
                            onClick: () => setRemoveTarget(member),
                            destructive: true,
                            disabled: !canManage,
                            title: canManage ? undefined : disabledReason,
                          },
                        ]}
                      />
                    </div>
                  </li>
                );
              })}
            </ul>
          )}
        </CardContent>
      </Card>

      <Dialog open={Boolean(editTarget)} onOpenChange={(open) => !open && !saving && setEditTarget(null)}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{t("members.editRoleTitle")}</DialogTitle>
            <DialogDescription>
              {t("members.editRoleDescription", { email: editTarget?.email ?? "" })}
            </DialogDescription>
          </DialogHeader>
          <Select value={editRole} onValueChange={(value) => setEditRole(value as InviteRole)}>
            <SelectTrigger aria-label={t("members.roleLabel")}>
              <SelectValue>{t(`members.roles.${editRole}`)}</SelectValue>
            </SelectTrigger>
            <SelectContent>
              {inviteRoles.map((inviteRole) => (
                <SelectItem key={inviteRole} value={inviteRole}>
                  {t(`members.roles.${inviteRole}`)}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
          <DialogFooter>
            <Button variant="outline" onClick={() => setEditTarget(null)} disabled={saving}>
              {tc("cancel")}
            </Button>
            <Button
              onClick={() => void handleSaveRole()}
              disabled={saving || editTarget?.role === editRole || !canManageTargetRole(actorRole, editRole)}
            >
              {saving ? t("members.savingRole") : t("members.saveRole")}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <Dialog open={Boolean(removeTarget)} onOpenChange={(open) => !open && !saving && setRemoveTarget(null)}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{t("members.removeTitle")}</DialogTitle>
            <DialogDescription>
              {removeTarget?.status === "pending"
                ? t("members.removePendingDescription", { email: removeTarget.email })
                : t("members.removeDescription", { email: removeTarget?.email ?? "" })}
            </DialogDescription>
          </DialogHeader>
          <DialogFooter>
            <Button variant="outline" onClick={() => setRemoveTarget(null)} disabled={saving}>
              {tc("cancel")}
            </Button>
            <Button variant="destructive" onClick={() => void handleRemove()} disabled={saving}>
              {saving ? t("members.removing") : t("members.remove")}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  );
}
