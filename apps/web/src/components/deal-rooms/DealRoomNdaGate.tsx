import { useEffect, useState } from "react";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import { Checkbox } from "@/components/ui/checkbox";
import { Label } from "@/components/ui/label";
import { api, type DealRoomMemberNdaPreview } from "@/lib/api";
import { ApiError } from "@/lib/apiClient";
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
  const [agreed, setAgreed] = useState(false);
  const [preview, setPreview] = useState<DealRoomMemberNdaPreview | null>(null);
  const [previewStatus, setPreviewStatus] = useState<"loading" | "ready" | "error">("loading");
  const [previewEpoch, setPreviewEpoch] = useState(0);

  useEffect(() => {
    let cancelled = false;
    setPreviewStatus("loading");
    setAgreed(false);
    setPreview(null);
    void api
      .getDealRoomMemberNdaPreview(roomId)
      .then((data) => {
        if (cancelled) return;
        setPreview(data);
        setPreviewStatus("ready");
      })
      .catch(() => {
        if (cancelled) return;
        setPreview(null);
        setPreviewStatus("error");
      });
    return () => {
      cancelled = true;
    };
  }, [roomId, previewEpoch]);

  const pageUrls = preview?.previewPageUrls?.filter(Boolean) ?? [];
  const documentUrl = preview?.documentUrl?.trim() ?? "";
  const canReadAgreement = pageUrls.length > 0 || documentUrl !== "";
  const canSign = previewStatus === "ready" && canReadAgreement && agreed && !signing;

  const handleSign = async () => {
    if (!canSign || !preview) return;
    setSigning(true);
    try {
      await api.signDealRoomMemberNda(roomId, {
        agreed: true,
        content_sha256: preview.ndaTemplate.contentSha256,
      });
      toast.success(t("ndaGate.signed"));
      onSigned();
    } catch (error) {
      toast.error(apiErrorMessage(error, { messageKey: "dealRooms:ndaGate.failed" }));
      if (error instanceof ApiError && error.code === "nda_content_mismatch") {
        setPreviewEpoch((epoch) => epoch + 1);
      }
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

        <div
          className="max-h-[min(60vh,32rem)] w-full overflow-y-auto rounded-xl border border-border/60 bg-muted/10"
          data-testid="deal-room-nda-preview"
        >
          {previewStatus === "loading" ? (
            <p className="px-4 py-10 text-center text-sm text-muted-foreground">
              {t("ndaGate.previewLoading")}
            </p>
          ) : previewStatus === "error" || !canReadAgreement ? (
            <p className="px-4 py-10 text-center text-sm text-muted-foreground" role="alert">
              {t("ndaGate.previewUnavailable")}
            </p>
          ) : pageUrls.length > 0 ? (
            pageUrls.map((url, index) => (
              <img
                key={`${url}-${index}`}
                src={url}
                alt={t("ndaGate.previewPage", { page: index + 1 })}
                className="block h-auto w-full border-b border-border/40 last:border-b-0"
              />
            ))
          ) : (
            <iframe
              title={t("ndaGate.previewTitle")}
              src={documentUrl}
              className="h-[min(50vh,28rem)] w-full border-0"
            />
          )}
        </div>

        {previewStatus === "ready" && canReadAgreement ? (
          <>
            {preview?.signerEmail ? (
              <p className="text-xs text-muted-foreground">
                {t("ndaGate.signingAs", { email: preview.signerEmail })}
              </p>
            ) : null}
            <div className="flex w-full items-start gap-3 rounded-2xl border border-border/60 bg-background/60 px-4 py-3">
              <Checkbox
                id="deal-room-nda-agree"
                checked={agreed}
                onCheckedChange={(checked) => setAgreed(checked === true)}
                className="mt-0.5"
                data-testid="deal-room-nda-agree"
              />
              <Label htmlFor="deal-room-nda-agree" className="text-sm leading-relaxed font-normal">
                {t("ndaGate.agree")}
              </Label>
            </div>
            <Button
              type="button"
              onClick={() => void handleSign()}
              disabled={!canSign}
              data-testid="deal-room-nda-sign"
            >
              {signing ? t("ndaGate.signing") : t("ndaGate.sign")}
            </Button>
          </>
        ) : null}
      </CardContent>
    </Card>
  );
}
