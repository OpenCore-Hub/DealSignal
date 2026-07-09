import { ShareNetwork } from "@phosphor-icons/react";
import { Button } from "@/components/ui/button";
import { useTranslation } from "react-i18next";
import { DealRoomShareDialog } from "./DealRoomShareDialog";

interface DealRoomShareButtonProps {
  roomId: string;
  slug?: string;
}

export function DealRoomShareButton({ roomId, slug }: DealRoomShareButtonProps) {
  const { t } = useTranslation("dealRooms");

  return (
    <DealRoomShareDialog roomId={roomId} slug={slug}>
      <Button variant="outline" className="gap-1.5">
        <ShareNetwork size={16} />
        {t("toolbar.share")}
      </Button>
    </DealRoomShareDialog>
  );
}
