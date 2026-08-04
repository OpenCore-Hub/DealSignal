import { useEffect, useState } from "react";
import { useSearchParams } from "react-router";
import { CanvasViewer } from "@/components/viewer/CanvasViewer";
import { useUIStore } from "@/stores/uiStore";

/**
 * Seed workspace from ?ws= after UI persist hydration so /viewer API calls
 * work on full reload / new tab (ceiling Phase X).
 */
function useViewerWorkspaceReady(workspaceSlug: string | undefined): boolean {
  const [ready, setReady] = useState(false);

  useEffect(() => {
    const finish = () => {
      const ws = (workspaceSlug || "").trim();
      if (ws) {
        const cur = useUIStore.getState().currentWorkspace;
        if (cur?.slug !== ws) {
          useUIStore.getState().setCurrentWorkspace({
            id: cur?.id || `ws:${ws}`,
            slug: ws,
            name: cur?.name || ws,
            logoUrl: cur?.logoUrl,
          });
        }
      }
      setReady(true);
    };
    if (useUIStore.persist.hasHydrated()) {
      finish();
      return;
    }
    return useUIStore.persist.onFinishHydration(finish);
  }, [workspaceSlug]);

  return ready;
}

export function ViewerPage() {
  const [searchParams] = useSearchParams();
  const knowledgeRoomId = (searchParams.get("roomId") || "").trim() || undefined;
  const workspaceSlug = (searchParams.get("ws") || "").trim() || undefined;
  const ready = useViewerWorkspaceReady(workspaceSlug);

  return (
    <div className="relative flex h-[100dvh] flex-col overflow-hidden">
      {ready ? <CanvasViewer knowledgeRoomId={knowledgeRoomId} /> : null}
    </div>
  );
}
