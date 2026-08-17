import { describe, expect, it } from "vitest";
import { displayablePageTitle, displayablePageTitles } from "./pageTitleDisplay";

describe("displayablePageTitle", () => {
  it("keeps deck headings", () => {
    expect(displayablePageTitle("Financial Projections")).toBe("Financial Projections");
    expect(displayablePageTitle("  团队与组织  ")).toBe("团队与组织");
    expect(displayablePageTitle('"Q2 Financials"')).toBe('"Q2 Financials"');
    expect(displayablePageTitle('KPI "ARR": growth')).toBe('KPI "ARR": growth');
  });

  it("hides JSON fragments used as PDF page titles", () => {
    expect(
      displayablePageTitle(
        'nk_ic": 0.012, "net_ir": -0.18}, "decision": "rejected", "reason": " 成本后无增量"',
      ),
    ).toBe("");
    expect(
      displayablePageTitle('"parameters": {"window": 5, "volume_window": 20}, "r...'),
    ).toBe("");
    expect(displayablePageTitle('{"window": 5}')).toBe("");
    expect(
      displayablePageTitle(
        '{"parameters": {"window": 5, "volume_window": 20}, "m...',
      ),
    ).toBe("");
  });

  it("hides empty titles", () => {
    expect(displayablePageTitle("")).toBe("");
    expect(displayablePageTitle("   ")).toBe("");
    expect(displayablePageTitle(undefined)).toBe("");
  });

  it("drops JSON dumps from a title list", () => {
    expect(
      displayablePageTitles([
        "Financials",
        '{"parameters": {"window": 5, "volume_window": 20}, "m...',
        "  Team  ",
      ]),
    ).toEqual(["Financials", "Team"]);
  });
});
