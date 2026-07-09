import { useMemo, useState } from "react";
import { useTranslation } from "react-i18next";
import { toast } from "sonner";
import { ShareNetwork } from "@phosphor-icons/react";
import {
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import { Switch } from "@/components/ui/switch";
import { Tabs, TabsList, TabsTrigger, TabsContent } from "@/components/ui/tabs";
import { api } from "@/lib/api";
import type { AccessLog, AccessRule, Link, LinkInvitation } from "@/types";
import { useAsyncData } from "@/hooks/useAsyncData";
import { ConfirmDialog } from "@/components/common/ConfirmDialog";
import {
  ShareTab,
  InviteTab,
  AccessTab,
  AnalyticsTab,
  CopyButton,
  PRESETS,
  buildDraft,
  buildRules,
  buildLinkPayload,
  inferPreset,
  validateDraft,
  getPublicUrl,
} from "./";
import type { DraftLink } from "./types";

interface LinkShareDialogProps {
  linkId: string;
  defaultTab?: "share" | "invite" | "access" | "analytics";
  children?: React.ReactElement;
  onChanged?: () => void;
}

function isValidEmail(value: string): boolean {
  return /^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(value.trim());
}

function now(): number {
  return Date.now();
}

interface DialogData {
  link: Link;
  rules: AccessRule[];
  invitations: LinkInvitation[];
  logs: AccessLog[];
}

async function fetchDialogData(linkId: string): Promise<DialogData | null> {
  const [link, rulesRes, invitationsRes, logsRes] = await Promise.all([
    api.getLinkById(linkId),
    api.getLinkAccessRules(linkId),
    api.getLinkInvitations(linkId),
    api.getAccessLogs(linkId),
  ]);
  if (!link) return null;
  return {
    link,
    rules: rulesRes.data,
    invitations: invitationsRes.data,
    logs: logsRes.data,
  };
}

function LinkShareDialogContent({
  defaultTab = "share",
  data,
  loadingData,
  refetch,
  onChanged,
  onClose,
}: {
  defaultTab?: "share" | "invite" | "access" | "analytics";
  data: DialogData | null;
  loadingData: boolean;
  refetch: () => Promise<void>;
  onChanged?: () => void;
  onClose: () => void;
}) {
  const { t } = useTranslation("linkShare");
  const [tab, setTab] = useState<"share" | "invite" | "access" | "analytics">(defaultTab);
  const [draft, setDraft] = useState<DraftLink>(() => buildDraft(data?.link, data?.rules));
  const [errors, setErrors] = useState<Record<string, string>>({});
  const [saving, setSaving] = useState(false);

  const [inviteEmailsRaw, setInviteEmailsRaw] = useState("");
  const [inviteInvalid, setInviteInvalid] = useState<string[]>([]);
  const [inviteSending, setInviteSending] = useState(false);

  const link = data?.link ?? null;
  const invitations = data?.invitations ?? [];
  const logs = data?.logs ?? [];
  const preset = useMemo(() => inferPreset(draft), [draft]);

  const [confirmDialog, setConfirmDialog] = useState<{
    open: boolean;
    title: string;
    description: string;
    destructive?: boolean;
    onConfirm: () => void;
  }>({ open: false, title: "", description: "", onConfirm: () => {} });

  const updateDraft = (patch: Partial<DraftLink>) => {
    setDraft((prev) => ({ ...prev, ...patch }));
  };

  const saveLinkAndRules = async (): Promise<boolean> => {
    if (!link) return false;
    setSaving(true);
    try {
      await api.updateLinkFull(link.id, buildLinkPayload(draft, link));
      await api.setLinkAccessRules(link.id, buildRules(draft));
      toast.success(t("share.saveSuccess"));
      await refetch();
      onChanged?.();
      return true;
    } catch {
      toast.error(t("common:error.saveFailed"));
      return false;
    } finally {
      setSaving(false);
    }
  };

  const handleSave = async () => {
    if (!link) return;
    const validationErrors = validateDraft(draft, link, t, now());
    if (Object.keys(validationErrors).length > 0) {
      setErrors(validationErrors);
      return;
    }
    await saveLinkAndRules();
  };

  const handleActiveChange = async (checked: boolean) => {
    if (!link) return;
    if (!checked) {
      setConfirmDialog({
        open: true,
        title: t("share.disableConfirmTitle"),
        description: t("share.disableConfirmDescription"),
        onConfirm: async () => {
          setConfirmDialog((prev) => ({ ...prev, open: false }));
          try {
            await api.updateLink(link.id, { status: "revoked" });
            await refetch();
            onChanged?.();
          } catch {
            toast.error(t("common:error.saveFailed"));
          }
        },
      });
      return;
    }
    try {
      await api.updateLink(link.id, { status: "active" });
      await refetch();
      onChanged?.();
    } catch {
      toast.error(t("common:error.saveFailed"));
    }
  };

  const parseEmails = (raw: string): { valid: string[]; invalid: string[] } => {
    const parts = raw.split(/[,;\n\t]+/).map((s) => s.trim()).filter(Boolean);
    const valid: string[] = [];
    const invalid: string[] = [];
    for (const part of parts) {
      if (isValidEmail(part)) valid.push(part.toLowerCase());
      else invalid.push(part);
    }
    return { valid, invalid };
  };

  const handleInviteSend = async () => {
    if (!link) return;
    const { valid, invalid } = parseEmails(inviteEmailsRaw);
    setInviteInvalid(invalid);
    if (valid.length === 0) return;

    setInviteSending(true);
    try {
      await api.inviteLinkViewers(link.id, valid);
      setInviteEmailsRaw("");
      await refetch();
      onChanged?.();
    } finally {
      setInviteSending(false);
    }
  };

  const handleInviteResend = async (email: string) => {
    if (!link) return;
    setInviteSending(true);
    try {
      await api.inviteLinkViewers(link.id, [email]);
      await refetch();
    } finally {
      setInviteSending(false);
    }
  };

  const handleInviteRevoke = async (invitation: LinkInvitation) => {
    if (!link) return;
    setConfirmDialog({
      open: true,
      title: t("invite.revokeConfirmTitle", { email: invitation.email }),
      description: t("invite.revokeConfirmDescription"),
      destructive: true,
      onConfirm: async () => {
        setConfirmDialog((prev) => ({ ...prev, open: false }));
        try {
          await api.revokeLinkInvitation(link.id, invitation.id, true);
          await refetch();
          onChanged?.();
        } catch {
          // error toast handled by api client
        }
      },
    });
  };

  const publicUrl = getPublicUrl(link);

  const primaryAction =
    tab === "share"
      ? { label: t("share.saveLinkSettings"), onClick: handleSave }
      : tab === "access"
      ? { label: t("accessRules.saveAccessRules"), onClick: handleSave }
      : tab === "invite"
      ? { label: t("invite.sendInvitations"), onClick: handleInviteSend }
      : { label: t("analytics.done"), onClick: onClose };

  const inviteHasInput = inviteEmailsRaw.trim().length > 0;

  return (
    <>
      <DialogHeader>
        <div className="flex items-start justify-between gap-4">
          <div className="min-w-0 flex-1 space-y-1">
            <DialogTitle className="flex items-center gap-2">
              <ShareNetwork size={20} />
              {link?.name || t("share.title")}
            </DialogTitle>
            {publicUrl && (
              <div className="flex items-center gap-2">
                <span className="truncate text-xs text-muted-foreground">{publicUrl}</span>
                <CopyButton
                  value={publicUrl}
                  label={t("share.copyLink")}
                  successLabel={t("share.copied")}
                  variant="ghost"
                  size="sm"
                  className="h-auto px-1 py-0"
                />
              </div>
            )}
          </div>
          {link && (
            <div className="flex items-center gap-2">
              <span className={link.isActive ? "text-success-600" : "text-muted-foreground"}>
                {link.isActive ? t("share.active") : t("share.inactive")}
              </span>
              <Switch checked={link.isActive ?? false} onCheckedChange={handleActiveChange} />
            </div>
          )}
        </div>
      </DialogHeader>

      {draft.allowedViewers.length > 0 && (
        <div className="rounded-md border border-warning-500/20 bg-warning-500/10 px-3 py-2 text-xs text-warning-700">
          {t("share.restrictedAlert")}
        </div>
      )}

      <Tabs value={tab} onValueChange={(v) => setTab(v as typeof tab)} className="flex flex-1 flex-col overflow-hidden">
        <TabsList variant="line" className="w-full">
          <TabsTrigger value="share">{t("share.title")}</TabsTrigger>
          <TabsTrigger value="invite">{t("invite.title")}</TabsTrigger>
          <TabsTrigger value="access">{t("accessRules.title")}</TabsTrigger>
          <TabsTrigger value="analytics">{t("analytics.title")}</TabsTrigger>
        </TabsList>

        <div className="flex-1 overflow-y-auto py-2">
          {loadingData || !data ? (
            <div className="py-10 text-center text-sm text-muted-foreground">{t("common:loading")}</div>
          ) : (
            <>
              <TabsContent value="share">
                <ShareTab
                  draft={draft}
                  updateDraft={updateDraft}
                  preset={preset}
                  setPreset={(name) => {
                    if (name === "public") {
                      updateDraft({
                        ...PRESETS.public,
                        allowedViewers: [],
                        blockedViewers: [],
                        password: "",
                      });
                    } else if (name !== "custom") {
                      updateDraft({
                        ...PRESETS[name],
                        password: name === "confidential" ? "" : draft.password,
                      });
                    }
                  }}
                  link={link}
                  onEditAccess={() => setTab("access")}
                  errors={errors}
                />
              </TabsContent>
              <TabsContent value="invite">
                <InviteTab
                  linkId={link?.id}
                  emailsRaw={inviteEmailsRaw}
                  setEmailsRaw={setInviteEmailsRaw}
                  invalid={inviteInvalid}
                  sending={inviteSending}
                  invitations={invitations}
                  loading={loadingData}
                  onSend={handleInviteSend}
                  onResend={handleInviteResend}
                  onRevoke={handleInviteRevoke}
                />
              </TabsContent>
              <TabsContent value="access">
                <AccessTab draft={draft} updateDraft={updateDraft} errors={errors} />
              </TabsContent>
              <TabsContent value="analytics">
                {link && <AnalyticsTab link={link} logs={logs} />}
              </TabsContent>
            </>
          )}
        </div>
      </Tabs>

      <DialogFooter>
        <Button variant="outline" onClick={onClose}>
          {t("common:cancel")}
        </Button>
        <Button
          onClick={primaryAction.onClick}
          disabled={
            saving ||
            inviteSending ||
            (tab === "invite" && (!inviteHasInput || inviteSending))
          }
        >
          {saving || inviteSending ? t("common:saving") : primaryAction.label}
        </Button>
      </DialogFooter>

      <ConfirmDialog
        open={confirmDialog.open}
        title={confirmDialog.title}
        description={confirmDialog.description}
        confirmLabel={t("common:confirm")}
        cancelLabel={t("common:cancel")}
        destructive={confirmDialog.destructive}
        onConfirm={confirmDialog.onConfirm}
        onCancel={() => setConfirmDialog((prev) => ({ ...prev, open: false }))}
      />
    </>
  );
}

export function LinkShareDialog({
  linkId,
  defaultTab,
  children,
  onChanged,
}: LinkShareDialogProps) {
  const [open, setOpen] = useState(false);
  const { data, loading, refetch } = useAsyncData(
    () => (open ? fetchDialogData(linkId) : Promise.resolve(null)),
    [open, linkId]
  );

  return (
    <Dialog open={open} onOpenChange={setOpen}>
      {children && <DialogTrigger render={children} />}
      <DialogContent className="flex max-h-[90vh] flex-col sm:max-w-xl">
        {open && (
          <LinkShareDialogContent
            key={linkId}

            defaultTab={defaultTab}
            data={data}
            loadingData={loading}
            refetch={refetch}
            onChanged={onChanged}
            onClose={() => setOpen(false)}
          />
        )}
      </DialogContent>
    </Dialog>
  );
}
