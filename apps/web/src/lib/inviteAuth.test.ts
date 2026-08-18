import { describe, expect, it } from "vitest";
import { workspaceInviteTokenFromRedirect } from "./inviteAuth";

describe("workspaceInviteTokenFromRedirect", () => {
  it("extracts the workspace invite token from a safe accept redirect", () => {
    expect(workspaceInviteTokenFromRedirect("/invitations/tok-1/accept")).toBe("tok-1");
  });

  it("rejects open redirects and unrelated paths", () => {
    expect(workspaceInviteTokenFromRedirect("https://evil.example/invitations/tok-1/accept")).toBe("");
    expect(workspaceInviteTokenFromRedirect("/login")).toBe("");
    expect(workspaceInviteTokenFromRedirect("/invitations/../secrets/accept")).toBe("");
    expect(workspaceInviteTokenFromRedirect("/room-invitations/dsr1.abc.def/accept")).toBe("");
  });
});
