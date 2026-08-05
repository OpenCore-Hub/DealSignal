// @vitest-environment jsdom
import { describe, expect, it, vi } from "vitest";
import { fireEvent, render, screen } from "@testing-library/react";
import { I18nextProvider } from "react-i18next";
import { createTestI18n } from "@/i18n/test-utils";
import { DocumentInsights } from "./DocumentInsights";
import type { PageAnalytics } from "@/types";

function pages(): PageAnalytics[] {
  return [
    { pageNumber: 1, viewCount: 2, avgDurationSeconds: 10, exitRate: 0.05 },
    { pageNumber: 2, viewCount: 4, avgDurationSeconds: 55, exitRate: 0.08 },
    { pageNumber: 3, viewCount: 3, avgDurationSeconds: 12, exitRate: 0.42 },
  ];
}

describe("DocumentInsights", () => {
  it("renders actionable insights and opens pages when clicked", async () => {
    const onOpenPage = vi.fn();
    const i18n = await createTestI18n({
      documents: {
        "insights.title": "Insights",
        "insights.subtitle": "From reading behavior",
        "insights.emptyTitle": "Empty",
        "insights.emptyDescription": "No data",
        "insights.topPage": "Focus on page {{pageNumber}}",
        "insights.topPageDescription": "{{seconds}}s average",
        "insights.exitRisk": "Exit risk on page {{pageNumber}}",
        "insights.exitRiskDescription": "{{percent}}% exit",
        "insights.sparse": "{{engaged}} of {{total}} pages engaged",
        "insights.sparseDescription": "{{count}} pages cold",
      },
    });

    render(
      <I18nextProvider i18n={i18n}>
        <DocumentInsights analytics={pages()} onOpenPage={onOpenPage} />
      </I18nextProvider>,
    );

    expect(screen.getByTestId("document-insights")).toHaveAttribute("data-count");
    fireEvent.click(screen.getByTestId("document-insight-top_dwell-2"));
    expect(onOpenPage).toHaveBeenCalledWith(2);

    fireEvent.click(screen.getByTestId("document-insight-exit_risk-3"));
    expect(onOpenPage).toHaveBeenCalledWith(3);
  });
});
