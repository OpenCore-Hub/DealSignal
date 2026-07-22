// @vitest-environment jsdom
import { describe, it, expect, vi } from "vitest";
import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import { I18nextProvider } from "react-i18next";
import { ManagementTab } from "./ManagementTab";
import { createTestI18n } from "@/i18n/test-utils";
import enLinkShare from "@/i18n/locales/en/linkShare.json";
import type { FileRequest, VisitorQuestion } from "@/types";

async function renderTab(props: {
  questions?: VisitorQuestion[];
  fileRequests?: FileRequest[];
  onAnswer?: (id: string, answer: string) => Promise<void>;
  onUpdateFileRequest?: (id: string, status: string) => Promise<void>;
}) {
  const i18n = await createTestI18n({
    linkShare: enLinkShare as unknown as Record<string, string>,
  });
  return render(
    <I18nextProvider i18n={i18n}>
      <ManagementTab
        questions={props.questions ?? []}
        fileRequests={props.fileRequests ?? []}
        onAnswer={props.onAnswer ?? vi.fn()}
        onUpdateFileRequest={props.onUpdateFileRequest ?? vi.fn()}
      />
    </I18nextProvider>
  );
}

function makeQuestion(overrides: Partial<VisitorQuestion> = {}): VisitorQuestion {
  return {
    id: "q1",
    link_id: "l1",
    visitor_id: "v1",
    visitor_email: "visitor@example.com",
    question: "What is the pricing?",
    status: "pending",
    created_at: "2026-07-11T10:00:00Z",
    updated_at: "2026-07-11T10:00:00Z",
    ...overrides,
  };
}

function makeFileRequest(overrides: Partial<FileRequest> = {}): FileRequest {
  return {
    id: "fr1",
    link_id: "l1",
    visitor_id: "v1",
    visitor_email: "visitor@example.com",
    message: "Please send the full report.",
    status: "pending",
    created_at: "2026-07-11T10:00:00Z",
    updated_at: "2026-07-11T10:00:00Z",
    ...overrides,
  };
}

describe("ManagementTab", () => {
  it("labels Ask Host inbox separately from audit and Signal", async () => {
    await renderTab({ questions: [] });
    expect(screen.getByText("Ask Host inbox")).toBeInTheDocument();
    expect(
      screen.getByText(
        /not Ask Docs audit and not the Signal inbox/i
      )
    ).toBeInTheDocument();
    expect(screen.getByText("No Ask Host questions yet.")).toBeInTheDocument();
  });

  it("renders questions and file requests", async () => {
    await renderTab({
      questions: [makeQuestion()],
      fileRequests: [makeFileRequest()],
    });
    expect(screen.getByText("What is the pricing?")).toBeInTheDocument();
    expect(screen.getByText("Please send the full report.")).toBeInTheDocument();
  });

  it("submits an answer via the onAnswer callback", async () => {
    const onAnswer = vi.fn().mockResolvedValue(undefined);
    await renderTab({ questions: [makeQuestion()], onAnswer });

    const textarea = screen.getByPlaceholderText("Type your answer...");
    fireEvent.change(textarea, { target: { value: "Pricing starts at $99." } });
    fireEvent.click(screen.getByText("Send answer"));

    await waitFor(() => {
      expect(onAnswer).toHaveBeenCalledWith("q1", "Pricing starts at $99.");
    });
  });
});
