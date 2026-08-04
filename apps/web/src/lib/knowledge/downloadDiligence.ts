import type { DealRoomKnowledgeDiligencePack } from "@/types";

/** Trigger a browser download for a diligence audit JSON pack. */
export function downloadDiligencePack(
  pack: DealRoomKnowledgeDiligencePack,
  filename?: string,
): void {
  const name =
    (filename || "").trim() ||
    `diligence-${pack.sessionId || "session"}.json`;
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
