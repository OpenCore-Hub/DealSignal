import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { useParams } from "react-router";
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
import { ApiError } from "@/lib/apiClient";
import { api } from "@/lib/api";
import type { AccessRule, DealRoomFolder, DealRoomFolderDocs, DealRoomKnowledgeBaseStatus, Link } from "@/types";
import { useAsyncData } from "@/hooks/useAsyncData";
import { ConfirmDialog } from "@/components/common/ConfirmDialog";
import {
  ShareTab,
  AccessTab,
  DocumentsTab,
  LinkAccessRequestsPanel,
  buildDraft,
  buildRules,
  buildAllowedLists,
  buildLinkPayload,
  toRFC3339,
  validateDraft,
} from "@/components/links/share";
import type { DraftLink } from "@/components/links/share";
import {
  askDocsCoverageWarningMessage,
  extractAskDocsWarnings,
  visitorAskSaveErrorMessage,
} from "@/components/links/share/visitorAskSaveFeedback";

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
  defaultTab?: "share" | "access" | "documents";
  children?: React.ReactElement;
  onChanged?: () => void | Promise<void>;
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
  folders: DealRoomFolder[];
  documents: DealRoomFolderDocs[];
}

async function fetchDialogData(roomId: string, linkId?: string): Promise<DialogData> {
  const [linksRes, docsRes, foldersRes] = await Promise.all([
    api.getDealRoomLinks(roomId),
    api.getDealRoomDocuments(roomId),
    api.getDealRoomFolders(roomId),
  ]);
  const loadedLinks = linksRes.data;
  const folders = foldersRes.data ?? [];
  const documents = docsRes.data ?? [];

  if (!linkId) {
    return { links: loadedLinks, selectedLink: null, rules: [], folders, documents };
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
    return { links: loadedLinks, selectedLink: null, rules: [], folders, documents };
  }

  const rulesRes = await api.getLinkAccessRules(selectedLink.id);

  return {
    links: loadedLinks,
    selectedLink,
    rules: rulesRes.data,
    folders,
    documents,
  };
}

interface DealRoomShareDialogContentProps {
  roomId: string;
  slug?: string;
  defaultTab?: "share" | "access" | "documents";
  data: DialogData | null;
  loadingData: boolean;
  refetch: () => Promise<void>;
  onChanged?: () => void | Promise<void>;
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
  const { workspaceSlug } = useParams<{ workspaceSlug: string }>();
  const reducedMotion = useReducedMotion();
  const [tab, setTab] = useState<"share" | "access" | "documents">(defaultTab);
  const [draft, setDraft] = useState<DraftLink>(() => buildDraft(data?.selectedLink, data?.rules));
  const [saving, setSaving] = useState(false);
  const [saveSuccess, setSaveSuccess] = useState(false);
  const [highlightedFields, setHighlightedFields] = useState<string[]>([]);
  const [ndaTemplates, setNdaTemplates] = useState<{ id: string; name: string; sourceDocumentId: string }[]>([]);
  const [knowledgeBaseStatus, setKnowledgeBaseStatus] = useState<DealRoomKnowledgeBaseStatus | null>(null);

  useEffect(() => {
    let cancelled = false;
    (async () => {
      try {
        const res = await api.listNDATemplates();
        if (cancelled) return;
        setNdaTemplates(
          (res.data ?? []).map((tpl) => ({
            id: tpl.id,
            name: tpl.name,
            sourceDocumentId: tpl.source_document_id,
          }))
        );
      } catch {
        if (!cancelled) setNdaTemplates([]);
      }
    })();
    return () => {
      cancelled = true;
    };
  }, []);

  useEffect(() => {
    let cancelled = false;
    (async () => {
      try {
        const kb = await api.getDealRoomKnowledgeBase(roomId);
        if (!cancelled) setKnowledgeBaseStatus(kb.status);
      } catch {
        if (!cancelled) setKnowledgeBaseStatus("none");
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [roomId]);

  const knowledgeBaseHref = workspaceSlug
    ? `/${workspaceSlug}/deal-rooms/${roomId}?tab=documents`
    : undefined;

  // Unsaved-changes tracking. We use a mutable ref instead of a callback so
  // the data-sync effect does not depend on the comparison function, which
  // would otherwise read draft/initialDraft and create a feedback loop.
  const [closeConfirmOpen, setCloseConfirmOpen] = useState(false);
  const hasUnsavedChangesRef = useRef(false);

  const markClean = useCallback(() => {
    hasUnsavedChangesRef.current = false;
  }, []);

  const selectedLink = data?.selectedLink ?? null;
  const isNew = !selectedLink;
  const isDealRoomLink = !isNew ? !!selectedLink?.dealRoomId : true;
  const existingNames = useMemo(
    () =>
      (data?.links ?? [])
        .filter((link) => link.id !== selectedLink?.id)
        .map((link) => link.name ?? "")
        .filter((name) => name.trim().length > 0),
    [data?.links, selectedLink?.id]
  );

  // 实时校验：所有必填项通过前，创建/保存按钮保持禁用。
  const validationErrors = useMemo(() => {
    if (loadingData || !data) return {};
    return validateDraft(draft, selectedLink, lt, now(), isDealRoomLink, existingNames);
  }, [draft, selectedLink, lt, isDealRoomLink, loadingData, data, existingNames]);

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
      setHighlightedFields([]);
      hasUnsavedChangesRef.current = false;
      loadedKeyRef.current = currentKey;
    } else if (currentKey && !hasUnsavedChangesRef.current) {
      // Same link, data refreshed (e.g. after save), no unsaved edits: echo server.
      const nextDraft = buildDraft(data?.selectedLink, data?.rules);
      setDraft(nextDraft);
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
      let savedPayload: unknown = link;
      if (!link) {
        const { allowedEmails, blockedEmails } = buildAllowedLists(draft);
        link = await api.createDealRoomLink(roomId, {
          name: draft.name.trim(),
          require_email: draft.requireEmail,
          require_email_verification: draft.requireEmailVerification,
          require_nda: draft.requireNda,
          nda_template_id: draft.requireNda ? (draft.ndaTemplateId || undefined) : undefined,
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
          folder_paths: draft.folderPaths,
        });
        savedPayload = link;
      } else {
        savedPayload = await api.updateLinkFull(link.id, buildLinkPayload(draft, link));
        await api.setLinkAccessRules(link.id, buildRules(draft));
      }

      markClean();
      setSaveSuccess(true);
      setTimeout(() => setSaveSuccess(false), 1500);
      toast.success(t(selectedLink ? "share.saveSuccess" : "share.createSuccess"));
      const coverage = askDocsCoverageWarningMessage(extractAskDocsWarnings(savedPayload), lt);
      if (coverage) {
        toast.warning(coverage);
      }
      await refetch();
      await onChanged?.();
      return link;
    } catch (err) {
      const kbGate =
        err instanceof ApiError ? visitorAskSaveErrorMessage(err, lt) : null;
      if (kbGate) {
        toast.error(kbGate);
      } else if (err instanceof ApiError && err.code === "duplicate_name") {
        toast.error(lt("share.linkNameDuplicate"));
      } else {
        toast.error(t("common:error.saveFailed"));
      }
      return null;
    } finally {
      setSaving(false);
    }
  };

  const handleSave = async () => {
    const currentErrors = validateDraft(draft, selectedLink, lt, now(), isDealRoomLink, existingNames);
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
        await onChanged?.();
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

  const primaryAction =
    tab === "share"
      ? { label: saveSuccess ? lt("share.savedButtonLabel") : isNew ? t("share.createLink") : t("share.saveLinkSettings"), onClick: handleSave }
      : tab === "access"
        ? { label: saveSuccess ? lt("accessRules.saved") : isNew ? t("share.createLink") : t("accessRules.saveAccessRules"), onClick: handleSave }
        : { label: saveSuccess ? lt("share.savedButtonLabel") : isNew ? t("share.createLink") : t("share.saveLinkSettings"), onClick: handleSave };

  return (
    <>
      <DialogHeader>
        <div className="flex items-start justify-between gap-4">
          <div className="min-w-0 flex-1 space-y-1">
            <DialogTitle className="flex items-center gap-2">
              <LinkIcon size={20} />
              {isNew ? t("share.createTitle") : selectedLink?.name}
            </DialogTitle>
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

      <Tabs value={tab} onValueChange={(v) => setTab(v as typeof tab)} className="flex flex-1 flex-col overflow-hidden">
        <TabsList variant="line" className="w-full">
          <TabsTrigger value="share">{lt("share.title")}</TabsTrigger>
          <TabsTrigger value="access">{lt("accessRules.title")}</TabsTrigger>
          <TabsTrigger value="documents">{lt("documents.title")}</TabsTrigger>
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
                    link={selectedLink}
                    onEditAccess={() => setTab("access")}
                    errors={validationErrors}
                    slug={slug}
                    highlightedFields={highlightedFields}
                    documents={data?.documents ?? []}
                  />
                </TabsContent>
                <TabsContent value="access" className="space-y-4">
                  {selectedLink ? (
                    <LinkAccessRequestsPanel
                      linkId={selectedLink.id}
                      onChanged={(detail) => {
                        if (detail?.action === "approve" && detail.email) {
                          const email = detail.email.trim().toLowerCase();
                          setDraft((prev) => {
                            if (prev.allowedViewers.some((v) => v.trim().toLowerCase() === email)) {
                              return prev;
                            }
                            return { ...prev, allowedViewers: [...prev.allowedViewers, email] };
                          });
                          // Approving grants access without marking the rest of the form dirty.
                        }
                        void refetch();
                      }}
                    />
                  ) : null}
                  <AccessTab
                    draft={draft}
                    updateDraft={updateDraft}
                    errors={validationErrors}
                    highlightedFields={highlightedFields}
                    isDealRoomLink={isDealRoomLink}
                    passwordAlreadySet={Boolean(selectedLink?.requirePassword)}
                    ndaTemplates={ndaTemplates}
                    knowledgeBaseStatus={knowledgeBaseStatus}
                    knowledgeBaseHref={knowledgeBaseHref}
                    documents={(data?.documents ?? [])
                      .flatMap((folder) => folder.documents ?? [])
                      .map((d) => ({ id: d.document_id, title: d.title }))}
                  />
                </TabsContent>
                <TabsContent value="documents">
                  <DocumentsTab
                    folders={data?.folders ?? []}
                    documents={data?.documents ?? []}
                    selectedPaths={draft.folderPaths}
                    scopeMode={draft.folderScopeMode}
                    onChange={({ scopeMode, selectedPaths }) =>
                      updateDraft({ folderScopeMode: scopeMode, folderPaths: selectedPaths })
                    }
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
