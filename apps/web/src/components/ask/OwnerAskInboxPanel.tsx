import { useCallback, useEffect, useMemo, useState } from "react";
import { useTranslation } from "react-i18next";
import { toast } from "sonner";
import { ChatCircleText } from "@phosphor-icons/react";
import { Button } from "@/components/ui/button";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { EmptyState } from "@/components/common/EmptyState";
import { OwnerAskInboxCard } from "@/components/ask/OwnerAskInboxCard";
import { useAsyncData } from "@/hooks/useAsyncData";
import { api } from "@/lib/api";
import {
  countOwnerAskPendingAttention,
  moveOwnerAskPinnedFAQ,
  ownerAskFaqReorderEnabled,
  ownerAskInboxQuery,
  ownerAskInboxUsesPinnedFAQApi,
  ownerAskTurnSuggestPinFAQ,
  sortOwnerAskPinnedFAQs,
  type OwnerAskInboxView,
} from "@/lib/ownerAskInbox";
import type { DealRoomKnowledgeQueryHit, OwnerAskTurn } from "@/types";

export type OwnerAskInboxScope =
  | { type: "room"; roomId: string; linkFilter?: string }
  | { type: "link"; linkId: string };

export interface OwnerAskInboxPanelProps {
  scope: OwnerAskInboxScope;
  i18nNs: "dealRooms" | "linkShare";
  linkLabels?: Map<string, string>;
  initialView?: OwnerAskInboxView;
  onPendingCountChange?: (count: number) => void;
  onOpenCitation?: (hit: DealRoomKnowledgeQueryHit) => void;
}

function emptyTitleKey(view: OwnerAskInboxView, i18nNs: OwnerAskInboxPanelProps["i18nNs"]): string {
  if (i18nNs === "linkShare" && view === "needs_host") {
    return "noQuestions";
  }
  switch (view) {
    case "needs_host":
      return "emptyTitle";
    case "ai_handled":
      return "emptyAiTitle";
    case "pinned_faq":
      return "emptyPinnedFaqTitle";
    case "formal_queue":
      return "emptyFormalTitle";
    default:
      return "emptyAllTitle";
  }
}

function emptyDescriptionKey(
  view: OwnerAskInboxView,
  i18nNs: OwnerAskInboxPanelProps["i18nNs"],
): string {
  if (i18nNs === "linkShare" && view === "needs_host") {
    return "questionsDescription";
  }
  switch (view) {
    case "needs_host":
      return "emptyDescription";
    case "ai_handled":
      return "emptyAiDescription";
    case "pinned_faq":
      return "emptyPinnedFaqDescription";
    case "formal_queue":
      return "emptyFormalDescription";
    default:
      return "emptyAllDescription";
  }
}

export function OwnerAskInboxPanel({
  scope,
  i18nNs,
  linkLabels,
  initialView,
  onPendingCountChange,
  onOpenCitation,
}: OwnerAskInboxPanelProps) {
  const { t } = useTranslation(i18nNs);
  const prefix = i18nNs === "linkShare" ? "management" : "qa";
  const [view, setView] = useState<OwnerAskInboxView>(() => initialView ?? "needs_host");
  const [answerDraft, setAnswerDraft] = useState<Record<string, string>>({});
  const [localOverrides, setLocalOverrides] = useState<Record<string, OwnerAskTurn>>({});
  const [reordering, setReordering] = useState(false);

  const roomId = scope.type === "room" ? scope.roomId : undefined;
  const linkId = scope.type === "link" ? scope.linkId : undefined;
  const linkFilter = scope.type === "room" ? scope.linkFilter : undefined;

  const faqReorderLinkId =
    scope.type === "link"
      ? linkId
      : linkFilter && linkFilter !== "all"
        ? linkFilter
        : undefined;
  const faqReorderEnabled = ownerAskFaqReorderEnabled(view, scope);

  const fetchTurns = useCallback(async () => {
    if (ownerAskInboxUsesPinnedFAQApi(view)) {
      if (scope.type === "room" && roomId) {
        const filteredLinkId = linkFilter && linkFilter !== "all" ? linkFilter : undefined;
        const res = await api.listRoomAskPinnedFAQ(roomId, { linkId: filteredLinkId });
        return res.data ?? [];
      }
      if (scope.type === "link" && linkId) {
        const res = await api.listLinkAskPinnedFAQ(linkId);
        return res.data ?? [];
      }
      return [];
    }
    const query = ownerAskInboxQuery(view);
    if (scope.type === "room" && roomId) {
      const filteredLinkId = linkFilter && linkFilter !== "all" ? linkFilter : undefined;
      const res = await api.listRoomAsk(roomId, { ...query, linkId: filteredLinkId });
      return res.data ?? [];
    }
    if (scope.type === "link" && linkId) {
      const res = await api.listLinkAsk(linkId, query);
      return res.data ?? [];
    }
    return [];
  }, [scope.type, roomId, linkId, linkFilter, view]);

  const { data, loading, error, refetch } = useAsyncData(fetchTurns, [
    scope.type,
    roomId,
    linkId,
    linkFilter,
    view,
  ]);

  const turns = useMemo(() => {
    const base = data ?? [];
    const merged =
      Object.keys(localOverrides).length === 0
        ? base
        : base.map((turn) => localOverrides[turn.id] ?? turn);
    return view === "pinned_faq" ? sortOwnerAskPinnedFAQs(merged) : merged;
  }, [data, localOverrides, view]);

  useEffect(() => {
    if (!onPendingCountChange) return;
    let cancelled = false;
    const loadPendingAttention = async () => {
      try {
        const hostQuery = ownerAskInboxQuery("needs_host");
        const formalQuery = ownerAskInboxQuery("formal_queue");
        let hostTurns: OwnerAskTurn[] = [];
        let formalTurns: OwnerAskTurn[] = [];
        if (scope.type === "room" && roomId) {
          const filteredLinkId = linkFilter && linkFilter !== "all" ? linkFilter : undefined;
          const [hostRes, formalRes] = await Promise.all([
            api.listRoomAsk(roomId, { ...hostQuery, linkId: filteredLinkId }),
            api.listRoomAsk(roomId, { ...formalQuery, linkId: filteredLinkId }),
          ]);
          hostTurns = hostRes.data ?? [];
          formalTurns = formalRes.data ?? [];
        } else if (scope.type === "link" && linkId) {
          const [hostRes, formalRes] = await Promise.all([
            api.listLinkAsk(linkId, hostQuery),
            api.listLinkAsk(linkId, formalQuery),
          ]);
          hostTurns = hostRes.data ?? [];
          formalTurns = formalRes.data ?? [];
        }
        if (!cancelled) {
          onPendingCountChange(countOwnerAskPendingAttention(hostTurns, formalTurns));
        }
      } catch {
        // Keep last badge value on transient fetch errors.
      }
    };
    void loadPendingAttention();
    return () => {
      cancelled = true;
    };
  }, [scope.type, roomId, linkId, linkFilter, data, onPendingCountChange]);

  const handleAnswered = (turn: OwnerAskTurn) => {
    setLocalOverrides((prev) => ({ ...prev, [turn.id]: turn }));
    setAnswerDraft((prev) => ({ ...prev, [turn.id]: "" }));
    toast.success(t(`${prefix}.answerSuccess`));
    void refetch();
  };

  const handlePinned = (turn: OwnerAskTurn) => {
    setLocalOverrides((prev) => ({ ...prev, [turn.id]: turn }));
    void refetch();
  };

  const handleFormalPublished = (turn: OwnerAskTurn) => {
    setLocalOverrides((prev) => {
      const next = { ...prev };
      delete next[turn.id];
      return next;
    });
    void refetch();
  };

  const handleUnpinned = (turn: OwnerAskTurn) => {
    setLocalOverrides((prev) => {
      const next = { ...prev };
      delete next[turn.id];
      return next;
    });
    void refetch();
  };

  const handleMoveFAQ = async (turnId: string, direction: "up" | "down") => {
    if (!faqReorderLinkId) return;
    const reordered = moveOwnerAskPinnedFAQ(turns, turnId, direction);
    try {
      setReordering(true);
      const res = await api.reorderLinkAskFAQ(
        faqReorderLinkId,
        reordered.map((turn) => turn.id),
      );
      setLocalOverrides(Object.fromEntries((res.data ?? []).map((turn) => [turn.id, turn])));
      toast.success(t(`${prefix}.faqReorderSuccess`));
      void refetch();
    } catch {
      toast.error(t(`${prefix}.faqReorderFailed`));
    } finally {
      setReordering(false);
    }
  };

  const handleViewChange = (value: string) => {
    if (
      value === "all" ||
      value === "needs_host" ||
      value === "ai_handled" ||
      value === "pinned_faq" ||
      value === "formal_queue"
    ) {
      setView(value);
      setLocalOverrides({});
    }
  };

  const handleRetry = async () => {
    setLocalOverrides({});
    await refetch();
  };

  const listContent = loading ? (
    <p className="py-6 text-center text-sm text-muted-foreground">{t(`${prefix}.loading`)}</p>
  ) : error ? (
    <div className="space-y-3 py-4 text-center">
      <p className="text-sm text-muted-foreground">{t(`${prefix}.loadFailed`)}</p>
      <Button variant="outline" size="sm" onClick={() => void handleRetry()}>
        {t(`${prefix}.retry`)}
      </Button>
    </div>
  ) : turns.length === 0 ? (
    <EmptyState
      icon={<ChatCircleText size={40} />}
      title={t(`${prefix}.${emptyTitleKey(view, i18nNs)}`)}
      description={t(`${prefix}.${emptyDescriptionKey(view, i18nNs)}`)}
    />
  ) : (
    <ul className="space-y-4">
      {turns.map((turn, index) => (
        <OwnerAskInboxCard
          key={turn.id}
          turn={turn}
          linkLabel={linkLabels?.get(turn.link_id)}
          i18nNs={i18nNs}
          answerDraft={answerDraft[turn.id] ?? ""}
          onAnswerDraftChange={(value) =>
            setAnswerDraft((prev) => ({ ...prev, [turn.id]: value }))
          }
          onAnswered={handleAnswered}
          onFormalPublished={handleFormalPublished}
          onPinned={handlePinned}
          onUnpinned={handleUnpinned}
          repeatCount={view === "pinned_faq" ? undefined : turn.repeat_count}
          suggestPinFAQ={
            view === "pinned_faq" ? false : ownerAskTurnSuggestPinFAQ(turn)
          }
          faqReorder={
            faqReorderEnabled
              ? {
                  canMoveUp: index > 0,
                  canMoveDown: index < turns.length - 1,
                  moving: reordering,
                  onMoveUp: () => void handleMoveFAQ(turn.id, "up"),
                  onMoveDown: () => void handleMoveFAQ(turn.id, "down"),
                }
              : undefined
          }
          onOpenCitation={onOpenCitation}
        />
      ))}
    </ul>
  );

  return (
    <Tabs value={view} onValueChange={handleViewChange}>
      <TabsList aria-label={t(`${prefix}.inboxViewsLabel`)}>
        <TabsTrigger value="needs_host">{t(`${prefix}.inboxNeedsHost`)}</TabsTrigger>
        <TabsTrigger value="ai_handled">{t(`${prefix}.inboxAiHandled`)}</TabsTrigger>
        <TabsTrigger value="formal_queue">{t(`${prefix}.inboxFormalQueue`)}</TabsTrigger>
        <TabsTrigger value="pinned_faq">{t(`${prefix}.inboxPinnedFaq`)}</TabsTrigger>
        <TabsTrigger value="all">{t(`${prefix}.inboxAll`)}</TabsTrigger>
      </TabsList>
      <TabsContent value={view} className="mt-4">
        {listContent}
      </TabsContent>
    </Tabs>
  );
}
