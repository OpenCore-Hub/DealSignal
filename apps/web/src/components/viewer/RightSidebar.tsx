import { useState } from "react";
import { motion, AnimatePresence } from "motion/react";
import { ChatCenteredDots, FileText, X } from "@phosphor-icons/react";
import { useTranslation } from "react-i18next";
import { FileRequestPanel } from "./FileRequestPanel";
import { UnifiedQAPanel } from "./UnifiedQAPanel";

interface DocSummary {
  id: string;
  title: string;
  pageCount: number;
}

type SidebarTab = "documents" | "qa" | "requests";

interface RightSidebarProps {
  open: boolean;
  onClose: () => void;
  documents?: DocSummary[];
  selectedDocIndex?: number;
  onSelectDoc?: (index: number) => void;
  activeDocumentId?: string;
  aiCopilotEnabled?: boolean;
  qaEnabled?: boolean;
  fileRequestsEnabled?: boolean;
  publicToken?: string;
  publicSessionToken?: string;
}

export function RightSidebar({
  open,
  onClose,
  documents,
  selectedDocIndex = 0,
  onSelectDoc,
  activeDocumentId,
  aiCopilotEnabled,
  qaEnabled,
  fileRequestsEnabled,
  publicToken,
  publicSessionToken,
}: RightSidebarProps) {
  const { t } = useTranslation(["documents", "ai"]);
  const qaAvailable = aiCopilotEnabled || qaEnabled;
  const [activeTab, setActiveTab] = useState<SidebarTab>(qaAvailable ? "qa" : "documents");
  const hasAnyFeature = qaAvailable || fileRequestsEnabled;

  return (
    <AnimatePresence>
      {open && (
        <motion.aside
          initial={{ width: 0, opacity: 0 }}
          animate={{ width: 320, opacity: 1 }}
          exit={{ width: 0, opacity: 0 }}
          transition={{ duration: 0.2, ease: [0.16, 1, 0.3, 1] }}
          className="flex h-full shrink-0 flex-col border-l border-border bg-card overflow-hidden"
          style={{ minWidth: open ? 320 : 0 }}
        >
          {/* Header with documents tab */}
          <div className="flex items-center border-b border-border">
            <button
              type="button"
              onClick={() => setActiveTab("documents")}
              className={`flex flex-1 items-center justify-center gap-1.5 border-b-2 px-3 py-2.5 text-xs font-medium transition-colors ${
                activeTab === "documents"
                  ? "border-primary text-primary"
                  : "border-transparent text-muted-foreground hover:text-foreground"
              }`}
            >
              <FileText size={14} />
              {t("documents:viewer.sidebarDocuments")}
            </button>
            {!hasAnyFeature && (
              <button
                type="button"
                onClick={onClose}
                className="mx-1 flex h-7 w-7 items-center justify-center rounded text-muted-foreground hover:bg-muted hover:text-foreground transition-colors"
                aria-label={t("ai:viewer.close")}
              >
                <X size={14} />
              </button>
            )}
          </div>

          {/* Feature tabs row */}
          {hasAnyFeature && (
            <div className="flex items-center border-b border-border">
              {qaAvailable && (
                <button
                  type="button"
                  onClick={() => setActiveTab("qa")}
                  className={`flex flex-1 items-center justify-center gap-1.5 border-b-2 px-2 py-2.5 text-xs font-medium transition-colors ${
                    activeTab === "qa"
                      ? "border-primary text-primary"
                      : "border-transparent text-muted-foreground hover:text-foreground"
                  }`}
                >
                  <ChatCenteredDots size={14} />
                  {t("documents:viewer.sidebarQA")}
                </button>
              )}
              {fileRequestsEnabled && (
                <button
                  type="button"
                  onClick={() => setActiveTab("requests")}
                  className={`flex flex-1 items-center justify-center gap-1.5 border-b-2 px-2 py-2.5 text-xs font-medium transition-colors ${
                    activeTab === "requests"
                      ? "border-primary text-primary"
                      : "border-transparent text-muted-foreground hover:text-foreground"
                  }`}
                >
                  <FileText size={14} />
                  {t("documents:viewer.sidebarRequests")}
                </button>
              )}
              <button
                type="button"
                onClick={onClose}
                className="mx-1 flex h-7 w-7 items-center justify-center rounded text-muted-foreground hover:bg-muted hover:text-foreground transition-colors shrink-0"
                aria-label={t("ai:viewer.close")}
              >
                <X size={14} />
              </button>
            </div>
          )}

          {/* Content */}
          <div className="flex-1 overflow-hidden">
            {activeTab === "documents" && (
              <div className="h-full overflow-y-auto py-1">
                {(!documents || documents.length === 0) ? (
                  <p className="px-4 py-8 text-center text-xs text-muted-foreground">
                    {t("documents:viewer.noDocuments")}
                  </p>
                ) : (
                  documents.map((d, i) => (
                    <button
                      key={d.id}
                      type="button"
                      onClick={() => onSelectDoc?.(i)}
                      className={`flex w-full items-start gap-3 px-4 py-2.5 text-left transition-colors hover:bg-muted/50 ${
                        i === selectedDocIndex
                          ? "bg-primary/5 border-l-2 border-l-primary"
                          : "border-l-2 border-l-transparent"
                      }`}
                    >
                      <FileText
                        size={16}
                        className={`mt-0.5 shrink-0 ${
                          i === selectedDocIndex ? "text-primary" : "text-muted-foreground"
                        }`}
                      />
                      <div className="min-w-0 flex-1">
                        <p
                          className={`text-sm font-medium truncate ${
                            i === selectedDocIndex ? "text-primary" : "text-foreground"
                          }`}
                        >
                          {d.title}
                        </p>
                        <p className="text-xs text-muted-foreground">
                          {d.pageCount} {t("documents:viewer.pages")}
                        </p>
                      </div>
                    </button>
                  ))
                )}
              </div>
            )}
            {activeTab === "requests" && fileRequestsEnabled && publicToken && (
              <FileRequestPanel token={publicToken} sessionToken={publicSessionToken} />
            )}
            {activeTab === "qa" && qaAvailable && publicToken && (
              <UnifiedQAPanel
                token={publicToken}
                sessionToken={publicSessionToken}
                documentId={activeDocumentId}
                qaEnabled={qaEnabled}
                aiCopilotEnabled={aiCopilotEnabled}
              />
            )}
          </div>
        </motion.aside>
      )}
    </AnimatePresence>
  );
}
