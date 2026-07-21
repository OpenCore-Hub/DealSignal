import { describe, it, expect } from "vitest";
import {
  findMissingRecommendedFiles,
  matchesRecommendedFile,
  resolveFolderForRecommended,
} from "./dealRoomReadiness";

describe("dealRoomReadiness", () => {
  it("matches recommended labels loosely against titles", () => {
    expect(matchesRecommendedFile("Acme Seed Round Pitch Deck", "Pitch deck")).toBe(true);
    expect(matchesRecommendedFile("Q2 Report", "Pitch deck")).toBe(false);
  });

  it("lists only missing recommended files", () => {
    expect(
      findMissingRecommendedFiles(
        ["Pitch deck", "Financial model", "Cap table"],
        ["Acme Seed Round Pitch Deck"]
      )
    ).toEqual(["Financial model", "Cap table"]);
  });

  it("resolves folder path for a recommended label", () => {
    expect(
      resolveFolderForRecommended("Pitch deck", [
        { path: "/financials", name: "02 Financials" },
        { path: "/pitch", name: "01 Pitch Deck" },
      ])
    ).toBe("/pitch");
  });
});
