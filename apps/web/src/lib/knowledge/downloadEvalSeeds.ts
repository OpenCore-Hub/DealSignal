import type { DealRoomKnowledgeEvalSeedExport } from "@/types";

/** Trigger a browser download for accepted gold seeds (ceiling Phase R). */
export function downloadEvalSeedExport(
  pack: DealRoomKnowledgeEvalSeedExport,
  filename?: string,
): void {
  const name =
    (filename || "").trim() ||
    `knowledge-eval-seeds-${new Date().toISOString().slice(0, 10)}.json`;
  const blob = new Blob([JSON.stringify(pack, null, 2)], {
    type: "application/json;charset=utf-8",
  });
  const url = URL.createObjectURL(blob);
  const a = document.createElement("a");
  a.href = url;
  a.download = name;
  a.rel = "noopener";
  document.body.appendChild(a);
  a.click();
  a.remove();
  URL.revokeObjectURL(url);
}
