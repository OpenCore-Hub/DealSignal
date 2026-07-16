// @vitest-environment jsdom
import { describe, it, expect } from "vitest";
import { render, screen } from "@testing-library/react";
import { I18nextProvider } from "react-i18next";
import { MemoryRouter } from "react-router";
import { RecentActivityFeed } from "./RecentActivityFeed";
import { createTestI18n } from "@/i18n/test-utils";
import type { RecentActivityItem } from "@/lib/api";

function makeActivity(eventType: RecentActivityItem["eventType"], objectType: RecentActivityItem["objectType"]): RecentActivityItem {
  return {
    id: `act-${eventType}`,
    eventType,
    actor: "alice@example.test",
    objectType,
    objectName: "Financial Model",
    objectId: "doc-1",
    createdAt: new Date().toISOString(),
  };
}

async function renderFeed(activities: RecentActivityItem[]) {
  const i18n = await createTestI18n({
    dashboard: {
      "sections.activityFeed": "Recent activity",
      "activity.events.visit": "visited",
      "activity.events.download": "downloaded",
      "activity.events.question": "asked about",
      "activity.events.upload": "uploaded",
      "activity.anonymousUser": "Anonymous user",
      "empty.activity.title": "No activity",
      "empty.activity.description": "Recent activity will appear here.",
    },
    common: {
      back: "Back",
    },
  });
  render(
    <MemoryRouter>
      <I18nextProvider i18n={i18n}>
        <RecentActivityFeed activities={activities} workspaceSlug="acme" />
      </I18nextProvider>
    </MemoryRouter>
  );
}

describe("RecentActivityFeed", () => {
  it("renders empty state", async () => {
    await renderFeed([]);
    expect(screen.getByRole("heading", { name: "No activity" })).toBeInTheDocument();
    expect(screen.getByText("Recent activity will appear here.")).toBeInTheDocument();
  });

  it("renders activity items", async () => {
    await renderFeed([
      makeActivity("visit", "document"),
      makeActivity("download", "room"),
      makeActivity("question", "document"),
    ]);
    expect(screen.getAllByText(/alice@example.test/).length).toBe(3);
    expect(screen.getAllByText(/Financial Model/).length).toBe(3);
  });

  it("links document activity to document page", async () => {
    await renderFeed([makeActivity("visit", "document")]);
    const link = screen.getByText(/visited/).closest("a");
    expect(link).toHaveAttribute("href", "/acme/documents/doc-1");
  });

  it("links room activity to deal room page", async () => {
    await renderFeed([makeActivity("download", "room")]);
    const link = screen.getByText(/downloaded/).closest("a");
    expect(link).toHaveAttribute("href", "/acme/deal-rooms/doc-1");
  });

  it("shows anonymous user label for opaque actor ids", async () => {
    await renderFeed([
      { ...makeActivity("visit", "document"), actor: "b307d3c93d3fc2d4" },
    ]);
    expect(screen.queryByText(/b307d3c93d3fc2d4/)).not.toBeInTheDocument();
    expect(screen.getByText("Anonymous user")).toBeInTheDocument();
  });
});
