import { useCallback, useEffect, useRef, useState } from "react";
import { api } from "@/lib/api";
import { buildRoomFollowUps } from "@/lib/knowledge/followUps";
import { isPromotableDeskFollowUpText } from "@/lib/knowledge/trustGates";
import type {
  DealRoomKnowledgeFollowUpSuggestion,
  DealRoomKnowledgeQATurn,
} from "@/types";

export type KnowledgeFollowUpChipState = {
  turnId: string;
  items: DealRoomKnowledgeFollowUpSuggestion[];
  source: string;
  upgrading: boolean;
};

type Translate = (key: string, params?: Record<string, string>) => string;

/**
 * Progressive follow-up chips: local split immediately, then llm/gap API upgrade.
 * Defers chip replacement while the dock is engaged (hover/focus)
 * so a mid-click swap cannot ask the wrong question.
 */
export function useKnowledgeFollowUpChips(
  roomId: string,
  turn: DealRoomKnowledgeQATurn | null,
  t: Translate,
): {
  chips: DealRoomKnowledgeFollowUpSuggestion[] | null;
  source: string;
  upgrading: boolean;
  setEngaged: (engaged: boolean) => void;
} {
  const [state, setState] = useState<KnowledgeFollowUpChipState | null>(null);
  const engagedRef = useRef(false);
  const pendingRef = useRef<Omit<KnowledgeFollowUpChipState, "upgrading"> | null>(
    null,
  );

  const setEngaged = useCallback((engaged: boolean) => {
    engagedRef.current = engaged;
    if (engaged) return;
    const pending = pendingRef.current;
    if (!pending) return;
    pendingRef.current = null;
    setState((prev) =>
      prev?.turnId === pending.turnId
        ? { ...pending, upgrading: false }
        : prev,
    );
  }, []);

  const turnId = turn?.id ?? null;

  useEffect(() => {
    pendingRef.current = null;
    if (!turnId || !turn) {
      setState(null);
      return;
    }

    const tips = buildRoomFollowUps({
      refused: turn.refused,
      resultStatus: turn.resultStatus,
      hits: turn.hits,
      answer: turn.answer,
      question: turn.question,
      claims: turn.claims,
      unresolved: turn.unresolved,
    });
    const templateItems = tips.map((tip) => ({
      id: tip.id,
      text: t(tip.messageKey, tip.params),
      kind: tip.kind,
      slot: tip.slot,
    }));
    const localSource = tips.some((tip) => tip.kind === "narrow") ? "template" : "gap";
    setState({
      turnId,
      items: templateItems,
      source: localSource,
      upgrading: true,
    });

    const ac = new AbortController();
    const requestedTurnId = turnId;
    void api
      .suggestDealRoomKnowledgeFollowUps(roomId, requestedTurnId, {
        signal: ac.signal,
      })
      .then((res) => {
        if (ac.signal.aborted) return;
        const source = (res.source || "template").trim() || "template";
        // Template payloads may carry server-locale strings; keep FE i18n chips (§9).
        // Upgrade only for LLM or deterministic gap split — never mission checklist dumps.
        if (!res.items?.length || (source !== "llm" && source !== "gap")) {
          setState((prev) =>
            prev?.turnId === requestedTurnId
              ? { ...prev, upgrading: false }
              : prev,
          );
          return;
        }
        // Defense-in-depth: never render meta / out-of-room upgraded chips.
        const safeItems = res.items.filter((it) =>
          isPromotableDeskFollowUpText(it.text),
        );
        if (!safeItems.length) {
          setState((prev) =>
            prev?.turnId === requestedTurnId
              ? { ...prev, upgrading: false }
              : prev,
          );
          return;
        }
        const next = {
          turnId: requestedTurnId,
          items: safeItems,
          source,
        };
        if (engagedRef.current) {
          pendingRef.current = next;
          setState((prev) =>
            prev?.turnId === requestedTurnId
              ? { ...prev, upgrading: false }
              : prev,
          );
          return;
        }
        setState({ ...next, upgrading: false });
      })
      .catch(() => {
        if (ac.signal.aborted) return;
        void api
          .recordDealRoomKnowledgeDeskEvent(roomId, {
            type: "followups_upgrade_failed",
          })
          .catch(() => {
            /* metrics must not surface toasts */
          });
        setState((prev) =>
          prev?.turnId === requestedTurnId
            ? { ...prev, upgrading: false }
            : prev,
        );
      });

    return () => {
      ac.abort();
      pendingRef.current = null;
    };
    // turn fields other than id are snapshotted when id changes (same as prior Tab effect).
    // eslint-disable-next-line react-hooks/exhaustive-deps -- turnId gates refresh
  }, [roomId, turnId, t]);

  return {
    chips: state?.items ?? null,
    source: state?.source ?? "template",
    upgrading: state?.upgrading ?? false,
    setEngaged,
  };
}
