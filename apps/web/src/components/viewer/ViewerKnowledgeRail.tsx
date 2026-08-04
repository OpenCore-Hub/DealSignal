import { SealCheck, X } from "@phosphor-icons/react";
import { useTranslation } from "react-i18next";
import { useNavigate, useSearchParams } from "react-router";
import { GroundedChatShell } from "@/components/deal-rooms/knowledge/GroundedChatShell";
import { useKnowledgeDeskSession } from "@/hooks/useKnowledgeDeskSession";
import { viewerPath } from "@/lib/knowledge/citations";
import { useUIStore } from "@/stores/uiStore";
import { cn } from "@/lib/utils";

interface ViewerKnowledgeRailProps {
  roomId: string;
  documentId: string;
  /** Jump within the current authenticated viewer document. */
  onJumpToPage: (page: number) => void;
  onClose: () => void;
  className?: string;
}

/**
 * Owner Viewer grounded-ask rail (ceiling Phase T).
 * Reuses Knowledge session APIs + store — never mounts on public Visitor channel.
 */
export function ViewerKnowledgeRail({
  roomId,
  documentId,
  onJumpToPage,
  onClose,
  className,
}: ViewerKnowledgeRailProps) {
  const { t } = useTranslation(["dealRooms", "documents"]);
  const navigate = useNavigate();
  const [searchParams] = useSearchParams();
  const storeSlug = useUIStore((s) => s.currentWorkspace?.slug);
  const workspaceSlug =
    (searchParams.get("ws") || "").trim() || (storeSlug || "").trim() || undefined;
  const {
    query,
    setQuery,
    asking,
    viewTurns,
    sessionHydrated,
    onAsk,
    onStop,
    onFeedback,
    setActiveCite,
    recordCiteOpen,
  } = useKnowledgeDeskSession(roomId, { allowAsk: true });

  return (
    <aside
      className={cn(
        "flex h-full w-[360px] shrink-0 flex-col border-l border-border bg-card",
        className,
      )}
      data-testid="viewer-knowledge-rail"
      aria-label={t("documents:viewer.knowledgeRailTitle")}
    >
      <div className="flex items-start justify-between gap-2 border-b border-border px-3 py-2.5">
        <div className="min-w-0">
          <p className="text-[11px] font-semibold uppercase tracking-[0.12em] text-muted-foreground">
            {t("documents:viewer.knowledgeRailTitle")}
          </p>
          <p
            className="mt-1 inline-flex items-center gap-1 text-[11px] text-foreground/70"
            data-testid="viewer-knowledge-trust-chip"
          >
            <SealCheck size={12} weight="bold" className="text-foreground/55" />
            {t("dealRooms:knowledge.trustScoped")}
          </p>
        </div>
        <button
          type="button"
          onClick={onClose}
          className="inline-flex h-7 w-7 items-center justify-center rounded-md text-muted-foreground transition-colors hover:bg-muted hover:text-foreground"
          aria-label={t("documents:viewer.sidebarClose")}
          data-testid="viewer-knowledge-rail-close"
        >
          <X size={14} weight="bold" />
        </button>
      </div>

      <div className="min-h-0 flex-1 overflow-hidden px-2 pb-2 pt-1">
        {sessionHydrated ? (
          <GroundedChatShell
            className="h-full"
            query={query}
            onQueryChange={setQuery}
            turns={viewTurns}
            asking={asking}
            askEnabled
            onAsk={(override) => {
              void onAsk(override);
            }}
            onStop={onStop}
            onActiveCite={setActiveCite}
            onOpenViewer={(docId, page) => {
              recordCiteOpen();
              if (docId === documentId && page && page > 0) {
                onJumpToPage(page);
                return;
              }
              navigate(viewerPath(docId, page, { roomId, workspaceSlug }));
            }}
            onFeedback={onFeedback}
          />
        ) : (
          <p className="px-2 py-4 text-[11px] text-muted-foreground">
            {t("documents:viewer.knowledgeRailLoading")}
          </p>
        )}
      </div>
    </aside>
  );
}
