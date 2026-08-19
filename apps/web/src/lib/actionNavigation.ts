import { dealRoomAccessPath } from "@/lib/dealRoomAccessPath";
import { dealRoomAskPath, parseDealRoomAskTarget } from "@/lib/dealRoomAskPath";
import { documentsSharePath } from "@/lib/documentsSharePath";
import type { ActionItem } from "@/types";

type NavAction = Pick<ActionItem, "sourceType" | "sourceId" | "targetId" | "actionType">;

/** Formal Q&A dashboard todos use actionType review (deal-room or document-library Ask). */
export function isFormalAskReviewAction(action: Pick<ActionItem, "sourceType" | "actionType">): boolean {
  return (
    (action.sourceType === "deal_room_link_question" || action.sourceType === "link_question") &&
    action.actionType === "review"
  );
}

/** Document-library Ask inbox deep link on the link detail page. */
export function libraryAskPath(
  workspaceSlug: string,
  linkId: string,
  opts?: { formalQueue?: boolean },
): string {
  const params = new URLSearchParams();
  if (opts?.formalQueue) {
    params.set("askInbox", "formal_queue");
  } else {
    params.set("askInbox", "needs_host");
  }
  const qs = params.toString();
  return `/${workspaceSlug}/links/${linkId}${qs ? `?${qs}` : ""}`;
}

/** Insights Formal Ask CTA: deal-room Ask inbox vs document-library link Ask. */
export function formalAskSuggestionPath(
  workspaceSlug: string,
  suggestion: { linkId: string; dealRoomId?: string },
): string | null {
  const linkId = suggestion.linkId?.trim();
  if (!workspaceSlug || !linkId) return null;
  const dealRoomId = suggestion.dealRoomId?.trim();
  if (dealRoomId) {
    return dealRoomAskPath(workspaceSlug, dealRoomId, { linkId, formalQueue: true });
  }
  return libraryAskPath(workspaceSlug, linkId, { formalQueue: true });
}

/**
 * Resolve dashboard operational-action deep links.
 *
 * Document Library and Deal Room share surfaces must never cross:
 * - link_access_request → Document Library → Share
 * - deal_room_link_access_request → Deal Room → Access (targetId = room)
 * - deal_room_link_question → Deal Room → Ask Inbox / QA (targetId = room or room/link)
 * - room_* → Deal Room (sourceId = room)
 */
export function actionNavigatePath(
  workspaceSlug: string,
  action: NavAction,
): string | null {
  if (!workspaceSlug || !action.sourceType || !action.sourceId) {
    return null;
  }

  switch (action.sourceType) {
    case "link_access_request":
      return documentsSharePath(workspaceSlug, { linkId: action.sourceId });

    case "deal_room_link_access_request":
      if (!action.targetId) return null;
      return dealRoomAccessPath(workspaceSlug, action.targetId, {
        linkId: action.sourceId,
      });

    case "room_access_request":
      return dealRoomAccessPath(workspaceSlug, action.sourceId);

    case "room_nda": {
      // Member-keyed: targetId = room. Legacy: sourceId = room.
      const roomId = action.targetId || action.sourceId;
      return dealRoomAccessPath(workspaceSlug, roomId);
    }

    case "expiring_room":
      // No host editor writes deal_rooms.expires_at. Null so ACT cannot
      // open the documents tab as a fake renew.
      return null;

    case "deal_room_link_question": {
      if (!action.targetId) return null;
      const { roomId, linkId } = parseDealRoomAskTarget(action.targetId);
      return dealRoomAskPath(workspaceSlug, roomId, {
        linkId: linkId ? linkId : undefined,
        formalQueue: isFormalAskReviewAction(action),
      });
    }

    case "link_question":
      // sourceId = turn id; targetId = link id (document-library Ask).
      if (!action.targetId) return null;
      return libraryAskPath(workspaceSlug, action.targetId, {
        formalQueue: isFormalAskReviewAction(action),
      });

    case "uploaded_file":
      return `/${workspaceSlug}/links/${action.sourceId}`;

    case "expiring_link":
      return expiringLinkPath(workspaceSlug, action.sourceId);

    default:
      return null;
  }
}

/**
 * Operational link renew. Library → existing expiry editor. Deal-room
 * shares stay on link detail (bundle pipeline must not edit them).
 * Mirrors apps/api/internal/radar/paths.go expiringLinkPath.
 */
export function expiringLinkPath(
  workspaceSlug: string,
  linkId: string,
  opts?: { dealRoomId?: string },
): string | null {
  const slug = workspaceSlug.trim();
  const id = linkId.trim();
  if (!slug || !id) return null;
  if (opts?.dealRoomId?.trim()) {
    return `/${slug}/links/${id}`;
  }
  return `/${slug}/links/${id}/edit?focus=expiry`;
}

/**
 * Host destination for Diligence gate holds that have no operational
 * sourceType (blocked_attempt). Rooms → Access. Library → share link
 * detail, not the Share request inbox. Operational approve still uses
 * documentsSharePath via actionNavigatePath. Mirrors
 * apps/api/internal/radar/paths.go diligenceRemediationPath.
 */
export function diligenceRemediationPath(
  workspaceSlug: string,
  opts?: { dealRoomId?: string; linkId?: string },
): string | null {
  const slug = workspaceSlug.trim();
  const roomId = opts?.dealRoomId?.trim();
  const linkId = opts?.linkId?.trim();
  if (!slug) return null;
  if (roomId) {
    return dealRoomAccessPath(slug, roomId, linkId ? { linkId } : undefined);
  }
  if (linkId) {
    return `/${slug}/links/${linkId}`;
  }
  return null;
}
