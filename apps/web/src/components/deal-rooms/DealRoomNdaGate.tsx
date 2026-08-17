import { useState } from "react";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import { api } from "@/lib/api";
import { apiErrorMessage } from "@/lib/apiErrors";
import { useTranslation } from "react-i18next";
import { toast } from "sonner";

interface DealRoomNdaGateProps {
  roomId: string;
  roomName: string;
  onSigned: () => void;
}

export function DealRoomNdaGate({ roomId, roomName, onSigned }: DealRoomNdaGateProps) {
  const { t } = useTranslation("dealRooms");
  const [signing, setSigning] = useState(false);

  const handleSign = async () => {
    setSigning(true);
    try {
      await api.signDealRoomMemberNda(roomId);
      toast.success(t("ndaGate.signed"));
      onSigned();
    } catch (error) {
      toast.error(apiErrorMessage(error, { messageKey: "dealRooms:ndaGate.failed" }));
    } finally {
      setSigning(false);
    }
  };

  return (
    <Card data-testid="deal-room-nda-gate">
      <CardContent className="flex flex-col items-start gap-4 p-6">
        <div className="space-y-2">
          <h2 className="text-h3">{t("ndaGate.title")}</h2>
          <p className="text-sm text-muted-foreground">
            {t("ndaGate.description", { name: roomName })}
          </p>
        </div>
        <Button
          type="button"
          onClick={() => void handleSign()}
          disabled={signing}
          data-testid="deal-room-nda-sign"
        >
          {signing ? t("ndaGate.signing") : t("ndaGate.sign")}
        </Button>
      </CardContent>
    </Card>
  );
}
