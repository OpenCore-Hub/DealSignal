// @vitest-environment node
import { describe, expect, it } from "vitest";
import {
  accessEventPrimaryLabel,
  accessEventReasonLabel,
  accessEventSecondaryLabel,
  accessEventTypeLabel,
} from "./accessEventLabels";

const en: Record<string, string> = {
  "access.eventTypes.security_gate_failed": "Security gate failed",
  "access.eventTypes.invalid_password": "Invalid password",
  "access.eventTypes.not_in_allow_list": "Not on allow list",
  "access.reasons.email_code_required": "Email verification required",
  "access.reasons.nda_required": "NDA required",
};

const t = (key: string) => en[key] ?? key;

describe("accessEventLabels", () => {
  it("promotes gate reason for security_gate_failed", () => {
    expect(accessEventPrimaryLabel(t, "security_gate_failed", "email_code_required")).toBe(
      "Email verification required",
    );
    expect(accessEventSecondaryLabel(t, "security_gate_failed", "email_code_required")).toBe(
      "Security gate failed",
    );
  });

  it("keeps non-gate types as primary without secondary", () => {
    expect(accessEventPrimaryLabel(t, "invalid_password", "bad password")).toBe("Invalid password");
    expect(accessEventSecondaryLabel(t, "invalid_password", "bad password")).toBeNull();
  });

  it("falls back to humanized reason codes", () => {
    expect(accessEventReasonLabel(t, "custom_new_reason")).toBe("custom new reason");
  });

  it("labels known event types", () => {
    expect(accessEventTypeLabel(t, "not_in_allow_list")).toBe("Not on allow list");
  });
});
