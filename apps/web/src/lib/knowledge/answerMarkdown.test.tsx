/** @vitest-environment jsdom */
import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { AnswerMarkdown } from "@/components/deal-rooms/knowledge/AnswerMarkdown";
import {
  encodeAnswerCitations,
  normalizeAnswerMarkdown,
  prepareAnswerMarkdown,
  CITE_TOKEN_OPEN,
  CITE_TOKEN_CLOSE,
} from "./answerMarkdown";

describe("answerMarkdown helpers", () => {
  it("encodes cite markers so markdown will not treat them as links", () => {
    expect(encodeAnswerCitations("价格为十百万 [1]。")).toBe(
      `价格为十百万 ${CITE_TOKEN_OPEN}1${CITE_TOKEN_CLOSE}。`,
    );
  });

  it("inserts blank lines before headings and lists", () => {
    const got = normalizeAnswerMarkdown(
      "导语如下：\n### 保密条款\n- 定义 [1]\n- 排除 [2]",
    );
    expect(got).toContain("\n\n### 保密条款");
    expect(got).toContain("\n\n- 定义");
  });

  it("prepares source with both normalize and cite encode", () => {
    const got = prepareAnswerMarkdown("前言\n## 概要\n要点 [3]");
    expect(got).toContain("\n\n## 概要");
    expect(got).toContain(`${CITE_TOKEN_OPEN}3${CITE_TOKEN_CLOSE}`);
    expect(got).not.toContain("[3]");
  });
});

describe("AnswerMarkdown", () => {
  it("renders headings, lists, and clickable cites from answer text", () => {
    const onCite = vi.fn();
    render(
      <AnswerMarkdown
        answer={"根据资料：\n### 保密条款\n- 定义见合同 [1]\n- 排除条款 [2]"}
        onCite={onCite}
      />,
    );
    expect(screen.getByTestId("knowledge-answer-markdown")).toBeInTheDocument();
    expect(screen.getByRole("heading", { level: 3, name: "保密条款" })).toBeInTheDocument();
    expect(screen.getByText("定义见合同")).toBeInTheDocument();
    fireEvent.click(screen.getByTestId("knowledge-cite-1"));
    expect(onCite).toHaveBeenCalledWith(1);
  });

  it("marks the active cite chip", () => {
    render(
      <AnswerMarkdown answer="条款成立 [1]。" activeCite={1} onCite={() => {}} />,
    );
    expect(screen.getByTestId("knowledge-cite-1").className).toMatch(/ring-1/);
  });
});
