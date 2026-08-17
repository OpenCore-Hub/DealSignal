import { useEffect, useState } from "react";
import { api } from "@/lib/api";

export type DocumentDeleteImpact = {
  activeLinkCount: number;
  revokedLinkCount: number;
  dealRoomCount: number;
};

export async function loadDocumentDeleteImpact(
  documentId: string,
  fallbackLinkCount: number,
): Promise<DocumentDeleteImpact> {
  try {
    const impact = await api.getDocumentDeleteImpact(documentId);
    const revoked =
      typeof impact.revoked_link_count === "number"
        ? impact.revoked_link_count
        : impact.active_link_count;
    return {
      activeLinkCount: impact.active_link_count,
      revokedLinkCount: revoked,
      dealRoomCount: impact.deal_room_count,
    };
  } catch {
    return {
      activeLinkCount: fallbackLinkCount,
      revokedLinkCount: fallbackLinkCount,
      dealRoomCount: 0,
    };
  }
}

/** Loads delete/archive impact for a confirm dialog; null while idle or loading. */
export function useDocumentDeleteImpact(
  doc: { id: string; links: { length: number } } | null,
): DocumentDeleteImpact | null {
  const [impact, setImpact] = useState<DocumentDeleteImpact | null>(null);

  useEffect(() => {
    if (!doc) {
      setImpact(null);
      return;
    }
    let cancelled = false;
    setImpact(null);
    void loadDocumentDeleteImpact(doc.id, doc.links.length).then((next) => {
      if (!cancelled) setImpact(next);
    });
    return () => {
      cancelled = true;
    };
  }, [doc]);

  return impact;
}
