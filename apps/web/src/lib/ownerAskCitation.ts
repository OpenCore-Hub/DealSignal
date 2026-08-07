import { useCallback } from "react";
import { useNavigate, useParams } from "react-router";
import { viewerPath } from "@/lib/knowledge/citations";
import type { DealRoomKnowledgeQueryHit } from "@/types";

/** Navigate authenticated owner to document viewer at citation locus. */
export function useOwnerAskCitationNavigation(dealRoomId?: string) {
  const navigate = useNavigate();
  const { workspaceSlug } = useParams<{ workspaceSlug: string }>();

  return useCallback(
    (hit: DealRoomKnowledgeQueryHit) => {
      const documentId = hit.documentId?.trim();
      if (!documentId) return;
      const page = hit.viewerPage ?? hit.pages?.[0];
      navigate(
        viewerPath(documentId, page, {
          roomId: dealRoomId?.trim() || undefined,
          workspaceSlug: workspaceSlug?.trim() || undefined,
        }),
      );
    },
    [dealRoomId, navigate, workspaceSlug],
  );
}
