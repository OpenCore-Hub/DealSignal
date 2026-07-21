import { describe, it, expect } from "vitest";
import { buildAllowedLists, buildDraft, buildLinkPayload, buildRules, toDateTimeLocal, toRFC3339, validateDraft, isValidCustomDomain } from "./utils";
import type { DraftLink } from "./types";
import type { AccessRule, Link } from "@/types";

const baseDraft: DraftLink = {
  name: "Test Link",
  expiresAt: "",
  requireEmail: false,
  requireEmailVerification: false,
  requirePassword: false,
  password: "",
  watermarkEnabled: false,
  requireNda: false,
  ndaDocumentId: "",
  ndaTemplateId: "",
  allowDownloading: false,
  enableScreenshotProtection: false,
  aiCopilotEnabled: false,
  enableFileRequests: false,
  enableIndexFileGeneration: false,
  enableQaConversations: false,
  allowedViewers: [],
  blockedViewers: [],
  customDomain: "",
  notifyOnAccess: false,
  folderPaths: [],
  folderScopeMode: "allowlist",
  contactIds: [],
};

const t = (key: string) => key;

describe("toRFC3339", () => {
  it("converts datetime-local to RFC3339 with local timezone offset", () => {
    const result = toRFC3339("2026-08-17T08:41");
    expect(result).toMatch(/^2026-08-17T08:41:00[+-]\d{2}:\d{2}$/);
  });

  it("returns empty string for empty input", () => {
    expect(toRFC3339("")).toBe("");
  });

  it("returns empty string for invalid input", () => {
    expect(toRFC3339("not-a-date")).toBe("");
    expect(toRFC3339("2026-08-17")).toBe("");
  });
});

describe("toDateTimeLocal", () => {
  it("round-trips an ISO string through datetime-local", () => {
    const iso = "2026-08-17T08:41:00+08:00";
    const local = toDateTimeLocal(iso);
    expect(local).toMatch(/^2026-08-17T08:41$/);
  });

  it("returns empty string for empty input", () => {
    expect(toDateTimeLocal("")).toBe("");
  });
});

describe("buildLinkPayload", () => {
  it("formats expires_at as RFC3339 when set", () => {
    const draft = { ...baseDraft, expiresAt: "2026-08-17T08:41" };
    const payload = buildLinkPayload(draft);
    expect(payload.expires_at).toMatch(/^2026-08-17T08:41:00[+-]\d{2}:\d{2}$/);
  });

  it("omits expires_at when not set", () => {
    const draft = { ...baseDraft, expiresAt: "" };
    const payload = buildLinkPayload(draft);
    expect(payload.expires_at).toBeUndefined();
  });

  it("includes contact_ids for document links when verification is enabled", () => {
    const draft: DraftLink = {
      ...baseDraft,
      requireEmailVerification: true,
      contactIds: ["contact-1"],
    };
    const existingLink = {
      id: "link-1",
      documentIds: ["doc-1"],
      dealRoomId: undefined,
    } as unknown as Link;
    const payload = buildLinkPayload(draft, existingLink);
    expect(payload.contact_ids).toEqual(["contact-1"]);
  });

  it("omits contact_ids for deal-room links", () => {
    const draft: DraftLink = {
      ...baseDraft,
      requireEmailVerification: true,
      contactIds: ["contact-1"],
    };
    const existingLink = {
      id: "link-1",
      documentIds: [],
      dealRoomId: "room-1",
    } as unknown as Link;
    const payload = buildLinkPayload(draft, existingLink);
    expect(payload.contact_ids).toBeUndefined();
  });

  it("omits password when requirePassword is false", () => {
    const draft: DraftLink = {
      ...baseDraft,
      requirePassword: false,
      password: "secret123",
    };
    const payload = buildLinkPayload(draft);
    expect(payload.password).toBeUndefined();
  });

  it("includes custom_domain when set", () => {
    const draft: DraftLink = {
      ...baseDraft,
      customDomain: "investors.example.com",
    };
    const payload = buildLinkPayload(draft);
    expect(payload.custom_domain).toBe("investors.example.com");
  });

  it("uses permission_type nda when requireNda is true", () => {
    const draft: DraftLink = {
      ...baseDraft,
      requireNda: true,
      requireEmail: true,
      requireEmailVerification: false,
    };
    const payload = buildLinkPayload(draft);
    expect(payload.permission_type).toBe("nda");
  });

  it("sends allowlist folder_paths including empty deny-all", () => {
    const draft: DraftLink = {
      ...baseDraft,
      folderScopeMode: "allowlist",
      folderPaths: [],
    };
    const payload = buildLinkPayload(draft);
    expect(payload.folder_scope_mode).toBe("allowlist");
    expect(payload.folder_paths).toEqual([]);
  });

  it("omits folder_paths when preserving legacy full mode", () => {
    const draft: DraftLink = {
      ...baseDraft,
      folderScopeMode: "full",
      folderPaths: [],
    };
    const payload = buildLinkPayload(draft);
    expect(payload.folder_scope_mode).toBe("full");
    expect(payload.folder_paths).toBeUndefined();
  });
});

describe("buildAllowedLists", () => {
  it("converts allowed/blocked viewers to email lists", () => {
    const draft: DraftLink = {
      ...baseDraft,
      allowedViewers: ["alice@vc.com", "bob@vc.com"],
      blockedViewers: ["leaker@bad.com"],
    };
    const { allowedEmails, blockedEmails } = buildAllowedLists(draft);
    expect(allowedEmails).toEqual(["alice@vc.com", "bob@vc.com"]);
    expect(blockedEmails).toEqual(["leaker@bad.com"]);
  });

  it("filters out empty strings", () => {
    const draft: DraftLink = {
      ...baseDraft,
      allowedViewers: ["alice@vc.com", "", "bob@vc.com"],
      blockedViewers: ["", "leaker@bad.com"],
    };
    const { allowedEmails, blockedEmails } = buildAllowedLists(draft);
    expect(allowedEmails).toEqual(["alice@vc.com", "bob@vc.com"]);
    expect(blockedEmails).toEqual(["leaker@bad.com"]);
  });
});

describe("buildRules", () => {
  it("creates allow and block access rules", () => {
    const draft: DraftLink = {
      ...baseDraft,
      allowedViewers: ["alice@vc.com"],
      blockedViewers: ["leaker@bad.com"],
    };
    const rules = buildRules(draft);
    expect(rules).toEqual([
      { ruleType: "email", value: "alice@vc.com", action: "allow" },
      { ruleType: "email", value: "leaker@bad.com", action: "block" },
    ]);
  });
});

describe("buildDraft", () => {
  it("populates allowed and blocked viewers from lowercase access rules", () => {
    const rules: AccessRule[] = [
      { ruleType: "email", value: "alice@vc.com", action: "allow" },
      { ruleType: "email", value: "leaker@bad.com", action: "block" },
    ];
    const draft = buildDraft(null, rules);
    expect(draft.allowedViewers).toEqual(["alice@vc.com"]);
    expect(draft.blockedViewers).toEqual(["leaker@bad.com"]);
  });

  it("populates allowed and blocked viewers from PascalCase access rules", () => {
    const rules = [
      { RuleType: "email", Value: "alice@vc.com", Action: "allow" },
      { RuleType: "email", Value: "leaker@bad.com", Action: "block" },
    ] as unknown as AccessRule[];
    const draft = buildDraft(null, rules);
    expect(draft.allowedViewers).toEqual(["alice@vc.com"]);
    expect(draft.blockedViewers).toEqual(["leaker@bad.com"]);
  });

  it("skips rules with missing or non-string values", () => {
    const rules = [
      { ruleType: "email", value: "alice@vc.com", action: "allow" },
      { ruleType: "email", value: null, action: "block" },
      { ruleType: "email", action: "allow" },
    ] as unknown as AccessRule[];
    const draft = buildDraft(null, rules);
    expect(draft.allowedViewers).toEqual(["alice@vc.com"]);
    expect(draft.blockedViewers).toEqual([]);
  });
});

describe("isValidCustomDomain", () => {
  it("accepts a valid subdomain domain", () => {
    expect(isValidCustomDomain("investors.example.com")).toBe(true);
  });

  it("accepts a valid apex domain", () => {
    expect(isValidCustomDomain("example.com")).toBe(true);
  });

  it("rejects an empty string", () => {
    expect(isValidCustomDomain("")).toBe(false);
  });

  it("rejects a domain with protocol", () => {
    expect(isValidCustomDomain("https://investors.example.com")).toBe(false);
  });

  it("rejects a domain with trailing path", () => {
    expect(isValidCustomDomain("investors.example.com/path")).toBe(false);
  });

  it("rejects a domain starting with a hyphen", () => {
    expect(isValidCustomDomain("-investors.example.com")).toBe(false);
  });
});

describe("validateDraft", () => {
  it("rejects expires_at in the past", () => {
    const draft: DraftLink = {
      ...baseDraft,
      expiresAt: "2020-01-01T00:00",
    };
    const now = Date.now();
    const errors = validateDraft(draft, null, t, now);
    expect(errors.expiresAt).toBe("share.expiresAtFuture");
  });

  it("accepts expires_at in the future", () => {
    const future = new Date(Date.now() + 24 * 60 * 60 * 1000);
    const year = future.getFullYear();
    const month = String(future.getMonth() + 1).padStart(2, "0");
    const day = String(future.getDate()).padStart(2, "0");
    const hours = String(future.getHours()).padStart(2, "0");
    const minutes = String(future.getMinutes()).padStart(2, "0");
    const draft: DraftLink = {
      ...baseDraft,
      expiresAt: `${year}-${month}-${day}T${hours}:${minutes}`,
    };
    const errors = validateDraft(draft, null, t, Date.now());
    expect(errors.expiresAt).toBeUndefined();
  });

  it("errors when document link requires verification without contacts", () => {
    const draft: DraftLink = {
      ...baseDraft,
      requireEmailVerification: true,
    };
    const link = {
      id: "link-1",
      dealRoomId: undefined,
    } as unknown as Link;
    const errors = validateDraft(draft, link, t, Date.now());
    expect(errors.requireVerificationContacts).toBe("accessRules.errors.requireVerificationContacts");
  });

  it("allows deal-room link with verification but no contacts", () => {
    const draft: DraftLink = {
      ...baseDraft,
      requireEmailVerification: true,
    };
    const link = {
      id: "link-1",
      dealRoomId: "room-1",
    } as unknown as Link;
    const errors = validateDraft(draft, link, t, Date.now());
    expect(errors.requireVerificationContacts).toBeUndefined();
  });

  it("errors when allowed viewers exist but no email gate is enabled", () => {
    const draft: DraftLink = {
      ...baseDraft,
      allowedViewers: ["alice@vc.com"],
      requireEmail: false,
      requireEmailVerification: false,
    };
    const errors = validateDraft(draft, null, t, Date.now());
    expect(errors.allowedViewers).toBe("accessRules.errors.allowRequiresEmail");
  });

  it("errors when email collection is enabled but no allowed viewers are set", () => {
    const draft: DraftLink = {
      ...baseDraft,
      allowedViewers: [],
      requireEmail: true,
    };
    const errors = validateDraft(draft, null, t, Date.now());
    expect(errors.allowedViewers).toBe("accessRules.errors.allowedViewersRequired");
  });

  it("errors when email verification is enabled but no allowed viewers are set", () => {
    const draft: DraftLink = {
      ...baseDraft,
      allowedViewers: [],
      requireEmailVerification: true,
    };
    const errors = validateDraft(draft, null, t, Date.now());
    expect(errors.allowedViewers).toBe("accessRules.errors.allowedViewersRequired");
  });

  it("allows allowed viewers when email collection is enabled", () => {
    const draft: DraftLink = {
      ...baseDraft,
      allowedViewers: ["alice@vc.com"],
      requireEmail: true,
    };
    const errors = validateDraft(draft, null, t, Date.now());
    expect(errors.allowedViewers).toBeUndefined();
  });

  it("allows blocked viewers without an email gate", () => {
    const draft: DraftLink = {
      ...baseDraft,
      blockedViewers: ["leaker@bad.com"],
      requireEmail: false,
      requireEmailVerification: false,
    };
    const errors = validateDraft(draft, null, t, Date.now());
    expect(errors.blockedViewers).toBeUndefined();
    expect(errors.allowedViewers).toBeUndefined();
  });

  it("errors when password is shorter than 8 characters", () => {
    const draft: DraftLink = {
      ...baseDraft,
      requirePassword: true,
      password: "short",
    };
    const errors = validateDraft(draft, null, t, Date.now());
    expect(errors.password).toBe("accessRules.errors.passwordMinLength");
  });

  it("allows password of exactly 8 characters", () => {
    const draft: DraftLink = {
      ...baseDraft,
      requirePassword: true,
      password: "abcdefgh",
    };
    const errors = validateDraft(draft, null, t, Date.now());
    expect(errors.password).toBeUndefined();
  });

  it("errors when custom domain format is invalid", () => {
    const draft: DraftLink = {
      ...baseDraft,
      customDomain: "https://investors.example.com",
    };
    const errors = validateDraft(draft, null, t, Date.now());
    expect(errors.customDomain).toBe("share.customDomainInvalid");
  });

  it("allows a valid custom domain", () => {
    const draft: DraftLink = {
      ...baseDraft,
      customDomain: "investors.example.com",
    };
    const errors = validateDraft(draft, null, t, Date.now());
    expect(errors.customDomain).toBeUndefined();
  });

  it("errors when the same email is in allowed and blocked viewers", () => {
    const draft: DraftLink = {
      ...baseDraft,
      allowedViewers: ["alice@vc.com"],
      blockedViewers: ["alice@vc.com"],
    };
    const errors = validateDraft(draft, null, t, Date.now());
    expect(errors.conflict).toBe("accessRules.errors.conflict");
  });

  it("errors when link name already exists (case-insensitive)", () => {
    const draft: DraftLink = {
      ...baseDraft,
      name: "Acme DD",
    };
    const errors = validateDraft(draft, null, t, Date.now(), true, ["acme dd", "Other"]);
    expect(errors.name).toBe("share.linkNameDuplicate");
  });

  it("allows renaming to the same name when existing names exclude self", () => {
    const draft: DraftLink = {
      ...baseDraft,
      name: "Acme DD",
    };
    const errors = validateDraft(draft, null, t, Date.now(), true, ["Other"]);
    expect(errors.name).toBeUndefined();
  });
});
