import type { PublicAskFAQ } from "@/types";

/** Client-side Help Center filter over the public FAQ catalog (≤20 items). */
export function filterVisitorFAQs(faqs: PublicAskFAQ[], query: string): PublicAskFAQ[] {
  const needle = query.trim().toLowerCase();
  if (!needle) return faqs;
  return faqs.filter((faq) => {
    if (faq.question.toLowerCase().includes(needle)) return true;
    if (faq.answer.toLowerCase().includes(needle)) return true;
    return (faq.aliases ?? []).some((alias) => alias.toLowerCase().includes(needle));
  });
}
