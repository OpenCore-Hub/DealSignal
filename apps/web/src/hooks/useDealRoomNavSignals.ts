import { useEffect, useRef } from "react";
import { api } from "@/lib/api";
import { useDealRoomNavStore } from "@/stores/dealRoomNavStore";
import type { Link } from "@/types";

const ANALYTICS_LINK_CAP = 8;

function isLinkActive(link: Link): boolean {
  if (link.isActive === false) return false;
  if (link.status === "disabled" || link.status === "deleted" || link.status === "expired") {
    return false;
  }
  return true;
}

/**
 * Loads lightweight cross-tab signals for the deal-room sidebar badges
 * and documents attention banner. Deep remediates stay on Share / Q&A.
 */
export function useDealRoomNavSignals(roomId: string | undefined, refreshKey = 0) {
  const setSignals = useDealRoomNavStore((s) => s.setSignals);
  const clear = useDealRoomNavStore((s) => s.clear);
  const generationRef = useRef(0);

  useEffect(() => {
    if (!roomId) {
      clear();
      return;
    }

    const generation = ++generationRef.current;
    let cancelled = false;

    async function load() {
      try {
        const linksRes = await api.getDealRoomLinks(roomId!);
        if (cancelled || generation !== generationRef.current) return;

        const links = (linksRes.data ?? []).filter(isLinkActive);
        const viewCount = links.reduce((sum, l) => sum + (l.accessCount ?? 0), 0);

        let failedDeliveries = 0;
        let unreadQuestions = 0;

        const [roomQuestionsRes, analyticsResults] = await Promise.all([
          api.listRoomQuestions(roomId!).catch(() => ({ data: [] as { status: string }[] })),
          Promise.all(
            links.slice(0, ANALYTICS_LINK_CAP).map(async (link) => {
              if (!link.requireEmailVerification) return null;
              return api.getLinkAnalytics(link.id).catch(() => null);
            }),
          ),
        ]);

        if (cancelled || generation !== generationRef.current) return;

        unreadQuestions = (roomQuestionsRes.data ?? []).filter((q) => q.status === "pending").length;

        for (const analyticsRes of analyticsResults) {
          const analytics = analyticsRes?.data ?? null;
          if (!analytics) continue;
          failedDeliveries +=
            analytics.access_code_failed_count ??
            (analytics.access_code_contacts ?? []).filter(
              (c: { send_status: string }) => c.send_status === "failed",
            ).length;
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
      clear();
    };
  }, [roomId, refreshKey, setSignals, clear]);
}

export async function fetchDealRoomLinks(roomId: string): Promise<Link[]> {
  const res = await api.getDealRoomLinks(roomId);
  return (res.data ?? []).filter(isLinkActive);
}
