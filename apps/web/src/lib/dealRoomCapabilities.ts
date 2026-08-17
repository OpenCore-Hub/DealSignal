import type { DealRoom, DealRoomMemberRole } from "@/types";

type RoomCaps = Pick<DealRoom, "isAdmin" | "canContribute" | "oversight" | "roomRole" | "ndaRequired">;

const GRANTABLE_BY_OWNER: DealRoomMemberRole[] = ["admin", "member", "guest"];
const GRANTABLE_BY_ADMIN: DealRoomMemberRole[] = ["member", "guest"];

/** Room owner/admin — invite, folders, links, access policy, delete. */
export function canManageRoom(room?: RoomCaps | null): boolean {
  return room?.isAdmin === true;
}

/** Room owner/admin/member — documents and knowledge write. */
export function canContributeToRoom(room?: RoomCaps | null): boolean {
  if (!room) return false;
  if (typeof room.canContribute === "boolean") return room.canContribute;
  return room.isAdmin === true;
}

/** Anyone who can open the room, including oversight and room guests. */
export function canViewRoomKnowledge(room?: RoomCaps | null): boolean {
  if (!room || room.ndaRequired) return false;
  return true;
}

/** Room owner/admin, or workspace oversight (view-only). */
export function canViewRoomAccessPolicy(room?: RoomCaps | null): boolean {
  return canManageRoom(room) || room?.oversight === true;
}

/** Roles the actor may invite or assign. Empty when the actor cannot manage. */
export function grantableRoomRoles(actorRoomRole?: string | null): DealRoomMemberRole[] {
  if (actorRoomRole === "owner") return GRANTABLE_BY_OWNER;
  if (actorRoomRole === "admin") return GRANTABLE_BY_ADMIN;
  return [];
}

/** Whether the actor may change or remove a member with targetRole. */
export function canManageRoomMember(
  actorRoomRole: string | undefined,
  targetRole: string,
): boolean {
  if (targetRole === "owner") return false;
  if (actorRoomRole === "owner") return true;
  return actorRoomRole === "admin" && (targetRole === "member" || targetRole === "guest");
}

/**
 * Host reply / Formal publish / pin FAQ.
 * Deal-room links require room manage; document links require workspace owner/admin.
 */
export function canManageAskHost(input: {
  dealRoomId?: string | null;
  roomCanManage?: boolean;
  workspaceCanManage?: boolean;
  linkCanManageAsk?: boolean;
}): boolean {
  if (typeof input.linkCanManageAsk === "boolean") return input.linkCanManageAsk;
  if (input.dealRoomId) return input.roomCanManage === true;
  return input.workspaceCanManage === true;
}

/**
 * Edit / enable / archive a share link.
 * Deal-room links follow room manage (same as Ask Host); document links keep workspace write.
 */
export function canMutateShareLink(input: {
  dealRoomId?: string | null;
  roomCanManage?: boolean;
  workspaceCanWrite?: boolean;
  linkCanManageAsk?: boolean;
}): boolean {
  if (input.dealRoomId) {
    return canManageAskHost({
      dealRoomId: input.dealRoomId,
      roomCanManage: input.roomCanManage,
      linkCanManageAsk: input.linkCanManageAsk,
    });
  }
  return input.workspaceCanWrite === true;
}
