import { useEffect, useState } from "react";
import { api } from "@/lib/api";

export type DocumentDeleteImpact = {
  activeLinkCount: number;
  dealRoomCount: number;
};

export async function loadDocumentDeleteImpact(
  documentId: string,
  fallbackLinkCount: number,
): Promise<DocumentDeleteImpact> {
  try {
    const impact = await api.getDocumentDeleteImpact(documentId);
    return {
      activeLinkCount: impact.active_link_count,
      dealRoomCount: impact.deal_room_count,
    };
  } catch {
    return {
      activeLinkCount: fallbackLinkCount,
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
