import { Gear, ShieldCheck, Users, Lock, Envelope } from "@phosphor-icons/react";
import { useTranslation } from "react-i18next";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { InviteMemberDialog } from "@/components/deal-rooms/InviteMemberDialog";
import { deriveRoomStage } from "@/lib/dealRoomNav";
import type { DealRoom } from "@/types";

interface DealRoomSettingsTabProps {
  roomId: string;
  room: Pick<DealRoom, "status" | "ndaEnabled" | "requiresApproval" | "memberCount">;
  activeLinkCount: number;
  onMemberInvited?: () => void;
}

export function DealRoomSettingsTab({
  roomId,
  room,
  activeLinkCount,
  onMemberInvited,
}: DealRoomSettingsTabProps) {
  const { t } = useTranslation("dealRooms");
  const stage = deriveRoomStage(activeLinkCount);

  const rows = [
    {
      key: "stage",
      icon: Gear,
      label: t("settings.fields.stage"),
      value: t(`settings.stage.${stage}`),
      hint: t("settings.stageHint"),
    },
    {
      key: "status",
      icon: Lock,
      label: t("settings.fields.status"),
      value: t(`settings.status.${room.status}`),
    },
    {
      key: "nda",
      icon: ShieldCheck,
      label: t("settings.fields.nda"),
      value: room.ndaEnabled ? t("settings.enabled") : t("settings.disabled"),
    },
    {
      key: "approval",
      icon: Users,
      label: t("settings.fields.requiresApproval"),
      value: room.requiresApproval ? t("settings.enabled") : t("settings.disabled"),
    },
    {
      key: "members",
      icon: Users,
      label: t("settings.fields.members"),
      value: String(room.memberCount),
    },
  ];

  return (
    <Card data-testid="deal-room-settings-tab">
      <CardHeader className="pb-2">
        <div className="flex flex-wrap items-center justify-between gap-2">
          <div className="space-y-1">
            <div className="flex flex-wrap items-center gap-2">
              <CardTitle className="text-h3">{t("settings.title")}</CardTitle>
              <Badge variant="secondary">{t(`settings.stage.${stage}`)}</Badge>
            </div>
            <p className="text-body text-muted-foreground">{t("settings.description")}</p>
          </div>
          <InviteMemberDialog roomId={roomId} onInvited={onMemberInvited ?? (() => undefined)}>
            <Button variant="outline" className="gap-1.5 shrink-0">
              <Envelope size={16} />
              {t("settings.inviteMembers")}
            </Button>
          </InviteMemberDialog>
        </div>
      </CardHeader>
      <CardContent className="space-y-4">
        <ul className="divide-y divide-border rounded-lg border border-border">
          {rows.map((row) => {
            const Icon = row.icon;
            return (
              <li key={row.key} className="flex items-start gap-3 px-3 py-3">
                <span className="mt-0.5 flex h-8 w-8 shrink-0 items-center justify-center rounded-md bg-muted text-muted-foreground">
                  <Icon size={16} />
                </span>
                <div className="min-w-0 flex-1">
                  <p className="text-caption text-muted-foreground">{row.label}</p>
                  <p className="text-sm font-medium">{row.value}</p>
                  {"hint" in row && row.hint ? (
                    <p className="mt-0.5 text-caption text-muted-foreground">{row.hint}</p>
                  ) : null}
                </div>
              </li>
            );
          })}
        </ul>
        <p className="text-caption text-muted-foreground">{t("settings.moreComing")}</p>
      </CardContent>
    </Card>
  );
}
