// @vitest-environment jsdom
import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor, fireEvent } from "@testing-library/react";
import { I18nextProvider } from "react-i18next";
import { createTestI18n } from "@/i18n/test-utils";
import { DealRoomNdaGate } from "./DealRoomNdaGate";

const { signDealRoomMemberNdaMock } = vi.hoisted(() => ({
  signDealRoomMemberNdaMock: vi.fn(),
}));

vi.mock("@/lib/api", () => ({
  api: {
    signDealRoomMemberNda: signDealRoomMemberNdaMock,
  },
}));

vi.mock("sonner", () => ({
  toast: { success: vi.fn(), error: vi.fn() },
}));

describe("DealRoomNdaGate", () => {
  beforeEach(() => {
    signDealRoomMemberNdaMock.mockReset();
    signDealRoomMemberNdaMock.mockResolvedValue({ id: "room-1", ndaRequired: false });
  });

  it("signs NDA and notifies parent", async () => {
    const i18n = await createTestI18n({
      dealRooms: {
        ndaGate: {
          title: "Sign NDA to continue",
          description: "You have been added to {{name}}.",
          sign: "Sign NDA and continue",
          signing: "Signing…",
          signed: "NDA signed",
          failed: "Could not sign the NDA",
        },
      },
    });
    const onSigned = vi.fn();
    render(
      <I18nextProvider i18n={i18n}>
        <DealRoomNdaGate roomId="room-1" roomName="Series A" onSigned={onSigned} />
      </I18nextProvider>,
    );
    fireEvent.click(screen.getByTestId("deal-room-nda-sign"));
    await waitFor(() => {
      expect(signDealRoomMemberNdaMock).toHaveBeenCalledWith("room-1");
      expect(onSigned).toHaveBeenCalled();
    });
  });
});
