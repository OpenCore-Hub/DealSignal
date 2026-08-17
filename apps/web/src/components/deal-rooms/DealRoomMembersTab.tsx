import { Envelope } from "@phosphor-icons/react";
import { useTranslation } from "react-i18next";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { DealRoomMembersPanel } from "@/components/deal-rooms/DealRoomMembersPanel";
import { InviteMemberDialog } from "@/components/deal-rooms/InviteMemberDialog";
import { grantableRoomRoles } from "@/lib/dealRoomCapabilities";
import type { DealRoom } from "@/types";

interface DealRoomMembersTabProps {
  roomId: string;
  room: Pick<
    DealRoom,
    "ndaEnabled" | "ndaTemplateId" | "ndaDocumentId" | "roomRole"
  >;
  canManage?: boolean;
  onChanged?: () => void;
}

export function DealRoomMembersTab({
  roomId,
  room,
  canManage = false,
  onChanged,
}: DealRoomMembersTabProps) {
  const { t } = useTranslation("dealRooms");
  const canInvite = canManage && grantableRoomRoles(room.roomRole).length > 0;

  return (
    <div className="space-y-5" data-testid="deal-room-members-tab">
      <Card>
        <CardHeader className="pb-2">
          <div className="flex flex-wrap items-center justify-between gap-2">
            <div className="space-y-1">
              <CardTitle className="text-h3">{t("pageTabs.members")}</CardTitle>
              <p className="text-body text-muted-foreground">
                {canManage ? t("members.pageDescription") : t("members.oversightHint")}
              </p>
            </div>
            {canInvite ? (
              <InviteMemberDialog
                roomId={roomId}
                actorRoomRole={room.roomRole}
                ndaEnabled={room.ndaEnabled}
                ndaTemplateId={room.ndaTemplateId}
                ndaDocumentId={room.ndaDocumentId}
                onInvited={onChanged ?? (() => undefined)}
              >
                <Button variant="outline" className="gap-1.5 shrink-0">
                  <Envelope size={16} />
                  {t("members.inviteTitle")}
                </Button>
              </InviteMemberDialog>
            ) : null}
          </div>
        </CardHeader>
        <CardContent>
          <DealRoomMembersPanel
            roomId={roomId}
            canManage={canManage}
            actorRoomRole={room.roomRole}
            onChanged={onChanged}
            hideIntro
          />
        </CardContent>
      </Card>
    </div>
  );
}
