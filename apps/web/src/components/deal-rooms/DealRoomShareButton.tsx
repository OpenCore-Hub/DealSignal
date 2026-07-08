import { useState } from "react";
import { ShareNetwork, Copy, Check } from "@phosphor-icons/react";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { useTranslation } from "react-i18next";
import { toast } from "sonner";

interface DealRoomShareButtonProps {
  slug?: string;
}

export function DealRoomShareButton({ slug }: DealRoomShareButtonProps) {
  const { t } = useTranslation("dealRooms");
  const [copied, setCopied] = useState(false);
  const publicUrl = slug ? `${window.location.origin}/r/${slug}` : "";

  const handleCopy = async () => {
    if (!publicUrl) return;
    try {
      await navigator.clipboard.writeText(publicUrl);
      setCopied(true);
      toast.success(t("share.copied"));
      setTimeout(() => setCopied(false), 2000);
    } catch {
      toast.error("Failed to copy link");
    }
  };

  return (
    <Dialog>
      <DialogTrigger
        render={(
          <Button variant="outline" className="gap-1.5">
            <ShareNetwork size={16} />
            {t("toolbar.share")}
          </Button>
        )}
      />
      <DialogContent className="sm:max-w-md">
        <DialogHeader>
          <DialogTitle className="flex items-center gap-2">
            <ShareNetwork size={20} />
            {t("share.title")}
          </DialogTitle>
          <DialogDescription>{t("share.publicLink")}</DialogDescription>
        </DialogHeader>
        <div className="flex items-center gap-2 py-2">
          <Input value={publicUrl} readOnly className="flex-1" />
          <Button onClick={handleCopy} disabled={!publicUrl || copied} className="gap-1.5">
            {copied ? <Check size={16} /> : <Copy size={16} />}
            {copied ? t("share.copied") : t("share.copyLink")}
          </Button>
        </div>
      </DialogContent>
    </Dialog>
  );
}
