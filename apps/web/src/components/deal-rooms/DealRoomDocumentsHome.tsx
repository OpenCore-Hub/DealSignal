import { WarningCircle } from "@phosphor-icons/react";
import { useTranslation } from "react-i18next";
import type { DealRoomTab } from "@/hooks/useDealRoomTab";
import { KnowledgeBasePanel } from "./KnowledgeBasePanel";
import type { DealRoomDocumentItem, DealRoomFolder } from "@/types";

interface DealRoomDocumentsHomeProps {
  roomId: string;
  isAdmin?: boolean;
  documents?: DealRoomDocumentItem[];
  folders?: DealRoomFolder[];
  activeLinkCount: number;
  failedDeliveries: number;
  unreadQuestions: number;
  onJumpTab: (tab: DealRoomTab) => void;
  children: React.ReactNode;
}

/**
 * Documents tab shell: optional cross-tab attention signals only.
 * Command strip / readiness banners were removed — tree is the primary surface.
 */
export function DealRoomDocumentsHome({
  roomId,
  isAdmin = false,
  documents = [],
  folders = [],
  activeLinkCount,
  failedDeliveries,
  unreadQuestions,
  onJumpTab,
  children,
}: DealRoomDocumentsHomeProps) {
  const { t } = useTranslation("dealRooms");
  const attentionItems: { key: string; tab: DealRoomTab; label: string }[] = [];

  if (activeLinkCount === 0) {
    attentionItems.push({
      key: "no-links",
      tab: "participants",
      label: t("documentsHome.attention.noActiveLinks"),
    });
  }
  if (failedDeliveries > 0) {
    attentionItems.push({
      key: "failed",
      tab: "participants",
      label: t("documentsHome.attention.failedDeliveries", { count: failedDeliveries }),
    });
  }
  if (unreadQuestions > 0) {
    attentionItems.push({
      key: "qa",
      tab: "qa",
      label: t("documentsHome.attention.unreadQuestions", { count: unreadQuestions }),
    });
  }

  return (
    <div className="space-y-4">
      {attentionItems.length > 0 && (
        <div
          className="flex flex-col gap-2 rounded-lg border border-amber-500/30 bg-amber-500/5 px-3 py-2.5 sm:flex-row sm:items-center sm:justify-between"
          data-testid="deal-room-attention-banner"
          role="status"
        >
          <div className="flex items-start gap-2 text-sm text-foreground">
            <WarningCircle size={18} className="mt-0.5 shrink-0 text-amber-600 dark:text-amber-400" />
            <div className="flex flex-wrap gap-x-3 gap-y-1">
              {attentionItems.map((item) => (
                <button
                  key={item.key}
                  type="button"
                  className="text-left underline-offset-2 hover:underline"
                  onClick={() => onJumpTab(item.tab)}
                >
                  {item.label}
                </button>
              ))}
            </div>
          </div>
        </div>
      )}

      <KnowledgeBasePanel
        roomId={roomId}
        isAdmin={isAdmin}
        documents={documents}
        folders={folders}
      />

      {children}
    </div>
  );
}
