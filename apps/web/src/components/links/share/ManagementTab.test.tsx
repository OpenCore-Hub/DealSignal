// @vitest-environment jsdom
import { describe, it, expect, vi } from "vitest";
import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import { ManagementTab } from "./ManagementTab";
import type { FileRequest, VisitorQuestion } from "@/types";

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
  it("renders questions and file requests", () => {
    render(
      <ManagementTab
        questions={[makeQuestion()]}
        fileRequests={[makeFileRequest()]}
        onAnswer={vi.fn()}
        onUpdateFileRequest={vi.fn()}
      />
    );
    expect(screen.getByText("What is the pricing?")).toBeInTheDocument();
    expect(screen.getByText("Please send the full report.")).toBeInTheDocument();
  });

  it("submits an answer via the onAnswer callback", async () => {
    const onAnswer = vi.fn().mockResolvedValue(undefined);
    render(
      <ManagementTab
        questions={[makeQuestion()]}
        fileRequests={[]}
        onAnswer={onAnswer}
        onUpdateFileRequest={vi.fn()}
      />
    );

    const textarea = screen.getByPlaceholderText("management.answerPlaceholder");
    fireEvent.change(textarea, { target: { value: "Pricing starts at $99." } });
    fireEvent.click(screen.getByText("management.sendAnswer"));

    await waitFor(() => {
      expect(onAnswer).toHaveBeenCalledWith("q1", "Pricing starts at $99.");
    });
  });
});
