// @vitest-environment jsdom
import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor, fireEvent } from "@testing-library/react";
import { I18nextProvider } from "react-i18next";
import { createTestI18n } from "@/i18n/test-utils";
import { ApiError } from "@/lib/apiClient";
import { DealRoomNdaGate } from "./DealRoomNdaGate";

const { getDealRoomMemberNdaPreviewMock, signDealRoomMemberNdaMock } = vi.hoisted(() => ({
  getDealRoomMemberNdaPreviewMock: vi.fn(),
  signDealRoomMemberNdaMock: vi.fn(),
}));

vi.mock("@/lib/api", () => ({
  api: {
    getDealRoomMemberNdaPreview: getDealRoomMemberNdaPreviewMock,
    signDealRoomMemberNda: signDealRoomMemberNdaMock,
  },
}));

vi.mock("sonner", () => ({
  toast: { success: vi.fn(), error: vi.fn() },
}));

const ndaGateCopy = {
  title: "Sign NDA to continue",
  description: "You have been added to {{name}}.",
  sign: "Sign NDA and continue",
  signing: "Signing…",
  signed: "NDA signed",
  failed: "Could not sign the NDA",
  previewLoading: "Loading agreement…",
  previewUnavailable: "Agreement preview is unavailable.",
  previewPage: "Page {{page}}",
  previewTitle: "Agreement preview",
  agree: "I have read and agree to this agreement",
  signingAs: "Signing as {{email}}",
};

const previewPayload = {
  ndaTemplate: {
    id: "tpl-1",
    name: "Standard NDA",
    contentSha256: "abc123",
    sourceDocumentId: "doc-1",
  },
  document: { id: "doc-1", title: "Standard NDA", pageCount: 1, sourceType: "upload" },
  previewPageUrls: ["https://files.example/nda-page-1.png"],
  documentUrl: "https://files.example/nda.pdf",
  signerEmail: "invitee@example.com",
  expiresAt: "2099-01-01T00:00:00Z",
};

describe("DealRoomNdaGate", () => {
  beforeEach(() => {
    getDealRoomMemberNdaPreviewMock.mockReset();
    signDealRoomMemberNdaMock.mockReset();
    getDealRoomMemberNdaPreviewMock.mockResolvedValue(previewPayload);
    signDealRoomMemberNdaMock.mockResolvedValue({ id: "room-1", ndaRequired: false });
  });

  it("does not sign until the agreement is shown and the member agrees", async () => {
    const i18n = await createTestI18n({
      dealRooms: { ndaGate: ndaGateCopy },
    });
    const onSigned = vi.fn();
    render(
      <I18nextProvider i18n={i18n}>
        <DealRoomNdaGate roomId="room-1" roomName="Series A" onSigned={onSigned} />
      </I18nextProvider>,
    );

    expect(await screen.findByAltText("Page 1")).toHaveAttribute(
      "src",
      "https://files.example/nda-page-1.png",
    );
    expect(screen.getByText("Signing as invitee@example.com")).toBeTruthy();

    const sign = screen.getByTestId("deal-room-nda-sign");
    expect(sign).toHaveProperty("disabled", true);
    fireEvent.click(sign);
    expect(signDealRoomMemberNdaMock).not.toHaveBeenCalled();

    fireEvent.click(screen.getByTestId("deal-room-nda-agree"));
    expect(sign).toHaveProperty("disabled", false);
    fireEvent.click(sign);
    await waitFor(() => {
      expect(signDealRoomMemberNdaMock).toHaveBeenCalledWith("room-1", {
        agreed: true,
        content_sha256: "abc123",
      });
      expect(onSigned).toHaveBeenCalled();
    });
  });

  it("hides the sign control when preview cannot load", async () => {
    getDealRoomMemberNdaPreviewMock.mockRejectedValueOnce(new Error("unavailable"));
    const i18n = await createTestI18n({
      dealRooms: { ndaGate: ndaGateCopy },
    });
    render(
      <I18nextProvider i18n={i18n}>
        <DealRoomNdaGate roomId="room-1" roomName="Series A" onSigned={vi.fn()} />
      </I18nextProvider>,
    );
    expect(await screen.findByRole("alert")).toHaveTextContent("Agreement preview is unavailable.");
    expect(screen.queryByTestId("deal-room-nda-sign")).toBeNull();
  });

  it("uses a translated iframe title, not the template name", async () => {
    getDealRoomMemberNdaPreviewMock.mockResolvedValueOnce({
      ...previewPayload,
      previewPageUrls: [],
    });
    const i18n = await createTestI18n({
      dealRooms: { ndaGate: ndaGateCopy },
    });
    render(
      <I18nextProvider i18n={i18n}>
        <DealRoomNdaGate roomId="room-1" roomName="Series A" onSigned={vi.fn()} />
      </I18nextProvider>,
    );
    const frame = await screen.findByTitle("Agreement preview");
    expect(frame.tagName).toBe("IFRAME");
    expect(frame.getAttribute("src")).toBe("https://files.example/nda.pdf");
  });

  it("reloads the agreement after a content-hash mismatch", async () => {
    getDealRoomMemberNdaPreviewMock
      .mockResolvedValueOnce(previewPayload)
      .mockResolvedValueOnce({
        ...previewPayload,
        ndaTemplate: { ...previewPayload.ndaTemplate, contentSha256: "newhash" },
        previewPageUrls: ["https://files.example/nda-page-2.png"],
      });
    signDealRoomMemberNdaMock.mockRejectedValueOnce(
      new ApiError({
        status: 409,
        code: "nda_content_mismatch",
        message: "mismatch",
        requestId: "r1",
      }),
    );
    const i18n = await createTestI18n({
      dealRooms: { ndaGate: ndaGateCopy },
    });
    render(
      <I18nextProvider i18n={i18n}>
        <DealRoomNdaGate roomId="room-1" roomName="Series A" onSigned={vi.fn()} />
      </I18nextProvider>,
    );
    expect(await screen.findByAltText("Page 1")).toBeTruthy();
    fireEvent.click(screen.getByTestId("deal-room-nda-agree"));
    fireEvent.click(screen.getByTestId("deal-room-nda-sign"));
    await waitFor(() => {
      expect(screen.getByAltText("Page 1")).toHaveAttribute(
        "src",
        "https://files.example/nda-page-2.png",
      );
    });
    expect(getDealRoomMemberNdaPreviewMock).toHaveBeenCalledTimes(2);
  });
});
