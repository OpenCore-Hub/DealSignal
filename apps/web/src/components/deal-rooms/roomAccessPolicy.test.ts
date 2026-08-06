import { describe, expect, it } from "vitest";
import {
  buildLinkScopedRules,
  clampDraftToRoomBlocklist,
  clampDraftToRoomSecurityFloors,
  draftFromRoomAccessPolicy,
  hydrateCreateDraftFromRoomPolicy,
  roomAccessPolicyPayloadFromDraft,
  roomSecurityFloors,
} from "./roomAccessPolicy";
import { buildDraft } from "@/components/links/share";

const policy = {
  dealRoomId: "room-1",
  configured: true,
  requireEmailVerificationFloor: true,
  requireNdaFloor: true,
  requireEmailVerification: true,
  requireNda: true,
  allowedEmails: ["should-not-appear@example.com"],
  blockedEmails: ["bad@example.com"],
};

describe("roomAccessPolicy", () => {
  it("resolves floors from thin keys", () => {
    expect(roomSecurityFloors(policy)).toEqual({
      requireEmailVerification: true,
      requireNda: true,
    });
  });

  it("hydrates room-security draft without allowlist or protections", () => {
    const draft = draftFromRoomAccessPolicy(policy);
    expect(draft.requireEmailVerification).toBe(true);
    expect(draft.requireNda).toBe(true);
    expect(draft.watermarkEnabled).toBe(false);
    expect(draft.allowedViewers).toEqual([]);
    expect(draft.blockedViewers).toEqual(["bad@example.com"]);
  });

  it("create hydration forces floors and never copies room allowlist", () => {
    const draft = hydrateCreateDraftFromRoomPolicy(policy);
    expect(draft.allowedViewers).toEqual([]);
    expect(draft.blockedViewers).toEqual(["bad@example.com"]);
    expect(draft.requireEmailVerification).toBe(true);
    expect(draft.requireNda).toBe(true);
    expect(draft.folderScopeMode).toBe("full");
  });

  it("clamp refuses turning floors off before save", () => {
    const draft = buildDraft(null, []);
    draft.requireEmailVerification = false;
    draft.requireNda = false;
    draft.requireEmail = true;
    const clamped = clampDraftToRoomSecurityFloors(draft, policy);
    expect(clamped.requireEmailVerification).toBe(true);
    expect(clamped.requireNda).toBe(true);
    expect(clamped.requireEmail).toBe(false);
  });

  it("builds link-scoped rules without persisting room blocks", () => {
    const draft = buildDraft(null, []);
    draft.allowedViewers = ["ok@example.com"];
    draft.blockedViewers = ["bad@example.com", "link-only@example.com"];
    const rules = buildLinkScopedRules(draft, policy);
    expect(rules).toEqual([
      { ruleType: "email", value: "ok@example.com", action: "allow" },
      { ruleType: "email", value: "link-only@example.com", action: "block" },
    ]);
  });

  it("clamp keeps room blocklist and strips blocked emails from allow list", () => {
    const draft = buildDraft(null, []);
    draft.allowedViewers = ["bad@example.com", "ok@example.com"];
    draft.blockedViewers = [];
    const clamped = clampDraftToRoomBlocklist(draft, policy);
    expect(clamped.blockedViewers).toEqual(["bad@example.com"]);
    expect(clamped.allowedViewers).toEqual(["ok@example.com"]);
  });

  it("builds thin upsert payload", () => {
    const draft = draftFromRoomAccessPolicy(policy);
    draft.allowedViewers = ["ignored@example.com"];
    const payload = roomAccessPolicyPayloadFromDraft(draft);
    expect(payload).toEqual({
      require_email_verification_floor: true,
      require_nda_floor: true,
      require_email_verification: true,
      require_nda: true,
      blocked_emails: ["bad@example.com"],
    });
  });
});
