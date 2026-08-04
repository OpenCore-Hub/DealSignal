// @vitest-environment jsdom
import { describe, expect, it, vi, beforeEach } from "vitest";
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { I18nextProvider } from "react-i18next";
import { createTestI18n } from "@/i18n/test-utils";
import { MissionProgressRail } from "./MissionProgressRail";

const getProgress = vi.fn();
const listMissions = vi.fn();
const setMission = vi.fn();

vi.mock("@/lib/api", () => ({
  api: {
    getDealRoomKnowledgeMissionProgress: (...args: unknown[]) =>
      getProgress(...args),
    listDealRoomKnowledgeMissions: (...args: unknown[]) => listMissions(...args),
    setDealRoomKnowledgeMission: (...args: unknown[]) => setMission(...args),
  },
}));

const dealRooms = {
  "knowledge.missionProgressTitle": "Mission progress",
  "knowledge.missionProgressHint": "Checklist vs audited session state.",
  "knowledge.missionProgressCount": "{{covered}} / {{total}} covered",
  "knowledge.missionProgressAsk": "Ask this",
  "knowledge.missionProgressComplete": "All checklist items covered.",
  "knowledge.missionProgressLoading": "Loading mission…",
  "knowledge.missionProgressSwitchPack": "Switch pack",
  "knowledge.missionProgressChange": "Change",
  "knowledge.missionProgressExpand": "Expand",
  "knowledge.missionProgressCollapse": "Collapse",
};

describe("MissionProgressRail", () => {
  beforeEach(() => {
    getProgress.mockReset();
    listMissions.mockReset();
    setMission.mockReset();
    listMissions.mockResolvedValue({
      items: [
        { packId: "financing_dd_v1", title: "Financing due diligence", source: "catalog" },
        { packId: "ma_redflag_v1", title: "M&A red-flag review", source: "catalog" },
      ],
    });
  });

  it("defaults from template without a primary pack select", async () => {
    getProgress.mockResolvedValue({
      packId: "financing_dd_v1",
      title: "Financing due diligence",
      source: "template_default",
      covered: 0,
      total: 1,
      items: [
        { id: "valuation_cap", prompt: "What is the valuation cap?", covered: false },
      ],
    });
    const i18n = await createTestI18n({ dealRooms });
    render(
      <I18nextProvider i18n={i18n}>
        <MissionProgressRail roomId="room-1" />
      </I18nextProvider>,
    );

    await waitFor(() => {
      expect(screen.getByTestId("knowledge-mission-progress-rail")).toBeInTheDocument();
    });
    expect(screen.getByText("Financing due diligence")).toBeInTheDocument();
    expect(screen.queryByText("Mission progress")).not.toBeInTheDocument();
    expect(screen.getByTestId("knowledge-mission-progress-rail")).toHaveAttribute(
      "data-source",
      "template_default",
    );
    expect(screen.queryByTestId("knowledge-mission-pack-select")).not.toBeInTheDocument();
    expect(screen.getByTestId("knowledge-mission-pack-change")).toBeInTheDocument();
  });

  it("shows covered and uncovered items and asks on uncovered click", async () => {
    getProgress.mockResolvedValue({
      packId: "financing_dd_v1",
      title: "Financing due diligence",
      source: "room",
      covered: 1,
      total: 2,
      items: [
        { id: "valuation_cap", prompt: "What is the valuation cap?", covered: true },
        { id: "option_pool", prompt: "How is the option pool sized?", covered: false },
      ],
    });
    const onAsk = vi.fn();
    const i18n = await createTestI18n({ dealRooms });
    render(
      <I18nextProvider i18n={i18n}>
        <MissionProgressRail
          roomId="room-1"
          sessionId="sess-1"
          onAskItem={onAsk}
        />
      </I18nextProvider>,
    );

    await waitFor(() => {
      expect(screen.getByTestId("knowledge-mission-progress-rail")).toBeInTheDocument();
    });
    expect(screen.getByTestId("knowledge-mission-progress-rail")).toHaveAttribute(
      "data-source",
      "room",
    );
    expect(screen.getByTestId("knowledge-mission-progress-count")).toHaveTextContent(
      "1 / 2 covered",
    );
    expect(screen.getByText("What is the valuation cap?")).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Ask this" }));
    expect(onAsk).toHaveBeenCalledWith("How is the option pool sized?");
    expect(getProgress).toHaveBeenCalledWith("room-1", { sessionId: "sess-1" });
  });

  it("changes pack from the secondary Change menu", async () => {
    getProgress
      .mockResolvedValueOnce({
        packId: "financing_dd_v1",
        title: "Financing due diligence",
        source: "template_default",
        covered: 0,
        total: 1,
        items: [
          { id: "valuation_cap", prompt: "What is the valuation cap?", covered: false },
        ],
      })
      .mockResolvedValueOnce({
        packId: "ma_redflag_v1",
        title: "M&A red-flag review",
        source: "room",
        covered: 0,
        total: 1,
        items: [
          {
            id: "change_of_control",
            prompt: "Which contracts have change-of-control rights?",
            covered: false,
          },
        ],
      });
    setMission.mockResolvedValue({
      packId: "ma_redflag_v1",
      title: "M&A red-flag review",
      source: "room",
    });

    const i18n = await createTestI18n({ dealRooms });
    render(
      <I18nextProvider i18n={i18n}>
        <MissionProgressRail roomId="room-1" sessionId="sess-1" />
      </I18nextProvider>,
    );

    await waitFor(() => {
      expect(screen.getByTestId("knowledge-mission-pack-change")).toBeInTheDocument();
    });
    fireEvent.click(screen.getByTestId("knowledge-mission-pack-change"));
    await waitFor(() => {
      expect(
        screen.getByTestId("knowledge-mission-pack-option-ma_redflag_v1"),
      ).toBeInTheDocument();
    });
    fireEvent.click(screen.getByTestId("knowledge-mission-pack-option-ma_redflag_v1"));

    await waitFor(() => {
      expect(setMission).toHaveBeenCalledWith("room-1", { packId: "ma_redflag_v1" });
    });
    await waitFor(() => {
      expect(screen.getByText("M&A red-flag review")).toBeInTheDocument();
      expect(screen.getByTestId("knowledge-mission-progress-rail")).toHaveAttribute(
        "data-source",
        "room",
      );
    });
  });

  it("shows complete message when all covered and can expand details", async () => {
    getProgress.mockResolvedValue({
      packId: "financing_dd_v1",
      title: "Financing due diligence",
      source: "template_default",
      covered: 1,
      total: 1,
      items: [
        { id: "valuation_cap", prompt: "What is the valuation cap?", covered: true },
      ],
    });
    const i18n = await createTestI18n({ dealRooms });
    render(
      <I18nextProvider i18n={i18n}>
        <MissionProgressRail roomId="room-1" />
      </I18nextProvider>,
    );
    await waitFor(() => {
      expect(screen.getByTestId("knowledge-mission-progress-rail")).toHaveAttribute(
        "data-expanded",
        "false",
      );
    });
    expect(screen.queryByTestId("knowledge-mission-progress-complete")).not.toBeInTheDocument();
    fireEvent.click(screen.getByTestId("knowledge-mission-progress-toggle"));
    expect(screen.getByTestId("knowledge-mission-progress-complete")).toBeInTheDocument();
  });

  it("stays expanded while checklist items remain uncovered", async () => {
    getProgress.mockResolvedValue({
      packId: "financing_dd_v1",
      title: "Financing due diligence",
      source: "room",
      covered: 0,
      total: 1,
      items: [
        { id: "valuation_cap", prompt: "What is the valuation cap?", covered: false },
      ],
    });
    const i18n = await createTestI18n({ dealRooms });
    render(
      <I18nextProvider i18n={i18n}>
        <MissionProgressRail roomId="room-1" />
      </I18nextProvider>,
    );
    await waitFor(() => {
      expect(screen.getByTestId("knowledge-mission-progress-rail")).toHaveAttribute(
        "data-expanded",
        "true",
      );
    });
    expect(screen.getByTestId("knowledge-mission-progress-items")).toBeInTheDocument();
    fireEvent.click(screen.getByTestId("knowledge-mission-progress-toggle"));
    expect(screen.queryByTestId("knowledge-mission-progress-items")).not.toBeInTheDocument();
  });

  it("keeps all checklist items in the DOM with a scrollable viewport past 5", async () => {
    getProgress.mockResolvedValue({
      packId: "financing_dd_v1",
      title: "Financing due diligence",
      source: "room",
      covered: 0,
      total: 7,
      items: Array.from({ length: 7 }, (_, i) => ({
        id: `item_${i + 1}`,
        prompt: `Checklist item ${i + 1}?`,
        covered: false,
      })),
    });
    const i18n = await createTestI18n({ dealRooms });
    render(
      <I18nextProvider i18n={i18n}>
        <MissionProgressRail roomId="room-1" />
      </I18nextProvider>,
    );
    await waitFor(() => {
      expect(screen.getByTestId("knowledge-mission-progress-items")).toBeInTheDocument();
    });
    expect(screen.getByText("Checklist item 7?")).toBeInTheDocument();
    expect(screen.getByTestId("knowledge-mission-progress-items").className).toMatch(
      /overflow-y-auto/,
    );
  });
});
