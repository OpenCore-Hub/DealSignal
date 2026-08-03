import { useState, createContext } from "react";
import { WarningCircle } from "@phosphor-icons/react";
import { useTranslation } from "react-i18next";
import type { DealRoomTab } from "@/hooks/useDealRoomTab";

/** Host element for the resources search/filter toolbar (portaled from the folder tree). */
export const ResourcesToolbarHostContext = createContext<HTMLElement | null>(null);

interface DealRoomDocumentsHomeProps {
  activeLinkCount: number;
  failedDeliveries: number;
  unreadQuestions: number;
  onJumpTab: (tab: DealRoomTab) => void;
  children: React.ReactNode;
}

/**
 * Documents tab shell: attention signals + external toolbar host share one row
 * above the resources card.
 */
export function DealRoomDocumentsHome({
  activeLinkCount,
  failedDeliveries,
  unreadQuestions,
  onJumpTab,
  children,
}: DealRoomDocumentsHomeProps) {
  const { t } = useTranslation("dealRooms");
  const [toolbarHost, setToolbarHost] = useState<HTMLDivElement | null>(null);
  const attentionItems: { key: string; tab: DealRoomTab; label: string }[] = [];

  if (activeLinkCount === 0) {
    attentionItems.push({
      key: "no-links",
      tab: "links",
      label: t("documentsHome.attention.noActiveLinks"),
    });
  }
  if (failedDeliveries > 0) {
    attentionItems.push({
      key: "failed",
      tab: "links",
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
    <ResourcesToolbarHostContext.Provider value={toolbarHost}>
      <div className="space-y-4">
        <div
          className="flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between sm:gap-3"
          data-testid="deal-room-documents-chrome"
        >
          <div className="min-w-0 flex-1">
            {attentionItems.length > 0 && (
              <div
                className="flex flex-col gap-2 rounded-lg border border-amber-500/30 bg-amber-500/5 px-3 py-2.5 sm:flex-row sm:items-center sm:justify-between"
                data-testid="deal-room-attention-banner"
                role="status"
              >
                <div className="flex items-start gap-2 text-sm text-foreground">
                  <WarningCircle
                    size={18}
                    className="mt-0.5 shrink-0 text-amber-600 dark:text-amber-400"
                  />
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
          </div>
          <div
            ref={setToolbarHost}
            className="flex shrink-0 flex-wrap items-center justify-end gap-2"
            data-testid="deal-room-resources-toolbar-host"
          />
        </div>

        {children}
      </div>
    </ResourcesToolbarHostContext.Provider>
  );
}
