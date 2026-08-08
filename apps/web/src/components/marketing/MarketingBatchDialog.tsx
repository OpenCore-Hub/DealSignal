import { useMemo, useState } from "react";
import { apiErrorMessage } from "@/lib/apiErrors";
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
import { ApiError } from "@/lib/apiClient";
import type { Contact } from "@/types";

interface MarketingBatchDialogProps {
  /** Route workspace slug — binds the send API path (never rely on stale window.location). */
  workspaceSlug: string;
  /** Recipients for this send — must be an explicit selection (never "all contacts" implied). */
  contacts: Contact[];
  /** Called after a successful API send (even if some recipients failed delivery). */
  onSent?: () => void;
}

function uniqueRecipientEmails(contacts: Contact[]): string[] {
  const seen = new Set<string>();
  const out: string[] = [];
  for (const c of contacts) {
    const email = c.email?.trim().toLowerCase();
    if (!email || seen.has(email)) continue;
    seen.add(email);
    out.push(email);
  }
  return out;
}

export function MarketingBatchDialog({
  workspaceSlug,
  contacts,
  onSent,
}: MarketingBatchDialogProps) {
  const { t } = useTranslation("contacts");
  const [open, setOpen] = useState(false);
  const [subject, setSubject] = useState("");
  const [body, setBody] = useState("");
  const [trackOpens, setTrackOpens] = useState(true);
  const [trackClicks, setTrackClicks] = useState(true);
  const [loading, setLoading] = useState(false);
  const [result, setResult] = useState<{
    sent: number;
    failed: number;
    failedRecipients: { email: string; message: string }[];
  } | null>(null);
  const [error, setError] = useState<string | null>(null);

  const recipients = useMemo(() => uniqueRecipientEmails(contacts), [contacts]);
  const canOpen = recipients.length > 0;

  const resetForm = () => {
    setSubject("");
    setBody("");
    setTrackOpens(true);
    setTrackClicks(true);
    setLoading(false);
    setResult(null);
    setError(null);
  };

  const handleOpenChange = (next: boolean) => {
    setOpen(next);
    if (next) {
      resetForm();
    }
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!subject.trim() || recipients.length === 0 || loading) return;
    setLoading(true);
    setError(null);
    setResult(null);
    try {
      const res = await api.sendMarketingBatch(
        {
          recipients,
          subject: subject.trim(),
          body: body.trim(),
          track_opens: trackOpens,
          track_clicks: trackClicks,
        },
        workspaceSlug,
      );
      setResult({
        sent: res.data.sent,
        failed: res.data.failed,
        failedRecipients: res.data.failed_recipients ?? [],
      });
      onSent?.();
    } catch (err) {
      if (err instanceof ApiError && err.code === "recipients_not_in_workspace") {
        setError(t("marketingBatch.recipientsNotInWorkspace"));
      } else if (err instanceof ApiError && err.code === "too_many_recipients") {
        setError(t("marketingBatch.tooManyRecipients"));
      } else {
        setError(apiErrorMessage(err, { messageKey: "contacts:marketingBatch.error" }));
      }
    } finally {
      setLoading(false);
    }
  };

  return (
    <Dialog open={open} onOpenChange={handleOpenChange}>
      <DialogTrigger
        render={
          <Button
            variant="outline"
            size="sm"
            disabled={!canOpen}
            title={!canOpen ? t("marketingBatch.noneSelected") : undefined}
            data-testid="contacts-bulk-email"
          >
            <Envelope className="mr-2 size-4" />
            {canOpen
              ? t("marketingBatch.triggerWithCount", { count: recipients.length })
              : t("marketingBatch.trigger")}
          </Button>
        }
      />
      <DialogContent className="sm:max-w-lg">
        <form onSubmit={handleSubmit}>
          <DialogHeader>
            <DialogTitle>{t("marketingBatch.title")}</DialogTitle>
            <DialogDescription>
              {t("marketingBatch.description", { count: recipients.length })}
            </DialogDescription>
          </DialogHeader>
          <div className="grid gap-4 py-4">
            <div className="space-y-2">
              <label htmlFor="marketing-batch-subject" className="text-sm font-medium">
                {t("marketingBatch.subject")}
              </label>
              <Input
                id="marketing-batch-subject"
                value={subject}
                onChange={(e) => setSubject(e.target.value)}
                placeholder={t("marketingBatch.subjectPlaceholder")}
                required
                disabled={loading}
              />
            </div>
            <div className="space-y-2">
              <label htmlFor="marketing-batch-body" className="text-sm font-medium">
                {t("marketingBatch.body")}
              </label>
              <textarea
                id="marketing-batch-body"
                value={body}
                onChange={(e) => setBody(e.target.value)}
                placeholder={t("marketingBatch.bodyPlaceholder")}
                rows={6}
                disabled={loading}
                className="focus-ring-field w-full rounded-lg border border-input bg-transparent px-3 py-2 text-sm outline-none placeholder:text-muted-foreground disabled:opacity-50"
              />
            </div>
            <div className="flex flex-col gap-2">
              <label className="flex items-center gap-2 text-sm">
                <Checkbox
                  checked={trackOpens}
                  onCheckedChange={(v) => setTrackOpens(v === true)}
                  disabled={loading}
                />
                {t("marketingBatch.trackOpens")}
              </label>
              <label className="flex items-center gap-2 text-sm">
                <Checkbox
                  checked={trackClicks}
                  onCheckedChange={(v) => setTrackClicks(v === true)}
                  disabled={loading}
                />
                {t("marketingBatch.trackClicks")}
              </label>
            </div>
            {result && (
              <div className="space-y-1 text-sm text-muted-foreground">
                <p>{t("marketingBatch.result", { sent: result.sent, failed: result.failed })}</p>
                {result.failedRecipients.length > 0 ? (
                  <p className="text-destructive">
                    {t("marketingBatch.failedRecipients", {
                      emails: result.failedRecipients.map((f) => f.email).join(", "),
                    })}
                  </p>
                ) : null}
              </div>
            )}
            {error && <p className="text-sm text-destructive">{error}</p>}
          </div>
          <DialogFooter>
            <Button type="submit" disabled={loading || !subject.trim() || recipients.length === 0}>
              <PaperPlaneTilt className="mr-2 size-4" />
              {loading ? t("marketingBatch.sending") : t("marketingBatch.send")}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}
