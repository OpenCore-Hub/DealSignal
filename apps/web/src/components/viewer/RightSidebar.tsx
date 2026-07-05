import { useState } from "react";
import { motion, AnimatePresence } from "motion/react";
import { FileText, Robot, X } from "@phosphor-icons/react";
import { useTranslation } from "react-i18next";
import { SidebarAIChat } from "./SidebarAIChat";

interface DocSummary {
  id: string;
  title: string;
  pageCount: number;
}

interface RightSidebarProps {
  open: boolean;
  onClose: () => void;
  documents?: DocSummary[];
  selectedDocIndex?: number;
  onSelectDoc?: (index: number) => void;
  activeDocumentId?: string;
}

export function RightSidebar({
  open,
  onClose,
  documents,
  selectedDocIndex = 0,
  onSelectDoc,
  activeDocumentId,
}: RightSidebarProps) {
  const { t } = useTranslation(["documents", "ai"]);
  const [activeTab, setActiveTab] = useState<"documents" | "ai">("documents");

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
          {/* Header with tabs */}
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
            <button
              type="button"
              onClick={() => setActiveTab("ai")}
              className={`flex flex-1 items-center justify-center gap-1.5 border-b-2 px-3 py-2.5 text-xs font-medium transition-colors ${
                activeTab === "ai"
                  ? "border-primary text-primary"
                  : "border-transparent text-muted-foreground hover:text-foreground"
              }`}
            >
              <Robot size={14} />
              {t("documents:viewer.sidebarAI")}
            </button>
            <button
              type="button"
              onClick={onClose}
              className="mx-1 flex h-7 w-7 items-center justify-center rounded text-muted-foreground hover:bg-muted hover:text-foreground transition-colors"
              aria-label={t("ai:viewer.close")}
            >
              <X size={14} />
            </button>
          </div>

          {/* Content */}
          <div className="flex-1 overflow-hidden">
            {activeTab === "documents" ? (
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
            ) : (
              <SidebarAIChat documentId={activeDocumentId} />
            )}
          </div>
        </motion.aside>
      )}
    </AnimatePresence>
  );
}
