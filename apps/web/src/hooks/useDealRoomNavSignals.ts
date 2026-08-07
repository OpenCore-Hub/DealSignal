import { useEffect, useRef } from "react";
import { api } from "@/lib/api";
import { countUnreadOwnerAskTurns } from "@/lib/ownerAskInbox";
import { useDealRoomNavStore } from "@/stores/dealRoomNavStore";
import type { Link, OwnerAskTurn } from "@/types";

const ANALYTICS_LINK_CAP = 8;

export function isLinkActive(link: Link): boolean {
  if (link.isActive === false) return false;
  if (
    link.status === "disabled" ||
    link.status === "deleted" ||
    link.status === "expired" ||
    link.status === "revoked"
  ) {
    return false;
  }
  return true;
}

function linksFromResponse(res: { data?: Link[] } | Link[] | null | undefined): Link[] {
  if (!res) return [];
  if (Array.isArray(res)) return res;
  return Array.isArray(res.data) ? res.data : [];
}

/**
 * Loads lightweight cross-tab signals for the deal-room sidebar badges
 * and documents attention banner. Deep remediates stay on Share / Q&A.
 */
export function useDealRoomNavSignals(roomId: string | undefined, refreshKey = 0) {
  const setSignals = useDealRoomNavStore((s) => s.setSignals);
  const clear = useDealRoomNavStore((s) => s.clear);
  const generationRef = useRef(0);
  const activeRoomIdRef = useRef<string | undefined>(undefined);

  useEffect(() => {
    if (!roomId) {
      activeRoomIdRef.current = undefined;
      clear();
      return;
    }

    const roomChanged = activeRoomIdRef.current !== roomId;
    activeRoomIdRef.current = roomId;
    if (roomChanged) {
      // Reset only when switching rooms; avoid clearing mid-refresh (banner flicker).
      clear();
    }

    const generation = ++generationRef.current;
    let cancelled = false;

    async function load() {
      try {
        const linksRes = await api.getDealRoomLinks(roomId!);
        if (cancelled || generation !== generationRef.current) return;

        const links = linksFromResponse(linksRes).filter(isLinkActive);
        const viewCount = links.reduce((sum, l) => sum + (l.accessCount ?? 0), 0);

        // Publish link count immediately so documents home doesn't wait on analytics.
        setSignals({
          roomId: roomId!,
          failedDeliveries: 0,
          unreadQuestions: 0,
          activeLinkCount: links.length,
          viewCount,
        });

        const [roomAskRes, analyticsResults] = await Promise.all([
          api.listRoomAsk(roomId!).catch(() => ({ data: [] as OwnerAskTurn[] })),
          Promise.all(
            links.slice(0, ANALYTICS_LINK_CAP).map(async (link) => {
              if (!link.requireEmailVerification) return null;
              return api.getLinkAnalytics(link.id).catch(() => null);
            }),
          ),
        ]);

        if (cancelled || generation !== generationRef.current) return;

        const askPayload = roomAskRes as { data?: OwnerAskTurn[] } | OwnerAskTurn[];
        const askTurns = Array.isArray(askPayload) ? askPayload : (askPayload.data ?? []);
        const unreadQuestions = countUnreadOwnerAskTurns(askTurns);

        let failedDeliveries = 0;
        for (const analyticsRes of analyticsResults) {
          if (!analyticsRes) continue;
          const row = (
            "data" in analyticsRes && analyticsRes.data != null ? analyticsRes.data : analyticsRes
          ) as {
            access_code_failed_count?: number;
            access_code_contacts?: { send_status: string }[];
          };
          failedDeliveries +=
            row.access_code_failed_count ??
            (row.access_code_contacts ?? []).filter((c) => c.send_status === "failed").length;
        }

        setSignals({
          roomId: roomId!,
          failedDeliveries,
          unreadQuestions,
          activeLinkCount: links.length,
          viewCount,
        });
      } catch {
        if (cancelled || generation !== generationRef.current) return;
        setSignals({
          roomId: roomId!,
          failedDeliveries: 0,
          unreadQuestions: 0,
          activeLinkCount: 0,
          viewCount: 0,
        });
      }
    }

    void load();

    return () => {
      cancelled = true;
    };
  }, [roomId, refreshKey, setSignals, clear]);
}

export async function fetchDealRoomLinks(roomId: string): Promise<Link[]> {
  const res = await api.getDealRoomLinks(roomId);
  return linksFromResponse(res).filter(isLinkActive);
}
