import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import { toast } from "sonner";
import { useReducedMotion } from "motion/react";
import { motion, AnimatePresence } from "motion/react";
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
} from "@/components/links/share";
import type { DraftLink } from "@/components/links/share";

const tabTransition = {
  initial: { opacity: 0, x: 8 },
  animate: { opacity: 1, x: 0 },
  exit: { opacity: 0, x: -8 },
  transition: { duration: 0.2, ease: [0.16, 1, 0.3, 1] as const },
};

interface DealRoomShareDialogProps {
  roomId: string;
  linkId?: string;
  slug?: string;
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
  links: Link[];
  selectedLink: Link | null;
  rules: AccessRule[];
  invitations: LinkInvitation[];
  logs: AccessLog[];
}

async function fetchDialogData(roomId: string, linkId?: string): Promise<DialogData> {
  const linksRes = await api.getDealRoomLinks(roomId);
  const loadedLinks = linksRes.data;
  const selectedLink = linkId
    ? loadedLinks.find((l) => l.id === linkId) || null
    : loadedLinks.find((l) => l.status === "active" || l.isActive) || null;

  if (!selectedLink) {
    return { links: loadedLinks, selectedLink: null, rules: [], invitations: [], logs: [] };
  }

  const [rulesRes, invitationsRes, logsRes] = await Promise.all([
    api.getLinkAccessRules(selectedLink.id),
    api.getLinkInvitations(selectedLink.id),
    api.getAccessLogs(selectedLink.id),
  ]);

  return {
    links: loadedLinks,
    selectedLink,
    rules: rulesRes.data,
    invitations: invitationsRes.data,
    logs: logsRes.data,
  };
}

interface DealRoomShareDialogContentProps {
  roomId: string;
  slug?: string;
  defaultTab?: "share" | "invite" | "access" | "analytics";
  data: DialogData | null;
  loadingData: boolean;
  refetch: () => Promise<void>;
  onChanged?: () => void;
  onClose: () => void;
  registerCloseGuard: (guard: () => boolean) => void;
}

function DealRoomShareDialogContent({
  roomId,
  slug,
  defaultTab = "share",
  data,
  loadingData,
  refetch,
  onChanged,
  onClose,
  registerCloseGuard,
}: DealRoomShareDialogContentProps) {
  const { t } = useTranslation("dealRooms");
  const { t: lt } = useTranslation("linkShare");
  const reducedMotion = useReducedMotion();
  const [tab, setTab] = useState<"share" | "invite" | "access" | "analytics">(defaultTab);
  const [draft, setDraft] = useState<DraftLink>(() => buildDraft(data?.selectedLink, data?.rules));
  const [errors, setErrors] = useState<Record<string, string>>({});
  const [saving, setSaving] = useState(false);

  const [inviteEmailsRaw, setInviteEmailsRaw] = useState("");
  const [inviteInvalid, setInviteInvalid] = useState<string[]>([]);
  const [inviteSending, setInviteSending] = useState(false);

  // Unsaved-changes tracking.
  const initialDraftRef = useRef<DraftLink>(draft);
  const [closeConfirmOpen, setCloseConfirmOpen] = useState(false);
  const hasUnsavedChanges = useCallback(() => {
    return JSON.stringify(draft) !== JSON.stringify(initialDraftRef.current);
  }, [draft]);

  // Resync baseline after save.
  const markClean = useCallback(() => {
    initialDraftRef.current = { ...draft };
  }, [draft]);

  const selectedLink = data?.selectedLink ?? null;
  const invitations = data?.invitations ?? [];
  const logs = data?.logs ?? [];
  const preset = useMemo(() => inferPreset(draft), [draft]);
  const isNew = !selectedLink;

  const [confirmDialog, setConfirmDialog] = useState<{
    open: boolean;
    title: string;
    description: string;
    confirmLabel: string;
    cancelLabel: string;
    destructive?: boolean;
    onConfirm: () => void;
  }>({
    open: false,
    title: "",
    description: "",
    confirmLabel: t("common:confirm"),
    cancelLabel: t("common:cancel"),
    onConfirm: () => {},
  });

  // Register close guard: when the Dialog tries to close (X button / ESC),
  // this function is called. Returns true when unsaved changes exist,
  // triggering the confirm dialog instead of closing.
  const handleConditionalClose = useCallback(() => {
    if (hasUnsavedChanges()) {
      setCloseConfirmOpen(true);
      return true; // blocked — content will show confirm
    }
    onClose();
    return false; // proceed with close
  }, [hasUnsavedChanges, onClose]);
  useEffect(() => {
    registerCloseGuard(handleConditionalClose);
  }, [registerCloseGuard, handleConditionalClose]);

  const updateDraft = (patch: Partial<DraftLink>) => {
    setDraft((prev) => ({ ...prev, ...patch }));
  };

  const saveLinkAndRules = async (): Promise<Link | null> => {
    setSaving(true);
    try {
      let link = selectedLink;
      if (!link) {
        link = await api.createDealRoomLink(roomId, {
          name: draft.name.trim(),
          require_email: draft.requireEmail,
          require_email_verification: draft.requireEmailVerification,
          require_nda: draft.requireNda,
          require_password: draft.requirePassword,
          password: draft.requirePassword && draft.password ? draft.password : undefined,
          expires_at: draft.expiresAt || undefined,
          download_enabled: draft.allowDownloading,
          watermark_enabled: draft.watermarkEnabled,
          ai_copilot_enabled: draft.aiCopilotEnabled,
          custom_domain: draft.customDomain || undefined,
          tags: draft.tags.length > 0 ? draft.tags : [],
          notify_on_access: draft.notifyOnAccess,
        });
      } else {
        await api.updateLinkFull(link.id, buildLinkPayload(draft, link));
      }

      await api.setLinkAccessRules(link.id, buildRules(draft));
      markClean();
      toast.success(t(selectedLink ? "share.saveSuccess" : "share.createSuccess"));
      await refetch();
      onChanged?.();
      return link;
    } catch {
      toast.error(t("common:error.saveFailed"));
      return null;
    } finally {
      setSaving(false);
    }
  };

  const handleSave = async () => {
    const validationErrors = validateDraft(draft, selectedLink, lt, now());
    if (Object.keys(validationErrors).length > 0) {
      setErrors(validationErrors);
      return;
    }
    const link = await saveLinkAndRules();
    if (link && isNew) {
      setTab("invite");
    }
  };

  const handleActiveChange = (checked: boolean) => {
    if (!selectedLink) return;
    const doUpdate = async () => {
      try {
        await api.updateLink(selectedLink.id, { status: checked ? "active" : "revoked" });
        await refetch();
        onChanged?.();
      } catch {
        toast.error(t("common:error.saveFailed"));
      }
    };
    if (!checked) {
      setConfirmDialog({
        open: true,
        title: t("share.disableConfirmTitle"),
        description: t("share.disableConfirmDescription"),
        confirmLabel: t("common:disable"),
        cancelLabel: t("common:cancel"),
        destructive: true,
        onConfirm: async () => {
          setConfirmDialog((prev) => ({ ...prev, open: false }));
          await doUpdate();
        },
      });
      return;
    }
    void doUpdate();
  };

  const handleInviteSend = async () => {
    if (!selectedLink) return;
    const parts = inviteEmailsRaw.split(/[,;\n\t]+/).map((s) => s.trim()).filter(Boolean);
    const valid: string[] = [];
    const invalid: string[] = [];
    for (const part of parts) {
      if (isValidEmail(part)) valid.push(part.toLowerCase());
      else invalid.push(part);
    }
    setInviteInvalid(invalid);
    if (valid.length === 0) return;

    setInviteSending(true);
    try {
      await api.inviteLinkViewers(selectedLink.id, valid);
      toast.success(lt("invite.sent", { count: valid.length }));
      setInviteEmailsRaw("");
      await refetch();
      onChanged?.();
    } finally {
      setInviteSending(false);
    }
  };

  const handleInviteResend = async (email: string) => {
    if (!selectedLink) return;
    setInviteSending(true);
    try {
      await api.inviteLinkViewers(selectedLink.id, [email]);
      await refetch();
    } finally {
      setInviteSending(false);
    }
  };

  const handleInviteRevoke = (invitation: LinkInvitation) => {
    if (!selectedLink) return;
    setConfirmDialog({
      open: true,
      title: t("invite.revokeConfirmTitle", { email: invitation.email }),
      description: t("invite.revokeConfirmDescription"),
      confirmLabel: t("invite.revokeConfirmButton"),
      cancelLabel: t("common:cancel"),
      destructive: true,
      onConfirm: async () => {
        setConfirmDialog((prev) => ({ ...prev, open: false }));
        try {
          await api.revokeLinkInvitation(selectedLink.id, invitation.id, true);
          await refetch();
          onChanged?.();
        } catch {
          // error toast handled by api client
        }
      },
    });
  };

  const publicUrl = getPublicUrl(selectedLink);

  const primaryAction =
    tab === "share"
      ? { label: isNew ? t("share.createLink") : t("share.saveLinkSettings"), onClick: handleSave }
      : tab === "access"
      ? { label: isNew ? t("share.createLink") : t("accessRules.saveAccessRules"), onClick: handleSave }
      : tab === "analytics"
      ? { label: t("common:close"), onClick: onClose }
      : { label: t("invite.sendInvitations"), onClick: handleInviteSend };

  const inviteHasInput = inviteEmailsRaw.trim().length > 0;

  return (
    <>
      <DialogHeader>
        <div className="flex items-start justify-between gap-4">
          <div className="min-w-0 flex-1 space-y-1">
            <DialogTitle className="flex items-center gap-2">
              <ShareNetwork size={20} />
              {isNew ? t("share.createTitle") : selectedLink?.name}
            </DialogTitle>
            {!isNew && publicUrl && (
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
          {!isNew && (
            <div className="flex items-center gap-2">
              <span className={selectedLink?.isActive ? "text-success-600" : "text-muted-foreground"}>
                {selectedLink?.isActive ? t("share.active") : t("share.inactive")}
              </span>
              <Switch
                checked={selectedLink?.isActive ?? false}
                onCheckedChange={handleActiveChange}
              />
            </div>
          )}
        </div>
      </DialogHeader>

      <AnimatePresence>
        {draft.allowedViewers.length > 0 && (
          <motion.div
            initial={reducedMotion ? {} : { opacity: 0, y: -8, height: 0 }}
            animate={{ opacity: 1, y: 0, height: "auto" }}
            exit={reducedMotion ? {} : { opacity: 0, y: -8, height: 0 }}
            className="rounded-md border border-warning-500/20 bg-warning-500/10 px-3 py-2 text-xs text-warning-700"
          >
            {t("share.restrictedAlert")}
          </motion.div>
        )}
      </AnimatePresence>

      <Tabs value={tab} onValueChange={(v) => setTab(v as typeof tab)} className="flex flex-1 flex-col overflow-hidden">
        <TabsList variant="line" className="w-full">
          <TabsTrigger value="share">{t("share.title")}</TabsTrigger>
          <TabsTrigger value="invite">{t("invite.title")}</TabsTrigger>
          <TabsTrigger value="access">{t("accessRules.title")}</TabsTrigger>
          {!isNew && (
            <TabsTrigger value="analytics">{t("analytics:title", { ns: "linkShare" })}</TabsTrigger>
          )}
        </TabsList>

        <div className="flex-1 overflow-y-auto py-2">
          {loadingData || !data ? (
            <div className="py-10 text-center text-sm text-muted-foreground">
              {t("common:loading")}
            </div>
          ) : (
            <>
              <AnimatePresence mode="wait" initial={false}>
              <motion.div key={tab} {...(reducedMotion ? {} : tabTransition)}>
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
                    link={selectedLink}
                    onEditAccess={() => setTab("access")}
                    errors={errors}
                    slug={slug}
                  />
                </TabsContent>
                <TabsContent value="invite">
                  <InviteTab
                    linkId={selectedLink?.id}
                    publicUrl={publicUrl}
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
                {!isNew && selectedLink && (
                  <TabsContent value="analytics">
                    <AnalyticsTab link={selectedLink} logs={logs} />
                  </TabsContent>
                )}
              </motion.div>
            </AnimatePresence>
            </>
          )}
        </div>
      </Tabs>

      <DialogFooter>
        <Button variant="outline" onClick={handleConditionalClose}>
          {t("common:cancel")}
        </Button>
        <Button
          className="min-w-[140px]"
          onClick={primaryAction.onClick}
          disabled={
            saving ||
            (tab === "invite" && (!inviteHasInput || inviteSending)) ||
            (tab === "analytics" && false)
          }
        >
          {saving || inviteSending ? t("common:saving") : primaryAction.label}
        </Button>
      </DialogFooter>

      <ConfirmDialog
        open={confirmDialog.open}
        title={confirmDialog.title}
        description={confirmDialog.description}
        confirmLabel={confirmDialog.confirmLabel}
        cancelLabel={confirmDialog.cancelLabel}
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

export function DealRoomShareDialog({
  roomId,
  linkId,
  slug,
  defaultTab,
  children,
  onChanged,
}: DealRoomShareDialogProps) {
  const [open, setOpen] = useState(false);
  const { data, loading, refetch } = useAsyncData(
    () => (open ? fetchDialogData(roomId, linkId) : Promise.resolve(null)),
    [open, roomId, linkId]
  );

  const dataKey = data ? (data.selectedLink?.id ?? "new") : "loading";

  // Close guard: the content registers a function that returns true when
  // unsaved changes exist. The wrapper's onOpenChange defers to it.
  const closeGuardRef = useRef<(() => boolean) | null>(null);
  const registerCloseGuard = useCallback((guard: () => boolean) => {
    closeGuardRef.current = guard;
  }, []);

  const handleOpenChange = useCallback((isOpen: boolean) => {
    if (!isOpen && closeGuardRef.current?.()) {
      return; // content handles confirmation
    }
    setOpen(isOpen);
  }, []);

  return (
    <Dialog open={open} onOpenChange={handleOpenChange}>
      {children && <DialogTrigger render={children} />}
      <DialogContent className="flex max-h-[90vh] flex-col sm:max-w-xl">
        {open && (
          <DealRoomShareDialogContent
            key={dataKey}
            roomId={roomId}
            slug={slug}
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
