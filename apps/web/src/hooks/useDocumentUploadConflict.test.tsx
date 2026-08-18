/** @vitest-environment jsdom */
import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor, fireEvent, act } from "@testing-library/react";
import { useState } from "react";
import { ApiError } from "@/lib/apiClient";
import {
  UploadCancelledError,
  isDocumentExistsError,
  useDocumentUploadConflict,
} from "./useDocumentUploadConflict";

const uploadDocumentMock = vi.hoisted(() => vi.fn());
const checkDocumentExistsMock = vi.hoisted(() => vi.fn());
const uploadDealRoomDocumentMock = vi.hoisted(() => vi.fn());
const checkDealRoomUploadExistsMock = vi.hoisted(() => vi.fn());

vi.mock("@/lib/api", () => ({
  api: {
    uploadDocument: uploadDocumentMock,
    checkDocumentExists: checkDocumentExistsMock,
    uploadDealRoomDocument: uploadDealRoomDocumentMock,
    checkDealRoomUploadExists: checkDealRoomUploadExistsMock,
  },
}));

vi.mock("react-i18next", () => ({
  useTranslation: () => ({
    t: (key: string, opts?: { name?: string }) =>
      opts?.name ? `${key}:${opts.name}` : key,
  }),
}));

function Harness({
  file,
  onAwaitingConflictChange,
}: {
  file: File;
  onAwaitingConflictChange?: (awaiting: boolean) => void;
}) {
  const { uploadDocument, conflictDialog } = useDocumentUploadConflict({
    onAwaitingConflictChange,
  });
  const [status, setStatus] = useState("idle");
  return (
    <div>
      <button
        type="button"
        onClick={() => {
          void uploadDocument(file)
            .then(() => setStatus("ok"))
            .catch((err: unknown) => {
              setStatus(err instanceof UploadCancelledError ? "cancelled" : "error");
            });
        }}
      >
        upload
      </button>
      <div data-testid="status">{status}</div>
      {conflictDialog}
    </div>
  );
}

function RoomHarness({ file }: { file: File }) {
  const { uploadDealRoomFile, conflictDialog } = useDocumentUploadConflict();
  const [status, setStatus] = useState("idle");
  const [code, setCode] = useState("");
  return (
    <div>
      <button
        type="button"
        onClick={() => {
          void uploadDealRoomFile("room-1", file, { folderPath: "/docs", sortOrder: 0 })
            .then(() => setStatus("ok"))
            .catch((err: unknown) => {
              if (err instanceof UploadCancelledError) {
                setStatus("cancelled");
                return;
              }
              setStatus("error");
              if (err && typeof err === "object" && "code" in err) {
                setCode(String((err as { code: unknown }).code));
              }
            });
        }}
      >
        upload-room
      </button>
      <div data-testid="status">{status}</div>
      <div data-testid="code">{code}</div>
      {conflictDialog}
    </div>
  );
}

describe("isDocumentExistsError", () => {
  it("matches ApiError and duck-typed payloads", () => {
    expect(
      isDocumentExistsError(
        new ApiError({
          status: 409,
          code: "document_exists",
          message: "exists",
          requestId: "r1",
        }),
      ),
    ).toBe(true);
    expect(isDocumentExistsError({ code: "document_exists", status: 409 })).toBe(true);
    expect(isDocumentExistsError({ code: "document_exists_outside_room", status: 409 })).toBe(false);
    expect(isDocumentExistsError({ code: "http_error", status: 409 })).toBe(false);
    expect(isDocumentExistsError(new Error("nope"))).toBe(false);
  });
});

describe("useDocumentUploadConflict", () => {
  beforeEach(() => {
    uploadDocumentMock.mockReset();
    checkDocumentExistsMock.mockReset();
    checkDocumentExistsMock.mockResolvedValue({ exists: false });
    uploadDealRoomDocumentMock.mockReset();
    checkDealRoomUploadExistsMock.mockReset();
    checkDealRoomUploadExistsMock.mockResolvedValue({ exists: false, replaceable: false });
  });

  it("returns cleanly when the server accepts the upload", async () => {
    uploadDocumentMock.mockResolvedValueOnce({ id: "d1", title: "a.pdf" });
    const file = new File(["x"], "a.pdf", { type: "application/pdf" });
    render(<Harness file={file} />);

    fireEvent.click(screen.getByRole("button", { name: "upload" }));
    await waitFor(() => expect(screen.getByTestId("status")).toHaveTextContent("ok"));
    expect(checkDocumentExistsMock).toHaveBeenCalledWith("a.pdf");
    expect(uploadDocumentMock).toHaveBeenCalledWith(file, undefined);
    expect(uploadDocumentMock).toHaveBeenCalledTimes(1);
  });

  it("asks before uploading when the preflight finds an existing document", async () => {
    checkDocumentExistsMock.mockResolvedValueOnce({
      exists: true,
      document: { id: "d1", title: "a.pdf" },
    });
    uploadDocumentMock.mockResolvedValueOnce({ id: "d1", title: "a.pdf" });

    const file = new File(["x"], "a.pdf", { type: "application/pdf" });
    render(<Harness file={file} />);

    fireEvent.click(screen.getByRole("button", { name: "upload" }));
    await screen.findByText("upload.replaceTitle");
    expect(uploadDocumentMock).not.toHaveBeenCalled();
    fireEvent.click(screen.getByRole("button", { name: "upload.replaceConfirm" }));

    await waitFor(() => expect(screen.getByTestId("status")).toHaveTextContent("ok"));
    expect(uploadDocumentMock).toHaveBeenCalledTimes(1);
    expect(uploadDocumentMock).toHaveBeenCalledWith(file, undefined, { replace: true });
  });

  it("does not upload bytes when the user cancels a preflight conflict", async () => {
    checkDocumentExistsMock.mockResolvedValueOnce({
      exists: true,
      document: { id: "d1", title: "a.pdf" },
    });

    const file = new File(["x"], "a.pdf", { type: "application/pdf" });
    render(<Harness file={file} />);

    fireEvent.click(screen.getByRole("button", { name: "upload" }));
    await screen.findByText("upload.replaceTitle");
    await act(async () => {
      fireEvent.click(screen.getByRole("button", { name: "upload.replaceCancel" }));
    });

    await waitFor(() =>
      expect(screen.getByTestId("status")).toHaveTextContent("cancelled"),
    );
    expect(uploadDocumentMock).not.toHaveBeenCalled();
  });

  it("retries with replace after the user confirms", async () => {
    uploadDocumentMock
      .mockRejectedValueOnce({ code: "document_exists", status: 409, message: "exists" })
      .mockResolvedValueOnce({ id: "d1", title: "a.pdf" });

    const awaiting = vi.fn();
    const file = new File(["x"], "a.pdf", { type: "application/pdf" });
    render(<Harness file={file} onAwaitingConflictChange={awaiting} />);

    fireEvent.click(screen.getByRole("button", { name: "upload" }));
    await screen.findByText("upload.replaceTitle");
    expect(awaiting).toHaveBeenCalledWith(true);
    fireEvent.click(screen.getByRole("button", { name: "upload.replaceConfirm" }));

    await waitFor(() => expect(screen.getByTestId("status")).toHaveTextContent("ok"));
    expect(uploadDocumentMock).toHaveBeenLastCalledWith(file, undefined, { replace: true });
    await waitFor(() => expect(awaiting).toHaveBeenCalledWith(false));
  });

  it("surfaces UploadCancelledError when the user discards", async () => {
    uploadDocumentMock.mockRejectedValueOnce(
      new ApiError({
        status: 409,
        code: "document_exists",
        message: "exists",
        requestId: "r1",
      }),
    );

    const file = new File(["x"], "a.pdf", { type: "application/pdf" });
    render(<Harness file={file} />);

    fireEvent.click(screen.getByRole("button", { name: "upload" }));
    await screen.findByText("upload.replaceTitle");
    await act(async () => {
      fireEvent.click(screen.getByRole("button", { name: "upload.replaceCancel" }));
    });

    await waitFor(() =>
      expect(screen.getByTestId("status")).toHaveTextContent("cancelled"),
    );
    expect(uploadDocumentMock).toHaveBeenCalledTimes(1);
  });
});

describe("useDocumentUploadConflict uploadDealRoomFile", () => {
  beforeEach(() => {
    uploadDocumentMock.mockReset();
    checkDocumentExistsMock.mockReset();
    uploadDealRoomDocumentMock.mockReset();
    checkDealRoomUploadExistsMock.mockReset();
    checkDealRoomUploadExistsMock.mockResolvedValue({ exists: false, replaceable: false });
  });

  it("uploads through the room endpoint without a library exists check", async () => {
    uploadDealRoomDocumentMock.mockResolvedValueOnce({ id: "d1", title: "a.pdf" });
    const file = new File(["x"], "a.pdf", { type: "application/pdf" });
    render(<RoomHarness file={file} />);

    fireEvent.click(screen.getByRole("button", { name: "upload-room" }));
    await waitFor(() => expect(screen.getByTestId("status")).toHaveTextContent("ok"));
    expect(checkDealRoomUploadExistsMock).toHaveBeenCalledWith("room-1", "a.pdf");
    expect(checkDocumentExistsMock).not.toHaveBeenCalled();
    expect(uploadDealRoomDocumentMock).toHaveBeenCalledWith(
      "room-1",
      file,
      expect.objectContaining({ folderPath: "/docs", sortOrder: 0 }),
    );
    expect(uploadDocumentMock).not.toHaveBeenCalled();
  });

  it("uploads when a stale preflight still reports outside_room", async () => {
    checkDealRoomUploadExistsMock.mockResolvedValueOnce({
      exists: true,
      replaceable: false,
      reason: "outside_room",
    });
    uploadDealRoomDocumentMock.mockResolvedValueOnce({ id: "d1", title: "a.pdf" });
    const file = new File(["x"], "a.pdf", { type: "application/pdf" });
    render(<RoomHarness file={file} />);

    fireEvent.click(screen.getByRole("button", { name: "upload-room" }));
    await waitFor(() => expect(screen.getByTestId("status")).toHaveTextContent("ok"));
    expect(uploadDealRoomDocumentMock).toHaveBeenCalled();
  });

  it("uploads when the preflight reports no this-room collision", async () => {
    checkDealRoomUploadExistsMock.mockResolvedValueOnce({
      exists: false,
      replaceable: false,
    });
    uploadDealRoomDocumentMock.mockResolvedValueOnce({ id: "d1", title: "a.pdf" });
    const file = new File(["x"], "a.pdf", { type: "application/pdf" });
    render(<RoomHarness file={file} />);

    fireEvent.click(screen.getByRole("button", { name: "upload-room" }));
    await waitFor(() => expect(screen.getByTestId("status")).toHaveTextContent("ok"));
    expect(uploadDealRoomDocumentMock).toHaveBeenCalled();
  });

  it("blocks replace when the this-room file is locked", async () => {
    checkDealRoomUploadExistsMock.mockResolvedValueOnce({
      exists: true,
      replaceable: false,
      reason: "locked",
    });
    const file = new File(["x"], "a.pdf", { type: "application/pdf" });
    render(<RoomHarness file={file} />);

    fireEvent.click(screen.getByRole("button", { name: "upload-room" }));
    await waitFor(() => expect(screen.getByTestId("status")).toHaveTextContent("error"));
    expect(screen.getByTestId("code")).toHaveTextContent("resource_locked");
    expect(uploadDealRoomDocumentMock).not.toHaveBeenCalled();
  });

  it("asks to replace when the title already belongs to this room", async () => {
    checkDealRoomUploadExistsMock.mockResolvedValueOnce({
      exists: true,
      replaceable: true,
      document: { id: "d1", title: "a.pdf" },
    });
    uploadDealRoomDocumentMock.mockResolvedValueOnce({ id: "d1", title: "a.pdf" });
    const file = new File(["x"], "a.pdf", { type: "application/pdf" });
    render(<RoomHarness file={file} />);

    fireEvent.click(screen.getByRole("button", { name: "upload-room" }));
    await screen.findByText("upload.replaceTitle");
    fireEvent.click(screen.getByRole("button", { name: "upload.replaceConfirm" }));
    await waitFor(() => expect(screen.getByTestId("status")).toHaveTextContent("ok"));
    expect(uploadDealRoomDocumentMock).toHaveBeenCalledWith(
      "room-1",
      file,
      expect.objectContaining({ replace: true, folderPath: "/docs" }),
    );
  });
});
