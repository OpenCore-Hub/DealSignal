import { describe, expect, it } from "vitest";
import {
  resolveAskPolicyFromExperience,
  resolveExperienceFromAskPolicy,
} from "./visitorAskExperience";

describe("visitorAskExperience", () => {
  it("maps unified experiences to ask policy fields", () => {
    expect(resolveAskPolicyFromExperience("host_only")).toEqual({
      askAiEnabled: false,
      askMode: "supervised",
    });
    expect(resolveAskPolicyFromExperience("ai_supervised")).toEqual({
      askAiEnabled: true,
      askMode: "supervised",
    });
    expect(resolveAskPolicyFromExperience("ai_self_serve")).toEqual({
      askAiEnabled: true,
      askMode: "self_serve",
    });
    expect(resolveAskPolicyFromExperience("formal")).toEqual({
      askAiEnabled: false,
      askMode: "formal",
    });
  });

  it("round-trips from stored policy fields", () => {
    expect(resolveExperienceFromAskPolicy(false, "supervised")).toBe("host_only");
    expect(resolveExperienceFromAskPolicy(true, "supervised")).toBe("ai_supervised");
    expect(resolveExperienceFromAskPolicy(true, "self_serve")).toBe("ai_self_serve");
    expect(resolveExperienceFromAskPolicy(true, "formal")).toBe("formal");
    expect(resolveExperienceFromAskPolicy(false, "formal")).toBe("formal");
  });
});
