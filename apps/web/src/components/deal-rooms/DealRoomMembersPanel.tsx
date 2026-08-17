import { useState } from "react";
import { useTranslation } from "react-i18next";
import { toast } from "sonner";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { Skeleton } from "@/components/ui/skeleton";
import { RowActions } from "@/components/common/RowActions";
import { useAsyncData } from "@/hooks/useAsyncData";
import { api } from "@/lib/api";
import { apiErrorMessage } from "@/lib/apiErrors";
import { getCachedAccountEmail } from "@/lib/authAccount";
import { canManageRoomMember, grantableRoomRoles } from "@/lib/dealRoomCapabilities";
import { getInitials } from "@/lib/formatters";
import type { DealRoomMember, DealRoomMemberRole } from "@/types";

interface DealRoomMembersPanelProps {
  roomId: string;
  canManage?: boolean;
  actorRoomRole?: DealRoomMemberRole | "";
  onChanged?: () => void;
  /** Hide the in-panel title when the parent tab already provides page chrome. */
  hideIntro?: boolean;
}

export function DealRoomMembersPanel({
  roomId,
  canManage = false,
  actorRoomRole,
  onChanged,
  hideIntro = false,
}: DealRoomMembersPanelProps) {
  const { t } = useTranslation("dealRooms");
  const { t: tc } = useTranslation("common");
  const { data: members, loading, error, refetch } = useAsyncData(
    () => api.getDealRoomMembers(roomId).then((res) => res.data ?? []),
    [roomId],
  );
  const [busyId, setBusyId] = useState<string | null>(null);
  const [removeTarget, setRemoveTarget] = useState<DealRoomMember | null>(null);
  const effectiveActorRole = actorRoomRole || undefined;
  const grantable = grantableRoomRoles(effectiveActorRole);
  const selfEmail = getCachedAccountEmail()?.toLowerCase();

  const refresh = async () => {
    await refetch();
    onChanged?.();
  };

  const handleRoleChange = async (member: DealRoomMember, role: DealRoomMemberRole) => {
    if (role === member.role) return;
    setBusyId(member.id);
    try {
      await api.updateDealRoomMemberRole(roomId, member.id, { role });
      toast.success(t("members.roleUpdated", { email: member.email }));
      await refresh();
    } catch (e) {
      toast.error(apiErrorMessage(e, { fallback: "saveFailed" }));
    } finally {
      setBusyId(null);
    }
  };

  const handleRemove = async () => {
    if (!removeTarget) return;
    setBusyId(removeTarget.id);
    try {
      await api.removeDealRoomMember(roomId, removeTarget.id);
      toast.success(t("members.removed", { email: removeTarget.email }));
      setRemoveTarget(null);
      await refresh();
    } catch (e) {
      toast.error(apiErrorMessage(e, { fallback: "saveFailed" }));
    } finally {
      setBusyId(null);
    }
  };

  return (
    <div data-testid="deal-room-members-panel" className="space-y-2">
      {hideIntro ? null : (
        <div>
          <p className="text-sm font-medium">{t("members.listTitle")}</p>
          <p className="text-caption text-muted-foreground">
            {canManage ? t("members.listDescription") : t("members.oversightHint")}
          </p>
        </div>
      )}
      {error ? (
        <div className="rounded-lg border border-error-500/20 bg-error-100 p-3">
          <p className="text-sm text-error-500">{t("members.loadFailed")}</p>
          <Button variant="outline" size="sm" className="mt-2" onClick={() => void refetch()}>
            {t("members.retry")}
          </Button>
        </div>
      ) : loading ? (
        <Skeleton className="h-24" />
      ) : !members?.length ? (
        <p className="text-caption text-muted-foreground">{t("members.empty")}</p>
      ) : (
        <ul className="divide-y divide-border rounded-lg border border-border">
          {members.map((member) => {
            const displayName = member.name?.trim() || member.email;
            const isSelf = Boolean(selfEmail && member.email.toLowerCase() === selfEmail);
            const manageable =
              canManage &&
              !isSelf &&
              canManageRoomMember(effectiveActorRole, member.role);
            const disabledReason =
              member.role === "owner"
                ? t("members.cannotModifyOwner")
                : isSelf
                  ? t("members.cannotModifySelf")
                  : t("members.cannotManageMember");
            return (
              <li
                key={member.id}
                className="flex items-center justify-between gap-3 px-3 py-2.5"
                data-testid={`deal-room-member-${member.id}`}
              >
                <div className="flex min-w-0 items-center gap-2.5">
                  <div className="flex h-8 w-8 shrink-0 items-center justify-center rounded-full bg-muted text-xs font-medium">
                    {getInitials(displayName)}
                  </div>
                  <div className="min-w-0">
                    <p className="truncate text-sm font-medium">{displayName}</p>
                    {member.name?.trim() ? (
                      <p className="truncate text-caption text-muted-foreground">{member.email}</p>
                    ) : null}
                  </div>
                </div>
                <div className="flex shrink-0 items-center gap-2">
                  {member.status === "pending" ? (
                    <Badge variant="secondary">{t("members.status.pending")}</Badge>
                  ) : null}
                  {member.nda_status === "signed" ? (
                    <Badge variant="secondary">{t("members.ndaSigned")}</Badge>
                  ) : null}
                  {manageable ? (
                    <Select
                      value={member.role}
                      onValueChange={(value) => {
                        void handleRoleChange(member, value as DealRoomMemberRole);
                      }}
                      disabled={busyId === member.id}
                    >
                      <SelectTrigger
                        className="w-[120px]"
                        aria-label={t("members.role")}
                        data-testid={`deal-room-member-role-${member.id}`}
                      >
                        <SelectValue>{t(`members.roles.${member.role}`)}</SelectValue>
                      </SelectTrigger>
                      <SelectContent>
                        {grantable.map((role) => (
                          <SelectItem key={role} value={role} label={t(`members.roles.${role}`)}>
                            {t(`members.roles.${role}`)}
                          </SelectItem>
                        ))}
                      </SelectContent>
                    </Select>
                  ) : (
                    <Badge variant={member.role === "owner" ? "default" : "secondary"}>
                      {t(`members.roles.${member.role}`, { defaultValue: member.role })}
                    </Badge>
                  )}
                  {canManage ? (
                    <RowActions
                      actions={[
                        {
                          label: t("members.remove", { email: member.email }),
                          onClick: () => setRemoveTarget(member),
                          destructive: true,
                          disabled: !manageable || busyId === member.id,
                          title: manageable ? undefined : disabledReason,
                        },
                      ]}
                    />
                  ) : null}
                </div>
              </li>
            );
          })}
        </ul>
      )}

      <Dialog
        open={Boolean(removeTarget)}
        onOpenChange={(open) => !open && !busyId && setRemoveTarget(null)}
      >
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{t("members.remove", { email: removeTarget?.email ?? "" })}</DialogTitle>
            <DialogDescription>
              {t("members.removeConfirm", { email: removeTarget?.email ?? "" })}
            </DialogDescription>
          </DialogHeader>
          <DialogFooter>
            <Button
              variant="outline"
              onClick={() => setRemoveTarget(null)}
              disabled={Boolean(busyId)}
            >
              {tc("cancel")}
            </Button>
            <Button
              variant="destructive"
              onClick={() => void handleRemove()}
              disabled={Boolean(busyId)}
            >
              {busyId ? t("members.removing") : t("members.remove", { email: removeTarget?.email ?? "" })}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  );
}
