import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import { toast } from "sonner";
import { Check } from "@phosphor-icons/react";
import { Button } from "@/components/ui/button";
import { ApiError } from "@/lib/apiClient";
import { api } from "@/lib/api";
import type { AccessRule, DealRoomFolderDocs, Link } from "@/types";
import { useAsyncData } from "@/hooks/useAsyncData";
import { DealRoomAccessRequestsPanel } from "./DealRoomAccessRequestsPanel";
import {
  AccessTab,
  LinkAccessRequestsPanel,
  buildDraft,
  buildRules,
  buildLinkPayload,
  validateDraft,
} from "@/components/links/share";
import type { DraftLink } from "@/components/links/share";
import {
  loadRoomAccessDefaults,
  saveRoomAccessDefaults,
} from "./roomAccessDefaults";

interface LinkEditorData {
  links: Link[];
  selectedLink: Link | null;
  rules: AccessRule[];
  documents: DealRoomFolderDocs[];
}

async function fetchEditorData(roomId: string, linkId?: string | null): Promise<LinkEditorData> {
  const [linksRes, docsRes] = await Promise.all([
    api.getDealRoomLinks(roomId),
    api.getDealRoomDocuments(roomId),
  ]);
  const links = linksRes.data ?? [];
  const documents = docsRes.data ?? [];

  const preferredId = linkId && linkId !== "__new__" ? linkId : links[0]?.id;
  if (!preferredId) {
    return { links, selectedLink: null, rules: [], documents };
  }

  let selectedLink = links.find((l) => l.id === preferredId) ?? null;
  if (!selectedLink) {
    try {
      const direct = await api.getLinkById(preferredId);
      if (direct.dealRoomId === roomId) selectedLink = direct;
    } catch {
      selectedLink = null;
    }
  }
  if (!selectedLink) {
    return { links, selectedLink: null, rules: [], documents };
  }

  const rulesRes = await api.getLinkAccessRules(selectedLink.id);
  return { links, selectedLink, rules: rulesRes.data ?? [], documents };
}

/** Access-control tab only validates visitor-security fields (not share-link naming). */
function validateAccessFields(
  draft: DraftLink,
  selectedLink: Link | null,
  lt: (key: string, options?: Record<string, unknown>) => string,
  existingNames: string[]
): Record<string, string> {
  const errors = validateDraft(draft, selectedLink, lt, Date.now(), true, existingNames);
  delete errors.name;
  delete errors.expiresAt;
  delete errors.customDomain;
  return errors;
}

interface DealRoomAccessControlTabProps {
  roomId: string;
  /** Prefer this link when applying settings that are stored per share link. */
  initialLinkId?: string;
  onChanged?: () => void | Promise<void>;
}

export function DealRoomAccessControlTab({
  roomId,
  initialLinkId,
  onChanged,
}: DealRoomAccessControlTabProps) {
  const { t: lt } = useTranslation("linkShare");
  const { t: tc } = useTranslation("common");

  const [draft, setDraft] = useState<DraftLink>(() => buildDraft(null, []));
  const [saving, setSaving] = useState(false);
  const [saveSuccess, setSaveSuccess] = useState(false);
  const [highlightedFields, setHighlightedFields] = useState<string[]>([]);
  const [ndaTemplates, setNdaTemplates] = useState<
    { id: string; name: string; sourceDocumentId: string }[]
  >([]);
  const dirtyRef = useRef(false);

  const preferredLinkId =
    initialLinkId && initialLinkId !== "__new__" ? initialLinkId : undefined;

  const {
    data,
    loading,
    refetch,
  } = useAsyncData(() => fetchEditorData(roomId, preferredLinkId), [roomId, preferredLinkId]);

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

  const selectedLink = data?.selectedLink ?? null;
  const existingNames = useMemo(
    () =>
      (data?.links ?? [])
        .filter((link) => link.id !== selectedLink?.id)
        .map((link) => link.name ?? "")
        .filter((name) => name.trim().length > 0),
    [data?.links, selectedLink?.id]
  );

  const loadedKeyRef = useRef<string | undefined>(undefined);
  useEffect(() => {
    if (!data) return;
    const currentKey = data.selectedLink?.id ?? `room:${roomId}`;
    const keyChanged = currentKey !== loadedKeyRef.current;
    if (keyChanged || !dirtyRef.current) {
      if (data.selectedLink) {
        setDraft(buildDraft(data.selectedLink, data.rules));
      } else {
        setDraft(loadRoomAccessDefaults(roomId) ?? buildDraft(null, []));
      }
      setHighlightedFields([]);
      dirtyRef.current = false;
      loadedKeyRef.current = currentKey;
    }
  }, [data, roomId]);

  const validationErrors = useMemo(() => {
    if (loading || !data) return {};
    return validateAccessFields(draft, selectedLink, lt, existingNames);
  }, [draft, selectedLink, lt, loading, data, existingNames]);

  const updateDraft = useCallback((patch: Partial<DraftLink>) => {
    setDraft((prev) => ({ ...prev, ...patch }));
    dirtyRef.current = true;
  }, []);

  const handleSave = async () => {
    const currentErrors = validateAccessFields(draft, selectedLink, lt, existingNames);
    if (Object.keys(currentErrors).length > 0) {
      setHighlightedFields(Object.keys(currentErrors));
      return;
    }

    setSaving(true);
    try {
      if (selectedLink) {
        await api.updateLinkFull(selectedLink.id, buildLinkPayload(draft, selectedLink));
        await api.setLinkAccessRules(selectedLink.id, buildRules(draft));
      } else {
        // Room-level defaults until the first share link exists; create-link dialog hydrates these.
        saveRoomAccessDefaults(roomId, draft);
      }

      dirtyRef.current = false;
      setSaveSuccess(true);
      setTimeout(() => setSaveSuccess(false), 1500);
      toast.success(lt("accessRules.saved"));
      await refetch();
      await onChanged?.();
    } catch (err) {
      if (err instanceof ApiError && err.code === "duplicate_name") {
        toast.error(lt("share.linkNameDuplicate"));
      } else {
        toast.error(tc("error.saveFailed"));
      }
    } finally {
      setSaving(false);
    }
  };

  return (
    <div className="space-y-5" data-testid="deal-room-access-control-tab">
      <DealRoomAccessRequestsPanel roomId={roomId} onChanged={() => void refetch()} />

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
              dirtyRef.current = true;
            }
            void refetch();
          }}
        />
      ) : null}

      {loading || !data ? (
        <p className="py-10 text-center text-sm text-muted-foreground">{tc("loading")}</p>
      ) : (
        <AccessTab
          layout="sections"
          draft={draft}
          updateDraft={updateDraft}
          errors={validationErrors}
          highlightedFields={highlightedFields}
          isDealRoomLink
          passwordAlreadySet={Boolean(selectedLink?.requirePassword)}
          ndaTemplates={ndaTemplates}
          documents={(data.documents ?? [])
            .flatMap((folder) => folder.documents ?? [])
            .map((d) => ({ id: d.document_id, title: d.title }))}
        />
      )}

      {!loading && data ? (
        // Offset by AppShell main padding (p-6 / md:p-8) so the bar sits flush
        // with the viewport bottom instead of floating above the padded edge.
        <div className="sticky bottom-[-1.5rem] z-10 -mx-6 border-t bg-background/95 px-6 pt-3 pb-[calc(0.75rem+1.5rem)] backdrop-blur supports-[backdrop-filter]:bg-background/80 md:bottom-[-2rem] md:-mx-8 md:px-8 md:pb-[calc(0.75rem+2rem)]">
          <div className="flex items-center justify-between gap-3">
            <p className="text-xs text-muted-foreground">{lt("accessRules.savedDescription")}</p>
            <Button
              type="button"
              onClick={() => void handleSave()}
              disabled={saving || Object.keys(validationErrors).length > 0}
              className="min-w-[140px]"
            >
              {saving ? (
                tc("saving")
              ) : saveSuccess ? (
                <span className="flex items-center gap-1.5">
                  <Check size={16} />
                  {lt("share.savedButtonLabel")}
                </span>
              ) : (
                lt("accessRules.saveAccessRules")
              )}
            </Button>
          </div>
        </div>
      ) : null}
    </div>
  );
}
