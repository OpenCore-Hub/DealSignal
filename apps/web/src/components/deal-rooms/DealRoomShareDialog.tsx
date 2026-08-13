import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import { useParams } from "react-router";
import { toast } from "sonner";
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
import { ApiError } from "@/lib/apiClient";
import { apiErrorMessage } from "@/lib/apiErrors";
import { usageAtCap } from "@/lib/planQuota";
import { api } from "@/lib/api";
import type {
  AccessRule,
  BillingInfo,
  DealRoomAccessPolicy,
  DealRoomFolder,
  DealRoomFolderDocs,
  Link,
  WorkspaceViewerDomain,
} from "@/types";
import { useAsyncData } from "@/hooks/useAsyncData";
import { ConfirmDialog } from "@/components/common/ConfirmDialog";
import {
  AccessTab,
  DocumentsTab,
  ShareTab,
  buildAllowedLists,
  buildDealRoomLinkCreatePayload,
  buildLinkPayload,
  validateDraft,
} from "@/components/links/share";
import type { DraftLink } from "@/components/links/share";
import { useNdaPickerSources } from "@/components/links/share/hooks";
import { resolveNdaDocumentFallback } from "@/components/links/share/ndaPicker";
import { resolveShareViewerDomains } from "@/components/links/share/viewerDomains";
import {
  clampDraftToRoomPolicy,
  buildLinkScopedRules,
  hydrateCreateDraftFromRoomPolicy,
  hydrateEditDraftFromRoomPolicy,
  linkOnlyBlockedViewers,
  roomBlockedEmails,
  roomSecurityFloors,
} from "./roomAccessPolicy";

interface DealRoomShareDialogProps {
  roomId: string;
  linkId?: string;
  slug?: string;
  /** @deprecated Access settings live on the Access Control page tab. */
  defaultTab?: "share" | "access" | "documents";
  children?: React.ReactElement;
  onChanged?: () => void | Promise<void>;
  /** Jump to Access Control for the current link (e.g. from access summary). */
  onEditAccess?: (linkId: string) => void;
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
  policy: DealRoomAccessPolicy | null;
  viewerDomain: WorkspaceViewerDomain | null;
  billing: BillingInfo | null;
}

async function fetchDialogData(roomId: string, linkId?: string): Promise<DialogData> {
  const [linksRes, docsRes, foldersRes, policyRes, viewerDomain, billing] = await Promise.all([
    api.getDealRoomLinks(roomId),
    api.getDealRoomDocuments(roomId),
    api.getDealRoomFolders(roomId),
    api.getDealRoomAccessPolicy(roomId),
    api.getWorkspaceViewerDomain().catch(() => null),
    api.getBillingInfo().catch(() => null),
  ]);
  const loadedLinks = linksRes.data;
  const documents = docsRes.data ?? [];
  const folders = foldersRes.data ?? [];
  // Always prefer unwrapped policy object; never drop floors to null silently.
  const policy = policyRes?.data ?? (policyRes as unknown as DealRoomAccessPolicy | null) ?? null;

  if (!linkId) {
    return {
      links: loadedLinks,
      selectedLink: null,
      rules: [],
      folders,
      documents,
      policy,
      viewerDomain,
      billing,
    };
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
    return {
      links: loadedLinks,
      selectedLink: null,
      rules: [],
      folders,
      documents,
      policy,
      viewerDomain,
      billing,
    };
  }

  const rulesRes = await api.getLinkAccessRules(selectedLink.id);

  return {
    links: loadedLinks,
    selectedLink,
    rules: rulesRes.data,
    folders,
    documents,
    policy,
    viewerDomain,
    billing,
  };
}

interface DealRoomShareDialogContentProps {
  roomId: string;
  slug?: string;
  data: DialogData | null;
  loadingData: boolean;
  refetch: () => Promise<void>;
  onChanged?: () => void | Promise<void>;
  onEditAccess?: (linkId: string) => void;
  onClose: () => void;
  registerCloseGuard: (guard: () => boolean) => void;
}

function DealRoomShareDialogContent({
  roomId,
  slug,
  data,
  loadingData,
  refetch,
  onChanged,
  onEditAccess,
  onClose,
  registerCloseGuard,
}: DealRoomShareDialogContentProps) {
  const { t } = useTranslation("dealRooms");
  const { t: lt } = useTranslation("linkShare");
  const { workspaceSlug } = useParams<{ workspaceSlug?: string }>();
  const shareDomains = useMemo(
    () => resolveShareViewerDomains(data?.viewerDomain),
    [data?.viewerDomain],
  );
  const brandSettingsHref = workspaceSlug ? `/${workspaceSlug}/settings/brand` : undefined;
  const planFeatures = useMemo(() => {
    const billing = data?.billing;
    if (!billing) return undefined;
    return {
      watermarkEnabled: billing.watermarkEnabled,
      ndaEnabled: billing.ndaEnabled,
      visitorAskAiEnabled: billing.visitorAskAiEnabled,
      accessControlsEnabled: billing.accessControlsEnabled,
    };
  }, [data?.billing]);
  const linksAtCap = Boolean(
    data?.billing && usageAtCap(data.billing.linksUsed, data.billing.linksLimit),
  );

  const [draft, setDraft] = useState<DraftLink>(() => {
    if (data?.selectedLink) {
      return hydrateEditDraftFromRoomPolicy(data.selectedLink, data.rules, data.policy);
    }
    return hydrateCreateDraftFromRoomPolicy(data?.policy, {
      visitorAskAiEnabled: data?.billing?.visitorAskAiEnabled,
      watermarkEnabled: data?.billing?.watermarkEnabled,
      ndaEnabled: data?.billing?.ndaEnabled,
      accessControlsEnabled: data?.billing?.accessControlsEnabled,
    });
  });
  const [saving, setSaving] = useState(false);
  const [saveSuccess, setSaveSuccess] = useState(false);
  const [highlightedFields, setHighlightedFields] = useState<string[]>([]);
  const { ndaTemplates, agreementDocs } = useNdaPickerSources();

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
  const floors = roomSecurityFloors(data?.policy);
  const lockedRoomBlocks = roomBlockedEmails(data?.policy);
  const existingNames = useMemo(
    () =>
      (data?.links ?? [])
        .filter((link) => link.id !== selectedLink?.id)
        .map((link) => link.name ?? "")
        .filter((name) => name.trim().length > 0),
    [data?.links, selectedLink?.id]
  );

  // Always validate the floor-clamped draft so create cannot bypass room security.
  const effectiveDraft = useMemo(
    () => clampDraftToRoomPolicy(draft, data?.policy),
    [draft, data?.policy],
  );
  const validationErrors = useMemo(() => {
    if (loadingData || !data) return {};
    return validateDraft(effectiveDraft, selectedLink, lt, now(), isDealRoomLink, existingNames);
  }, [effectiveDraft, selectedLink, lt, isDealRoomLink, loadingData, data, existingNames]);

  // Rebuild draft when the underlying link data changes (first load, create vs
  // edit, or switching to a different link). The parent key already remounts the
  // component in most cases, but this effect defends against stale state if the
  // data arrives after mount without a key change, and resets the unsaved-
  // changes baseline so the loaded data itself is not treated as a modification.
  // Edit mode only: re-echo server state after save/refetch when there are no
  // pending user edits. Create mode must not reset on links-list refresh — that
  // runs before the dialog closes and would flash empty-name validation.
  const loadedKeyRef = useRef<string | undefined>(
    data ? (data.selectedLink?.id ?? "new") : undefined
  );
  useEffect(() => {
    const currentKey = data ? (data.selectedLink?.id ?? "new") : undefined;
    const keyChanged = currentKey !== loadedKeyRef.current;
    if (keyChanged) {
      const nextDraft = data?.selectedLink
        ? hydrateEditDraftFromRoomPolicy(data.selectedLink, data.rules, data.policy)
        : hydrateCreateDraftFromRoomPolicy(data?.policy, {
            visitorAskAiEnabled: data?.billing?.visitorAskAiEnabled,
            watermarkEnabled: data?.billing?.watermarkEnabled,
            ndaEnabled: data?.billing?.ndaEnabled,
            accessControlsEnabled: data?.billing?.accessControlsEnabled,
          });
      setDraft(nextDraft);
      setHighlightedFields([]);
      hasUnsavedChangesRef.current = false;
      loadedKeyRef.current = currentKey;
    } else if (data?.selectedLink && currentKey && !hasUnsavedChangesRef.current) {
      // Same link, data refreshed (e.g. after save), no unsaved edits: echo server.
      const nextDraft = hydrateEditDraftFromRoomPolicy(
        data.selectedLink,
        data.rules,
        data.policy,
      );
      setDraft(nextDraft);
      setHighlightedFields([]);
    }
  }, [data, roomId]);

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

  const updateDraft = useCallback(
    (patch: Partial<DraftLink>) => {
      setDraft((prev) =>
        clampDraftToRoomPolicy({ ...prev, ...patch }, data?.policy ?? null),
      );
      hasUnsavedChangesRef.current = true;
    },
    [data?.policy],
  );

  const saveLinkAndRules = async (): Promise<Link | null> => {
    setSaving(true);
    try {
      // Floor clamp is mandatory — never trust a draft that dropped room gates.
      const payloadDraft = clampDraftToRoomPolicy(draft, data?.policy);
      let link = selectedLink;
      if (!link) {
        const { allowedEmails } = buildAllowedLists(payloadDraft);
        const linkOnlyBlocked = linkOnlyBlockedViewers(payloadDraft, data?.policy);
        link = await api.createDealRoomLink(
          roomId,
          buildDealRoomLinkCreatePayload(payloadDraft, {
            allowedEmails,
            blockedEmails: linkOnlyBlocked,
          }),
        );
      } else {
        await api.updateLinkFull(link.id, buildLinkPayload(payloadDraft, link));
        await api.setLinkAccessRules(link.id, buildLinkScopedRules(payloadDraft, data?.policy));
      }

      markClean();
      setSaveSuccess(true);
      setTimeout(() => setSaveSuccess(false), 1500);
      toast.success(t(selectedLink ? "share.saveSuccess" : "share.createSuccess"));
      // Create: close dialog before refetch so empty create draft is never re-validated.
      if (selectedLink) {
        await refetch();
        await onChanged?.();
      }
      return link;
    } catch (err) {
      if (err instanceof ApiError && err.code === "duplicate_name") {
        toast.error(lt("share.linkNameDuplicate"));
      } else {
        toast.error(
          apiErrorMessage(err, {
            fallback: "saveFailed",
            messageKey: "common:error.saveFailed",
          }),
        );
      }
      return null;
    } finally {
      setSaving(false);
    }
  };

  const handleSave = async () => {
    if (isNew && linksAtCap) {
      toast.error(t("share.linkLimitReached"));
      return;
    }
    const payloadDraft = clampDraftToRoomPolicy(draft, data?.policy);
    if (payloadDraft !== draft) {
      setDraft(payloadDraft);
    }
    const currentErrors = validateDraft(
      payloadDraft,
      selectedLink,
      lt,
      now(),
      isDealRoomLink,
      existingNames,
    );
    if (Object.keys(currentErrors).length > 0) {
      setHighlightedFields(Object.keys(currentErrors));
      return;
    }
    const link = await saveLinkAndRules();
    if (link && isNew) {
      onClose();
      await refetch();
      await onChanged?.();
    }
  };

  const handleActiveChange = (checked: boolean) => {
    if (!selectedLink) return;
    const doUpdate = async () => {
      try {
        await api.updateLink(selectedLink.id, { status: checked ? "active" : "revoked" });
        await refetch();
        await onChanged?.();
      } catch (err) {
        toast.error(apiErrorMessage(err, { fallback: "saveFailed" }));
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

  const handleEditAccess = () => {
    // Audience + gates are edited in this dialog; Access Control is room defaults.
    if (onEditAccess && selectedLink) {
      onClose();
      onEditAccess(selectedLink.id);
      return;
    }
    toast.message(lt("share.audienceEditedHere"));
  };

  const primaryLabel = saveSuccess
    ? lt("share.savedButtonLabel")
    : isNew
      ? t("share.createLink")
      : t("share.saveLinkSettings");

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

      <div className="flex-1 space-y-6 overflow-y-auto px-1 py-2">
        {loadingData || !data ? (
          <div className="py-10 text-center text-sm text-muted-foreground">
            {t("common:loading")}
          </div>
        ) : (
          <>
            <ShareTab
              draft={effectiveDraft}
              updateDraft={updateDraft}
              link={selectedLink}
              onEditAccess={handleEditAccess}
              errors={validationErrors}
              slug={slug}
              highlightedFields={highlightedFields}
              documents={data.documents}
              availableDomains={shareDomains.availableDomains}
              pendingHostname={shareDomains.pendingHostname}
              brandSettingsHref={brandSettingsHref}
            />
            <div className="space-y-4">
              <div className="space-y-1">
                <h4 className="text-sm font-medium">{lt("documents.title")}</h4>
                <p className="text-xs text-muted-foreground">
                  {lt("share.documentScope.modeLabel")}
                </p>
              </div>
              <DocumentsTab
                folders={data.folders}
                documents={data.documents}
                selectedPaths={effectiveDraft.folderPaths}
                scopeMode={effectiveDraft.folderScopeMode}
                onChange={({ scopeMode, selectedPaths }) =>
                  updateDraft({ folderScopeMode: scopeMode, folderPaths: selectedPaths })
                }
              />
            </div>
            <AccessTab
              layout="compact"
              audienceMode="full"
              draft={effectiveDraft}
              updateDraft={updateDraft}
              errors={validationErrors}
              highlightedFields={highlightedFields}
              isDealRoomLink
              passwordAlreadySet={Boolean(selectedLink?.requirePassword)}
              roomSecurityFloors={{
                requireEmailVerification: floors.requireEmailVerification,
                requireNda: floors.requireNda,
              }}
              roomBlockedEmails={lockedRoomBlocks}
              ndaTemplates={ndaTemplates}
              documents={resolveNdaDocumentFallback(agreementDocs)}
              linkId={selectedLink?.id}
              planFeatures={planFeatures}
            />
          </>
        )}
      </div>

      {isNew && linksAtCap ? (
        <p className="px-1 text-xs text-muted-foreground" data-testid="share-link-limit-hint">
          {t("share.linkLimitReached")}
        </p>
      ) : null}
      <DialogFooter>
        <Button variant="outline" onClick={handleConditionalClose}>
          {t("common:cancel")}
        </Button>
        <Button
          className="min-w-[140px]"
          onClick={handleSave}
          disabled={
            saving ||
            loadingData ||
            !data ||
            Object.keys(validationErrors).length > 0 ||
            (isNew && linksAtCap)
          }
        >
          {saving ? (
            t("common:saving")
          ) : saveSuccess ? (
            <span className="flex items-center gap-1.5">
              <Check size={16} />
              {primaryLabel}
            </span>
          ) : (
            primaryLabel
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
  children,
  onChanged,
  onEditAccess,
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

  const dataKey = data
    ? (linkId ?? data.selectedLink?.id ?? "new")
    : "loading";

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
            data={data}
            loadingData={loading}
            refetch={refetch}
            onChanged={onChanged}
            onEditAccess={onEditAccess}
            onClose={() => setOpen(false)}
            registerCloseGuard={registerCloseGuard}
          />
        )}
      </DialogContent>
    </Dialog>
  );
}
