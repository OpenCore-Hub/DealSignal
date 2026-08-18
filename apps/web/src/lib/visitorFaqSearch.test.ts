import { describe, expect, it } from "vitest";
import { filterVisitorFAQs } from "./visitorFaqSearch";
import type { PublicAskFAQ } from "@/types";

const faqs: PublicAskFAQ[] = [
  {
    id: "1",
    question: "What is ARR?",
    answer: "Annual recurring revenue is $12M.",
    source: "host",
    aliases: ["yearly recurring revenue"],
    pinned_at: "2026-01-01T00:00:00Z",
  },
  {
    id: "2",
    question: "What is GMV?",
    answer: "Gross merchandise value.",
    source: "ai",
    pinned_at: "2026-01-02T00:00:00Z",
  },
];

describe("filterVisitorFAQs", () => {
  it("returns all FAQs when the query is empty", () => {
    expect(filterVisitorFAQs(faqs, "  ")).toHaveLength(2);
  });

  it("matches question, answer, or alias", () => {
    expect(filterVisitorFAQs(faqs, "arr").map((f) => f.id)).toEqual(["1"]);
    expect(filterVisitorFAQs(faqs, "12m").map((f) => f.id)).toEqual(["1"]);
    expect(filterVisitorFAQs(faqs, "yearly").map((f) => f.id)).toEqual(["1"]);
  });

  it("returns empty when nothing matches", () => {
    expect(filterVisitorFAQs(faqs, "cap table")).toEqual([]);
  });
});
