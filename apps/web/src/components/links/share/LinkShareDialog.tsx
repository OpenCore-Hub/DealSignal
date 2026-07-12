import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import { toast } from "sonner";
import { motion, AnimatePresence } from "motion/react";
import { ShareNetwork, Check } from "@phosphor-icons/react";
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
  applyPreset,
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

function now(): number {
  return Date.now();
}

const tabTransition = {
  initial: { opacity: 0, x: 8 },
  animate: { opacity: 1, x: 0 },
  exit: { opacity: 0, x: -8 },
  transition: { duration: 0.2, ease: [0.16, 1, 0.3, 1] as const },
};

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
  registerCloseGuard,
}: {
  defaultTab?: "share" | "invite" | "access" | "analytics";
  data: DialogData | null;
  loadingData: boolean;
  refetch: () => Promise<void>;
  onChanged?: () => void;
  onClose: () => void;
  registerCloseGuard: (guard: () => boolean) => void;
}) {
  const { t } = useTranslation("linkShare");
  const [tab, setTab] = useState<"share" | "invite" | "access" | "analytics">(defaultTab);
  const [draft, setDraft] = useState<DraftLink>(() => buildDraft(data?.link, data?.rules));
  const [errors, setErrors] = useState<Record<string, string>>({});
  const [saving, setSaving] = useState(false);
  const [saveSuccess, setSaveSuccess] = useState(false);
  const [highlightedFields, setHighlightedFields] = useState<string[]>([]);

  const [inviteEmails, setInviteEmails] = useState<string[]>([]);
  const [inviteSending, setInviteSending] = useState(false);

  // Unsaved-changes tracking.
  const initialDraftRef = useRef<DraftLink>(draft);
  const [closeConfirmOpen, setCloseConfirmOpen] = useState(false);
  const hasUnsavedChanges = useCallback(() => {
    return (
      JSON.stringify(draft) !== JSON.stringify(initialDraftRef.current) ||
      inviteEmails.length > 0
    );
  }, [draft, inviteEmails]);
  const markClean = useCallback(() => {
    initialDraftRef.current = { ...draft };
    setInviteEmails([]);
  }, [draft]);

  const handleConditionalClose = useCallback(() => {
    if (hasUnsavedChanges()) {
      setCloseConfirmOpen(true);
      return true;
    }
    onClose();
    return false;
  }, [hasUnsavedChanges, onClose]);
  useEffect(() => {
    registerCloseGuard(handleConditionalClose);
  }, [registerCloseGuard, handleConditionalClose]);

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
    if (Object.keys(errors).length > 0) setErrors({});
  };

  const saveLinkAndRules = async (): Promise<boolean> => {
    if (!link) return false;
    setSaving(true);
    try {
      await api.updateLinkFull(link.id, buildLinkPayload(draft, link));
      await api.setLinkAccessRules(link.id, buildRules(draft));
      toast.success(t("share.saveSuccess"));
      markClean();
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
    const ok = await saveLinkAndRules();
    if (ok) {
      setSaveSuccess(true);
      setTimeout(() => onClose(), 1500);
    }
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

  const handleInviteSend = async () => {
    if (!link || inviteEmails.length === 0) return;

    setInviteSending(true);
    try {
      await api.inviteLinkViewers(link.id, inviteEmails);
      setInviteEmails([]);
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
      toast.success(t("invite.resendSuccess", { email }));
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
      ? { label: saveSuccess ? t("share.savedButtonLabel") : t("share.saveLinkSettings"), onClick: handleSave }
      : tab === "access"
      ? { label: saveSuccess ? t("accessRules.saved") : t("accessRules.saveAccessRules"), onClick: handleSave }
      : tab === "invite"
      ? { label: t("invite.sendInvitations"), onClick: handleInviteSend }
      : { label: t("analytics.done"), onClick: onClose };

  const inviteHasInput = inviteEmails.length > 0;

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
            <AnimatePresence mode="wait" initial={false}>
              <motion.div key={tab} {...tabTransition}>
                <TabsContent value="share">
                  <ShareTab
                    draft={draft}
                    updateDraft={updateDraft}
                    preset={preset}
                    setPreset={(name) => {
                      if (name === "custom") return;
                      const { patch, changedFields } = applyPreset(name, draft);
                      updateDraft(patch);
                      setHighlightedFields(changedFields);
                      const timer = setTimeout(() => setHighlightedFields([]), 200);
                      return () => clearTimeout(timer);
                    }}
                    link={link}
                    onEditAccess={() => setTab("access")}
                    errors={errors}
                    highlightedFields={highlightedFields}
                  />
                </TabsContent>
                <TabsContent value="invite">
                  <InviteTab
                    linkId={link?.id}
                    emails={inviteEmails}
                    setEmails={setInviteEmails}
                    sending={inviteSending}
                    invitations={invitations}
                    loading={loadingData}
                    onSend={handleInviteSend}
                    onResend={handleInviteResend}
                    onRevoke={handleInviteRevoke}
                  />
                </TabsContent>
                <TabsContent value="access">
                  <AccessTab draft={draft} updateDraft={updateDraft} errors={errors} highlightedFields={highlightedFields} />
                </TabsContent>
                <TabsContent value="analytics">
                  {link && <AnalyticsTab link={link} logs={logs} />}
                </TabsContent>
              </motion.div>
            </AnimatePresence>
          )}
        </div>
      </Tabs>

      <DialogFooter>
        <Button variant="outline" onClick={handleConditionalClose}>
          {t("common:cancel")}
        </Button>
        <Button
          onClick={primaryAction.onClick}
          disabled={
            saving ||
            inviteSending ||
            (tab === "invite" && (!inviteHasInput || inviteSending)) ||
            ((tab === "share" || tab === "access") && Object.keys(errors).length > 0)
          }
        >
          {saving || inviteSending ? (
            t("common:saving")
          ) : saveSuccess ? (
            <span className="flex items-center gap-1.5">
              <Check size={16} />
              {primaryAction.label}
            </span>
          ) : (
            primaryAction.label
          )}
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

      <ConfirmDialog
        open={closeConfirmOpen}
        title={t("common:unsavedChangesTitle")}
        description={t("common:unsavedChangesDescription")}
        confirmLabel={t("common:unsavedChangesConfirm")}
        cancelLabel={t("common:cancel")}
        destructive
        onConfirm={() => {
          setCloseConfirmOpen(false);
          markClean();
          onClose();
        }}
        onCancel={() => setCloseConfirmOpen(false)}
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

  const closeGuardRef = useRef<(() => boolean) | null>(null);
  const registerCloseGuard = useCallback((guard: () => boolean) => {
    closeGuardRef.current = guard;
  }, []);

  const handleOpenChange = useCallback((isOpen: boolean) => {
    if (!isOpen && closeGuardRef.current?.()) {
      return;
    }
    setOpen(isOpen);
  }, []);

  return (
    <Dialog open={open} onOpenChange={handleOpenChange}>
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
            registerCloseGuard={registerCloseGuard}
          />
        )}
      </DialogContent>
    </Dialog>
  );
}
