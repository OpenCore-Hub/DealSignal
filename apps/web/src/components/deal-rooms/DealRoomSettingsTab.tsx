import { useEffect, useState } from "react";
import { Gear, ShieldCheck, Users, Lock, Envelope } from "@phosphor-icons/react";
import { useTranslation } from "react-i18next";
import { toast } from "sonner";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { InviteMemberDialog } from "@/components/deal-rooms/InviteMemberDialog";
import {
  RoomNdaAgreementPicker,
  type RoomNdaSelection,
} from "@/components/deal-rooms/RoomNdaAgreementPicker";
import { useWorkspaceAccess } from "@/hooks/useWorkspaceAccess";
import { deriveRoomStage } from "@/lib/dealRoomNav";
import { api } from "@/lib/api";
import { apiErrorMessage } from "@/lib/apiErrors";
import type { DealRoom } from "@/types";

interface DealRoomSettingsTabProps {
  roomId: string;
  room: Pick<
    DealRoom,
    "status" | "ndaEnabled" | "ndaTemplateId" | "ndaDocumentId" | "requiresApproval" | "memberCount"
  >;
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
  const { canWrite } = useWorkspaceAccess();
  const stage = deriveRoomStage(activeLinkCount);
  const [ndaSelection, setNdaSelection] = useState<RoomNdaSelection>({
    ndaTemplateId: room.ndaTemplateId ?? "",
    ndaDocumentId: room.ndaDocumentId ?? "",
  });
  const [savingNda, setSavingNda] = useState(false);

  useEffect(() => {
    setNdaSelection({
      ndaTemplateId: room.ndaTemplateId ?? "",
      ndaDocumentId: room.ndaDocumentId ?? "",
    });
  }, [room.ndaTemplateId, room.ndaDocumentId]);

  const handleNdaChange = async (next: RoomNdaSelection) => {
    const prev = ndaSelection;
    setNdaSelection(next);
    if (!next.ndaTemplateId && !next.ndaDocumentId) return;
    setSavingNda(true);
    try {
      await api.patchDealRoomNdaAgreement(roomId, {
        nda_template_id: next.ndaTemplateId || undefined,
        nda_document_id: next.ndaDocumentId || undefined,
      });
      toast.success(t("settings.ndaSaved"));
      onMemberInvited?.();
    } catch (e) {
      setNdaSelection(prev);
      toast.error(apiErrorMessage(e, { fallback: "saveFailed" }));
    } finally {
      setSavingNda(false);
    }
  };

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
          {canWrite ? (
            <InviteMemberDialog
              roomId={roomId}
              ndaEnabled={room.ndaEnabled}
              ndaTemplateId={ndaSelection.ndaTemplateId || room.ndaTemplateId}
              ndaDocumentId={ndaSelection.ndaDocumentId || room.ndaDocumentId}
              onInvited={onMemberInvited ?? (() => undefined)}
            >
              <Button variant="outline" className="gap-1.5 shrink-0">
                <Envelope size={16} />
                {t("settings.inviteMembers")}
              </Button>
            </InviteMemberDialog>
          ) : null}
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
        {room.ndaEnabled ? (
          <div className="rounded-lg border border-border px-3 py-3">
            <RoomNdaAgreementPicker
              value={ndaSelection}
              onChange={(next) => {
                void handleNdaChange(next);
              }}
              disabled={!canWrite || savingNda}
              showError={canWrite && !ndaSelection.ndaTemplateId && !ndaSelection.ndaDocumentId}
            />
          </div>
        ) : null}
        <p className="text-caption text-muted-foreground">{t("settings.moreComing")}</p>
      </CardContent>
    </Card>
  );
}
