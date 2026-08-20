import { describe, expect, it } from "vitest";
import { canContributeToRoom, canManageAskHost, canManageRoom, canManageRoomMember, canMutateShareLink, canReviewShareAccessRequests, canViewRoomAccessPolicy, canViewRoomKnowledge, grantableRoomRoles } from "./dealRoomCapabilities";

describe("dealRoomCapabilities", () => {
  it("treats isAdmin as manage", () => {
    expect(canManageRoom({ isAdmin: true })).toBe(true);
    expect(canManageRoom({ isAdmin: false, oversight: true })).toBe(false);
    expect(canManageRoom(null)).toBe(false);
  });

  it("prefers canContribute over isAdmin fallback", () => {
    expect(canContributeToRoom({ isAdmin: false, canContribute: true })).toBe(true);
    expect(canContributeToRoom({ isAdmin: true, canContribute: false })).toBe(false);
    expect(canContributeToRoom({ isAdmin: true })).toBe(true);
    expect(canContributeToRoom({ oversight: true })).toBe(false);
  });

  it("allows knowledge view whenever a room payload exists", () => {
    expect(canViewRoomKnowledge({ oversight: true })).toBe(true);
    expect(canViewRoomKnowledge(null)).toBe(false);
    expect(canViewRoomKnowledge({ ndaRequired: true })).toBe(false);
  });

  it("lets oversight view access policy without manage", () => {
    expect(canViewRoomAccessPolicy({ isAdmin: true })).toBe(true);
    expect(canViewRoomAccessPolicy({ isAdmin: false, oversight: true })).toBe(true);
    expect(canViewRoomAccessPolicy({ isAdmin: false, canContribute: true })).toBe(false);
    expect(canViewRoomAccessPolicy(null)).toBe(false);
  });

  it("restricts grantable roles by actor room role", () => {
    expect(grantableRoomRoles("owner")).toEqual(["admin", "member", "guest"]);
    expect(grantableRoomRoles("admin")).toEqual(["member", "guest"]);
    expect(grantableRoomRoles("")).toEqual([]);
    expect(grantableRoomRoles(undefined)).toEqual([]);
  });

  it("blocks owner and peer-admin management for room admins", () => {
    expect(canManageRoomMember("owner", "admin")).toBe(true);
    expect(canManageRoomMember("admin", "member")).toBe(true);
    expect(canManageRoomMember("admin", "admin")).toBe(false);
    expect(canManageRoomMember("admin", "owner")).toBe(false);
    expect(canManageRoomMember(undefined, "guest")).toBe(false);
  });

  it("gates Ask Host by room manage vs workspace owner/admin", () => {
    expect(canManageAskHost({ dealRoomId: "r1", workspaceCanManage: true })).toBe(false);
    expect(canManageAskHost({ dealRoomId: "r1", roomCanManage: true })).toBe(true);
    expect(canManageAskHost({ workspaceCanManage: true })).toBe(true);
    expect(canManageAskHost({ workspaceCanManage: false })).toBe(false);
    expect(
      canManageAskHost({
        dealRoomId: "r1",
        workspaceCanManage: true,
        linkCanManageAsk: false,
      }),
    ).toBe(false);
    expect(
      canManageAskHost({
        dealRoomId: "r1",
        workspaceCanManage: false,
        linkCanManageAsk: true,
      }),
    ).toBe(true);
  });

  it("gates share-link mutate by room manage vs workspace write", () => {
    expect(canMutateShareLink({ dealRoomId: "r1", workspaceCanWrite: true })).toBe(false);
    expect(canMutateShareLink({ dealRoomId: "r1", roomCanManage: true })).toBe(true);
    expect(
      canMutateShareLink({
        dealRoomId: "r1",
        workspaceCanWrite: false,
        linkCanManageAsk: true,
      }),
    ).toBe(true);
    expect(canMutateShareLink({ workspaceCanWrite: true })).toBe(true);
    expect(canMutateShareLink({ workspaceCanWrite: false })).toBe(false);
  });

  it("gates access-request review by backend flag, not workspace write", () => {
    expect(canReviewShareAccessRequests({})).toBe(false);
    expect(canReviewShareAccessRequests({ linkCanReviewAccessRequests: true })).toBe(true);
    expect(canReviewShareAccessRequests({ linkCanReviewAccessRequests: false })).toBe(false);
    expect(canReviewShareAccessRequests({ dealRoomId: "r1", roomCanManage: true })).toBe(true);
    expect(canReviewShareAccessRequests({ dealRoomId: "r1", roomCanManage: false })).toBe(false);
    expect(
      canReviewShareAccessRequests({
        dealRoomId: "r1",
        roomCanManage: true,
        linkCanReviewAccessRequests: false,
      }),
    ).toBe(false);
  });
});
