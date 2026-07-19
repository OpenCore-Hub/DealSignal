import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import { toast } from "sonner";
import { useReducedMotion } from "motion/react";
import { motion, AnimatePresence } from "motion/react";
import { Link as LinkIcon, Check } from "@phosphor-icons/react";
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
import type { AccessRule, Link } from "@/types";
import { useAsyncData } from "@/hooks/useAsyncData";
import { ConfirmDialog } from "@/components/common/ConfirmDialog";
import {
  ShareTab,
  AccessTab,
  CopyButton,
  applyPreset,
  buildDraft,
  buildRules,
  buildAllowedLists,
  buildLinkPayload,
  inferPreset,
  toRFC3339,
  validateDraft,
  getPublicUrl,
} from "@/components/links/share";
import type { DraftLink, LinkPreset } from "@/components/links/share";

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
  defaultTab?: "share" | "access";
  children?: React.ReactElement;
  onChanged?: () => void;
  open?: boolean;
  onOpenChange?: (open: boolean) => void;
}


function now(): number {
  return Date.now();
}

interface DialogData {
  links: Link[];
  selectedLink: Link | null;
  rules: AccessRule[];
  documents: { id: string; title: string }[];
}

async function fetchDialogData(roomId: string, linkId?: string): Promise<DialogData> {
  const [linksRes, docsRes] = await Promise.all([
    api.getDealRoomLinks(roomId),
    api.getDealRoomDocuments(roomId),
  ]);
  const loadedLinks = linksRes.data;

  const documents = (docsRes.data ?? [])
    .flatMap((folder) => folder.documents ?? [])
    .map((d) => ({ id: d.document_id, title: d.title }));

  if (!linkId) {
    return { links: loadedLinks, selectedLink: null, rules: [], documents };
  }

  let selectedLink = loadedLinks.find((l) => l.id === linkId) || null;

  // Edit mode must not depend solely on the deal-room link list. The list can
  // be stale after creation, filtered by status, or cached; if the link is
  // missing, fall back to a direct lookup so saved rules are still loaded.
  if (!selectedLink) {
    try {
      const directLink = await api.getLinkById(linkId);
      if (directLink.dealRoomId === roomId) {
        selectedLink = directLink;
      }
    } catch {
      selectedLink = null;
    }
  }

  if (!selectedLink) {
    return { links: loadedLinks, selectedLink: null, rules: [], documents };
  }

  const rulesRes = await api.getLinkAccessRules(selectedLink.id);

  return {
    links: loadedLinks,
    selectedLink,
    rules: rulesRes.data,
    documents,
  };
}

interface DealRoomShareDialogContentProps {
  roomId: string;
  slug?: string;
  defaultTab?: "share" | "access";
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
  const [tab, setTab] = useState<"share" | "access">(defaultTab);
  const [draft, setDraft] = useState<DraftLink>(() => buildDraft(data?.selectedLink, data?.rules));
  const [saving, setSaving] = useState(false);
  const [saveSuccess, setSaveSuccess] = useState(false);
  const [highlightedFields, setHighlightedFields] = useState<string[]>([]);

  // Unsaved-changes tracking. We use a mutable ref instead of a callback so
  // the data-sync effect does not depend on the comparison function, which
  // would otherwise read draft/initialDraft and create a feedback loop.
  const [closeConfirmOpen, setCloseConfirmOpen] = useState(false);
  const hasUnsavedChangesRef = useRef(false);

  const markClean = useCallback(() => {
    hasUnsavedChangesRef.current = false;
  }, []);

  const selectedLink = data?.selectedLink ?? null;
  const [presetOverride, setPresetOverride] = useState<LinkPreset | null>(null);
  const preset = presetOverride ?? inferPreset(draft);
  const isNew = !selectedLink;
  const isDealRoomLink = !isNew ? !!selectedLink?.dealRoomId : true;

  // 实时校验：所有必填项通过前，创建/保存按钮保持禁用。
  const validationErrors = useMemo(() => {
    if (loadingData || !data) return {};
    return validateDraft(draft, selectedLink, lt, now(), isDealRoomLink);
  }, [draft, selectedLink, lt, isDealRoomLink, loadingData, data]);

  // Rebuild draft when the underlying link data changes (first load, create vs
  // edit, or switching to a different link). The parent key already remounts the
  // component in most cases, but this effect defends against stale state if the
  // data arrives after mount without a key change, and resets the unsaved-
  // changes baseline so the loaded data itself is not treated as a modification.
  // It also re-echoes server state after a successful save/refetch when there are
  // no pending user edits.
  const loadedKeyRef = useRef<string | undefined>(
    data ? (data.selectedLink?.id ?? "new") : undefined
  );
  useEffect(() => {
    const currentKey = data ? (data.selectedLink?.id ?? "new") : undefined;
    const keyChanged = currentKey !== loadedKeyRef.current;
    if (keyChanged) {
      const nextDraft = buildDraft(data?.selectedLink, data?.rules);
      setDraft(nextDraft);
      setPresetOverride(null);
      setHighlightedFields([]);
      hasUnsavedChangesRef.current = false;
      loadedKeyRef.current = currentKey;
    } else if (currentKey && !hasUnsavedChangesRef.current) {
      // Same link, data refreshed (e.g. after save), no unsaved edits: echo server.
      const nextDraft = buildDraft(data?.selectedLink, data?.rules);
      setDraft(nextDraft);
      setPresetOverride(null);
      setHighlightedFields([]);
    }
  }, [data]);

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
    if (hasUnsavedChangesRef.current) {
      setCloseConfirmOpen(true);
      return true; // blocked — content will show confirm
    }
    onClose();
    return false; // proceed with close
  }, [onClose]);
  useEffect(() => {
    registerCloseGuard(handleConditionalClose);
  }, [registerCloseGuard, handleConditionalClose]);

  const updateDraft = (patch: Partial<DraftLink>) => {
    setDraft((prev) => ({ ...prev, ...patch }));
    hasUnsavedChangesRef.current = true;
  };

  const saveLinkAndRules = async (): Promise<Link | null> => {
    setSaving(true);
    try {
      let link = selectedLink;
      if (!link) {
        const { allowedEmails, blockedEmails } = buildAllowedLists(draft);
        link = await api.createDealRoomLink(roomId, {
          name: draft.name.trim(),
          require_email: draft.requireEmail,
          require_email_verification: draft.requireEmailVerification,
          require_nda: draft.requireNda,
          nda_document_id: draft.requireNda ? draft.ndaDocumentId : undefined,
          require_password: draft.requirePassword,
          password: draft.requirePassword && draft.password ? draft.password : undefined,
          allowed_emails: allowedEmails.length > 0 ? allowedEmails : undefined,
          blocked_emails: blockedEmails.length > 0 ? blockedEmails : undefined,
          expires_at: toRFC3339(draft.expiresAt) || undefined,
          download_enabled: draft.allowDownloading,
          watermark_enabled: draft.watermarkEnabled,
          ai_copilot_enabled: draft.aiCopilotEnabled,
          qa_enabled: draft.enableQaConversations,
          file_requests_enabled: draft.enableFileRequests,
          index_file_enabled: draft.enableIndexFileGeneration,
          screenshot_protection_enabled: draft.enableScreenshotProtection,
          custom_domain: draft.customDomain || undefined,
          notify_on_access: draft.notifyOnAccess,
        });
      } else {
        await api.updateLinkFull(link.id, buildLinkPayload(draft, link));
        await api.setLinkAccessRules(link.id, buildRules(draft));
      }

      markClean();
      setSaveSuccess(true);
      setTimeout(() => setSaveSuccess(false), 1500);
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
    const currentErrors = validateDraft(draft, selectedLink, lt, now(), isDealRoomLink);
    if (Object.keys(currentErrors).length > 0) {
      return;
    }
    const link = await saveLinkAndRules();
    if (link && isNew) {
      onClose();
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

  const publicUrl = getPublicUrl(selectedLink);

  const primaryAction =
    tab === "share"
      ? { label: saveSuccess ? lt("share.savedButtonLabel") : isNew ? t("share.createLink") : t("share.saveLinkSettings"), onClick: handleSave }
      : { label: saveSuccess ? lt("accessRules.saved") : isNew ? t("share.createLink") : t("accessRules.saveAccessRules"), onClick: handleSave };

  return (
    <>
      <DialogHeader>
        <div className="flex items-start justify-between gap-4">
          <div className="min-w-0 flex-1 space-y-1">
            <DialogTitle className="flex items-center gap-2">
              <LinkIcon size={20} />
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
          <TabsTrigger value="access">{t("accessRules.title")}</TabsTrigger>

        </TabsList>

        <div className="flex-1 overflow-y-auto px-3 py-2" style={{ scrollbarGutter: "stable" }}>
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
                      if (name === "custom") {
                        setPresetOverride("custom");
                        return;
                      }
                      setPresetOverride(null);
                      const { patch, changedFields } = applyPreset(name, draft);
                      updateDraft(patch);
                      setHighlightedFields(changedFields);
                      setTimeout(() => setHighlightedFields([]), 200);
                    }}
                    link={selectedLink}
                    onEditAccess={() => setTab("access")}
                    errors={validationErrors}
                    slug={slug}
                    highlightedFields={highlightedFields}
                  />
                </TabsContent>
                <TabsContent value="access">
                  <AccessTab
                    draft={draft}
                    updateDraft={updateDraft}
                    errors={validationErrors}
                    highlightedFields={highlightedFields}
                    isDealRoomLink={isDealRoomLink}
                    documents={data?.documents ?? []}
                  />
                </TabsContent>

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
            Object.keys(validationErrors).length > 0
          }
        >
          {saving ? (
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
  open: openProp,
  onOpenChange,
}: DealRoomShareDialogProps) {
  const [openState, setOpenState] = useState(false);
  const open = openProp ?? openState;
  const setOpen = useCallback(
    (value: boolean) => {
      setOpenState(value);
      onOpenChange?.(value);
    },
    [onOpenChange]
  );
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

  const handleOpenChange = useCallback(
    (isOpen: boolean) => {
      if (!isOpen && closeGuardRef.current?.()) {
        return; // content handles confirmation
      }
      setOpen(isOpen);
    },
    [setOpen]
  );

  return (
    <Dialog open={open} onOpenChange={handleOpenChange}>
      {children && <DialogTrigger render={children} />}
      <DialogContent className="flex max-h-[90vh] flex-col sm:max-w-2xl">
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
