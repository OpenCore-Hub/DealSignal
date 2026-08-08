// @vitest-environment jsdom
import { describe, it, expect } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import { I18nextProvider, initReactI18next } from "react-i18next";
import i18n from "i18next";
import { TrendChart } from "./TrendChart";

async function renderChart(props: React.ComponentProps<typeof TrendChart>) {
  const instance = i18n.createInstance();
  await instance.use(initReactI18next).init({
    lng: "en",
    resources: {
      en: {
        common: {
          trendEmptyTitle: "No trend",
          empty: { description: "Empty" },
          start: "Start",
          now: "Now",
        },
      },
    },
  });
  return render(
    <I18nextProvider i18n={instance}>
      <TrendChart {...props} />
    </I18nextProvider>,
  );
}

describe("TrendChart", () => {
  it("renders dual series legend and tooltip", async () => {
    const { container } = await renderChart({
      title: "Trend",
      data: [1, 4],
      secondaryData: [1, 2],
      labels: ["Aug 1", "Aug 2"],
      primaryLegend: "Opens",
      secondaryLegend: "Visitors",
      formatValue: (v) => `${v} opens`,
      formatSecondaryValue: (v) => `${v} visitors`,
    });

    expect(screen.getByText("Opens")).toBeInTheDocument();
    expect(screen.getByText("Visitors")).toBeInTheDocument();
    const bars = container.querySelectorAll(".group");
    expect(bars.length).toBe(2);
    fireEvent.mouseEnter(bars[1]!);
    expect(screen.getByText("4 opens")).toBeInTheDocument();
    expect(screen.getByText("2 visitors")).toBeInTheDocument();
    // Axis tick + tooltip header both show the day label.
    expect(screen.getAllByText("Aug 2").length).toBeGreaterThanOrEqual(1);
  });

  it("aligns each day label under its column and paints active bars from the baseline", async () => {
    const { container } = await renderChart({
      title: "Trend",
      data: [0, 0, 1],
      secondaryData: [0, 0, 1],
      labels: ["Aug 1", "Aug 2", "Aug 3"],
    });

    // Per-column axis labels (not only first/last outside the plot).
    expect(screen.getByText("Aug 1")).toBeInTheDocument();
    expect(screen.getByText("Aug 2")).toBeInTheDocument();
    expect(screen.getByText("Aug 3")).toBeInTheDocument();

    const columns = container.querySelectorAll(".group");
    expect(columns.length).toBe(3);
    const lastPrimary = columns[2]!.querySelector(
      '[aria-hidden="true"][style*="height"]',
    ) as HTMLElement | null;
    expect(lastPrimary).toBeTruthy();
    expect(lastPrimary!.style.height).toBe("100%");
    expect(container.querySelector(".from-slate-50")).toBeTruthy();
  });
});
