import { useState } from "react";
import { useTranslation } from "react-i18next";
import { PaperPlaneTilt, Envelope } from "@phosphor-icons/react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Checkbox } from "@/components/ui/checkbox";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from "@/components/ui/dialog";
import { api } from "@/lib/api";
import type { Contact } from "@/types";

interface MarketingBatchDialogProps {
  contacts: Contact[];
}

export function MarketingBatchDialog({ contacts }: MarketingBatchDialogProps) {
  const { t } = useTranslation("contacts");
  const [open, setOpen] = useState(false);
  const [subject, setSubject] = useState("");
  const [body, setBody] = useState("");
  const [trackOpens, setTrackOpens] = useState(true);
  const [trackClicks, setTrackClicks] = useState(true);
  const [loading, setLoading] = useState(false);
  const [result, setResult] = useState<{ sent: number; failed: number } | null>(null);
  const [error, setError] = useState<string | null>(null);

  const recipients = contacts.map((c) => c.email);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!subject.trim() || recipients.length === 0) return;
    setLoading(true);
    setError(null);
    setResult(null);
    try {
      const res = await api.sendMarketingBatch({
        recipients,
        subject: subject.trim(),
        body: body.trim(),
        track_opens: trackOpens,
        track_clicks: trackClicks,
      });
      setResult({ sent: res.data.sent, failed: res.data.failed });
    } catch (err) {
      setError(err instanceof Error ? err.message : t("marketingBatch.error"));
    } finally {
      setLoading(false);
    }
  };

  return (
    <Dialog open={open} onOpenChange={setOpen}>
      <DialogTrigger>
        <Button variant="outline" size="sm" disabled={contacts.length === 0}>
          <Envelope className="mr-2 size-4" />
          {t("marketingBatch.trigger")}
        </Button>
      </DialogTrigger>
      <DialogContent className="sm:max-w-lg">
        <form onSubmit={handleSubmit}>
          <DialogHeader>
            <DialogTitle>{t("marketingBatch.title")}</DialogTitle>
            <DialogDescription>{t("marketingBatch.description", { count: recipients.length })}</DialogDescription>
          </DialogHeader>
          <div className="grid gap-4 py-4">
            <div className="space-y-2">
              <label htmlFor="subject" className="text-sm font-medium">
                {t("marketingBatch.subject")}
              </label>
              <Input
                id="subject"
                value={subject}
                onChange={(e) => setSubject(e.target.value)}
                placeholder={t("marketingBatch.subjectPlaceholder")}
                required
              />
            </div>
            <div className="space-y-2">
              <label htmlFor="body" className="text-sm font-medium">
                {t("marketingBatch.body")}
              </label>
              <textarea
                id="body"
                value={body}
                onChange={(e) => setBody(e.target.value)}
                placeholder={t("marketingBatch.bodyPlaceholder")}
                rows={6}
                className="w-full rounded-lg border border-input bg-transparent px-3 py-2 text-sm outline-none placeholder:text-muted-foreground focus-visible:border-ring focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 focus-visible:ring-offset-background"
              />
            </div>
            <div className="flex flex-col gap-2">
              <label className="flex items-center gap-2 text-sm">
                <Checkbox checked={trackOpens} onCheckedChange={(v) => setTrackOpens(v === true)} />
                {t("marketingBatch.trackOpens")}
              </label>
              <label className="flex items-center gap-2 text-sm">
                <Checkbox checked={trackClicks} onCheckedChange={(v) => setTrackClicks(v === true)} />
                {t("marketingBatch.trackClicks")}
              </label>
            </div>
            {result && (
              <p className="text-sm text-muted-foreground">
                {t("marketingBatch.result", { sent: result.sent, failed: result.failed })}
              </p>
            )}
            {error && <p className="text-sm text-destructive">{error}</p>}
          </div>
          <DialogFooter>
            <Button type="submit" disabled={loading || !subject.trim()}>
              <PaperPlaneTilt className="mr-2 size-4" />
              {loading ? t("marketingBatch.sending") : t("marketingBatch.send")}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}
