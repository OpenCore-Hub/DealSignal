import { cn } from "@/lib/utils";
import type {
  DealRoomKnowledgeAnswerClaim,
  DealRoomKnowledgeQueryHit,
} from "@/types";

/** 1-based evidence index for a claim's first hitId, or 0 when unbound. */
export function claimCiteIndex(
  claim: DealRoomKnowledgeAnswerClaim,
  hits: DealRoomKnowledgeQueryHit[],
): number {
  for (const id of claim.hitIds ?? []) {
    const idx = hits.findIndex((h) => (h.chunkId || "").trim() === id.trim());
    if (idx >= 0) return idx + 1;
  }
  return 0;
}

export function claimIsFactStyled(claim: DealRoomKnowledgeAnswerClaim): boolean {
  const conf = (claim.confidence || "").trim();
  return (
    (conf === "grounded" || conf === "weak") && (claim.hitIds?.length ?? 0) > 0
  );
}

/**
 * Sentence-level answer with hit binding.
 * Grounded/weak claims get fact styling + cite control; unbound stay muted narrative.
 */
export function renderBoundClaims(
  claims: DealRoomKnowledgeAnswerClaim[],
  hits: DealRoomKnowledgeQueryHit[],
  activeCite: number | null,
  onCite: (n: number) => void,
) {
  return claims.map((claim, i) => {
    const cite = claimCiteIndex(claim, hits);
    const fact = claimIsFactStyled(claim);
    const active = cite > 0 && activeCite === cite;
    const grounded = (claim.confidence || "").trim() === "grounded";
    return (
      <span
        key={`claim-${i}`}
        className={cn(
          "inline",
          fact
            ? cn(
                "rounded-sm px-0.5 transition-colors",
                grounded
                  ? "bg-foreground/[0.04] text-foreground"
                  : "text-foreground/90",
                active && "bg-foreground/[0.08] ring-1 ring-foreground/15",
              )
            : "text-muted-foreground",
        )}
        data-testid={fact ? "knowledge-claim-fact" : "knowledge-claim-narrative"}
        data-confidence={claim.confidence || "none"}
      >
        {claim.text}
        {cite > 0 ? (
          <button
            type="button"
            className="mx-0.5 inline-flex h-5 min-w-5 items-center justify-center rounded-sm bg-foreground/[0.06] px-1 align-baseline font-mono text-[11px] font-semibold text-foreground transition-colors hover:bg-foreground/10"
            data-testid={`knowledge-cite-${cite}`}
            onClick={() => onCite(cite)}
          >
            {cite}
          </button>
        ) : null}
        {i < claims.length - 1 ? " " : null}
      </span>
    );
  });
}
