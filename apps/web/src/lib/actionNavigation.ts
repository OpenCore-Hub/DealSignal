import { dealRoomAccessPath } from "@/lib/dealRoomAccessPath";
import { dealRoomAskPath, parseDealRoomAskTarget } from "@/lib/dealRoomAskPath";
import { documentsSharePath } from "@/lib/documentsSharePath";
import type { ActionItem } from "@/types";

type NavAction = Pick<ActionItem, "sourceType" | "sourceId" | "targetId">;

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
    case "room_nda":
      return dealRoomAccessPath(workspaceSlug, action.sourceId);

    case "expiring_room":
      return `/${workspaceSlug}/deal-rooms/${action.sourceId}`;

    case "deal_room_link_question": {
      if (!action.targetId) return null;
      const { roomId, linkId } = parseDealRoomAskTarget(action.targetId);
      return dealRoomAskPath(workspaceSlug, roomId, linkId ? { linkId } : undefined);
    }

    case "link_question":
      // Legacy rows pointed at /links/{questionId}; require targetId or refuse.
      return null;

    case "uploaded_file":
    case "expiring_link":
      return `/${workspaceSlug}/links/${action.sourceId}`;

    default:
      return null;
  }
}
