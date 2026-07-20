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
import { ApiError } from "@/lib/apiClient";
import type { AccessRule, Link } from "@/types";
import { useAsyncData } from "@/hooks/useAsyncData";
import { ConfirmDialog } from "@/components/common/ConfirmDialog";
import {
  ShareTab,
  AccessTab,
  buildDraft,
  buildRules,
  buildLinkPayload,
  validateDraft,
} from "./";
import type { DraftLink } from "./types";

interface LinkShareDialogProps {
  linkId: string;
  defaultTab?: "share" | "access";
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
}

async function fetchDialogData(linkId: string): Promise<DialogData | null> {
  const [link, rulesRes] = await Promise.all([
    api.getLinkById(linkId),
    api.getLinkAccessRules(linkId),
  ]);
  if (!link) return null;
  return {
    link,
    rules: rulesRes.data,
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
  defaultTab?: "share" | "access";
  data: DialogData | null;
  loadingData: boolean;
  refetch: () => Promise<void>;
  onChanged?: () => void;
  onClose: () => void;
  registerCloseGuard: (guard: () => boolean) => void;
}) {
  const { t } = useTranslation("linkShare");
  const [tab, setTab] = useState<"share" | "access">(defaultTab);
  const [draft, setDraft] = useState<DraftLink>(() => buildDraft(data?.link, data?.rules));
  const [saving, setSaving] = useState(false);
  const [saveSuccess, setSaveSuccess] = useState(false);
  const [highlightedFields, setHighlightedFields] = useState<string[]>([]);

  const link = data?.link ?? null;

  // 实时校验：所有必填项通过前，保存按钮保持禁用。
  const validationErrors = useMemo(() => {
    if (loadingData || !data || !link) return {};
    return validateDraft(draft, link, t, now(), !!link?.dealRoomId);
  }, [draft, link, t, loadingData, data]);

  // Unsaved-changes tracking. We use a mutable ref instead of a callback so
  // the data-sync effect does not depend on the comparison function, which
  // would otherwise read draft/initialDraft and create a feedback loop.
  const [closeConfirmOpen, setCloseConfirmOpen] = useState(false);
  const hasUnsavedChangesRef = useRef(false);
  const markClean = useCallback(() => {
    hasUnsavedChangesRef.current = false;
  }, []);

  // Rebuild draft when the underlying link data changes (e.g., first load or
  // switching to a different link). The key on the parent already remounts the
  // component in most cases, but this effect defends against stale state if
  // data arrives after mount without a key change, and also resets the unsaved-
  // changes baseline so the user is not warned about the loaded data itself.
  // It also re-echoes server state after a successful save/refetch when there are
  // no pending user edits.
  const loadedLinkIdRef = useRef<string | undefined>(data?.link?.id);
  useEffect(() => {
    const currentId = data?.link?.id;
    const keyChanged = currentId !== loadedLinkIdRef.current;
    if (keyChanged) {
      const nextDraft = buildDraft(data?.link, data?.rules);
      setDraft(nextDraft);
      setHighlightedFields([]);
      hasUnsavedChangesRef.current = false;
      loadedLinkIdRef.current = currentId;
    } else if (currentId && !hasUnsavedChangesRef.current) {
      // Same link, data refreshed (e.g. after save), no unsaved edits: echo server.
      const nextDraft = buildDraft(data?.link, data?.rules);
      setDraft(nextDraft);
      setHighlightedFields([]);
    }
  }, [data]);

  const handleConditionalClose = useCallback(() => {
    if (hasUnsavedChangesRef.current) {
      setCloseConfirmOpen(true);
      return true;
    }
    onClose();
    return false;
  }, [onClose]);
  useEffect(() => {
    registerCloseGuard(handleConditionalClose);
  }, [registerCloseGuard, handleConditionalClose]);

  const [confirmDialog, setConfirmDialog] = useState<{
    open: boolean;
    title: string;
    description: string;
    destructive?: boolean;
    onConfirm: () => void;
  }>({ open: false, title: "", description: "", onConfirm: () => {} });

  const updateDraft = (patch: Partial<DraftLink>) => {
    setDraft((prev) => ({ ...prev, ...patch }));
    hasUnsavedChangesRef.current = true;
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
    } catch (err) {
      console.error("saveLinkAndRules failed:", err);
      if (err instanceof ApiError && err.code === "duplicate_name") {
        toast.error(t("share.linkNameDuplicate"));
      } else {
        const message = err instanceof Error ? err.message : "";
        toast.error(message || t("common:error.saveFailed"));
      }
      return false;
    } finally {
      setSaving(false);
    }
  };

  const handleSave = async () => {
    if (!link) return;
    const currentErrors = validateDraft(draft, link, t, now(), !!link?.dealRoomId);
    if (Object.keys(currentErrors).length > 0) {
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

  const primaryAction =
    tab === "share"
      ? { label: saveSuccess ? t("share.savedButtonLabel") : t("share.saveLinkSettings"), onClick: handleSave }
      : { label: saveSuccess ? t("accessRules.saved") : t("accessRules.saveAccessRules"), onClick: handleSave };

  return (
    <>
      <DialogHeader>
        <div className="flex items-start justify-between gap-4">
          <div className="min-w-0 flex-1 space-y-1">
            <DialogTitle className="flex items-center gap-2">
              <ShareNetwork size={20} />
              {link?.name || t("share.title")}
            </DialogTitle>
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

      <Tabs value={tab} onValueChange={(v) => setTab(v as typeof tab)} className="flex flex-1 flex-col overflow-hidden">
        <TabsList variant="line" className="w-full">
          <TabsTrigger value="share">{t("share.title")}</TabsTrigger>
          <TabsTrigger value="access">{t("accessRules.title")}</TabsTrigger>

        </TabsList>

        <div className="flex-1 overflow-y-auto py-2 pr-4">
          {loadingData || !data ? (
            <div className="py-10 text-center text-sm text-muted-foreground">{t("common:loading")}</div>
          ) : (
            <AnimatePresence mode="wait" initial={false}>
              <motion.div key={tab} {...tabTransition}>
                <TabsContent value="share">
                  <ShareTab
                    draft={draft}
                    updateDraft={updateDraft}
                    link={link}
                    onEditAccess={() => setTab("access")}
                    errors={validationErrors}
                    highlightedFields={highlightedFields}
                  />
                </TabsContent>
                <TabsContent value="access">
                  <AccessTab
                    draft={draft}
                    updateDraft={updateDraft}
                    errors={validationErrors}
                    highlightedFields={highlightedFields}
                    isDealRoomLink={!!link?.dealRoomId}
                    documents={link?.documents.map((d) => ({ id: d.id, title: d.title })) ?? []}
                  />
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
            key={data?.link?.id ?? "loading"}
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
