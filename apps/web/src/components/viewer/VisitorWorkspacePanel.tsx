import { useEffect, useMemo, useState } from "react";
import { AnimatePresence, motion } from "motion/react";
import {
  CaretRight,
  ChatCenteredDots,
  FileText,
  Folder,
  Question,
} from "@phosphor-icons/react";
import { useTranslation } from "react-i18next";
import { useVisitorPinnedFAQs } from "@/hooks/useVisitorPinnedFAQs";
import { FileRequestPanel } from "./FileRequestPanel";
import { UnifiedQAPanel } from "./UnifiedQAPanel";
import { VisitorFaqPanel } from "./VisitorFaqPanel";
import {
  groupDocumentsByFolder,
  shouldGroupDocumentsByFolder,
} from "./rightSidebarUtils";
import {
  defaultVisitorWorkspaceTab,
  visitorWorkspaceTabs,
  type VisitorWorkspaceTab,
} from "./visitorWorkspace";
import { cn } from "@/lib/utils";
import type { DealRoomKnowledgeQueryHit } from "@/types";

interface DocSummary {
  id: string;
  title: string;
  pageCount: number;
  folderPath?: string;
}

interface VisitorWorkspacePanelProps {
  open: boolean;
  documents?: DocSummary[];
  selectedDocIndex?: number;
  onSelectDoc?: (index: number) => void;
  qaEnabled?: boolean;
  fileRequestsEnabled?: boolean;
  publicToken?: string;
  publicSessionToken?: string;
  onOpenCitation?: (hit: DealRoomKnowledgeQueryHit) => void;
}

export function VisitorWorkspacePanel({
  open,
  documents,
  selectedDocIndex = 0,
  onSelectDoc,
  qaEnabled,
  fileRequestsEnabled,
  publicToken,
  publicSessionToken,
  onOpenCitation,
}: VisitorWorkspacePanelProps) {
  const { t } = useTranslation("documents");
  const hasFolderStructure = shouldGroupDocumentsByFolder(documents ?? []);
  const documentCount = documents?.length ?? 0;
  const hasMultipleDocuments = documentCount > 1;
  const showDocumentsTab = hasMultipleDocuments;
  const showAskTab = Boolean(qaEnabled);
  const showRequestsTab = Boolean(fileRequestsEnabled);
  const { faqs: pinnedFaqs } = useVisitorPinnedFAQs({
    token: publicToken ?? "",
    sessionToken: publicSessionToken,
    qaEnabled: showAskTab && Boolean(publicToken),
  });
  const showFaqTab = showAskTab && pinnedFaqs.length > 0;
  const [pendingAsk, setPendingAsk] = useState<{ question: string; submit: boolean } | undefined>();

  const visibleTabs = useMemo(
    () =>
      visitorWorkspaceTabs({
        documentCount,
        fileRequestsEnabled: showRequestsTab,
        qaEnabled: showAskTab,
        faqCount: pinnedFaqs.length,
      }),
    [documentCount, pinnedFaqs.length, showAskTab, showRequestsTab],
  );

  const [activeTab, setActiveTab] = useState<VisitorWorkspaceTab>(() =>
    defaultVisitorWorkspaceTab({
      fileRequestsEnabled: showRequestsTab,
      hasMultipleDocuments,
      qaEnabled: showAskTab,
    }),
  );

  const folderGroups = useMemo(
    () =>
      documents && documents.length > 0
        ? groupDocumentsByFolder(documents, t("viewer.folderRoot"))
        : [],
    [documents, t],
  );

  const [expandedFolders, setExpandedFolders] = useState<Set<string>>(() => {
    if (!documents?.length) return new Set<string>();
    return new Set([documents[selectedDocIndex]?.folderPath ?? "/"]);
  });

  useEffect(() => {
    if (!hasFolderStructure || !documents?.length) return;
    const selectedPath = documents[selectedDocIndex]?.folderPath ?? "/";
    setExpandedFolders((prev) => {
      if (prev.has(selectedPath)) return prev;
      const next = new Set(prev);
      next.add(selectedPath);
      return next;
    });
  }, [documents, hasFolderStructure, selectedDocIndex]);

  useEffect(() => {
    if (visibleTabs.includes(activeTab)) return;
    setActiveTab(visibleTabs[0] ?? "documents");
  }, [activeTab, visibleTabs]);

  const tabs: Array<{
    id: VisitorWorkspaceTab;
    label: string;
    icon: typeof FileText;
  }> = [
    showDocumentsTab
      ? { id: "documents", label: t("viewer.sidebarDocuments"), icon: FileText }
      : null,
    showAskTab
      ? { id: "qa", label: t("viewer.sidebarQA"), icon: ChatCenteredDots }
      : null,
    showFaqTab
      ? { id: "faq", label: t("viewer.sidebarFAQ"), icon: Question }
      : null,
    showRequestsTab
      ? { id: "requests", label: t("viewer.sidebarRequests"), icon: FileText }
      : null,
  ].filter(Boolean) as Array<{
    id: VisitorWorkspaceTab;
    label: string;
    icon: typeof FileText;
  }>;

  const renderDocButton = (doc: DocSummary, index: number, indented: boolean) => {
    const isSelected = index === selectedDocIndex;
    return (
      <button
        key={doc.id}
        type="button"
        onClick={() => onSelectDoc?.(index)}
        className={cn(
          "flex w-full items-center gap-3 rounded-xl py-2.5 text-left transition-colors hover:bg-background/70",
          indented ? "px-3 pl-8" : "px-3",
          isSelected
            ? "bg-emerald-500/8 ring-1 ring-emerald-500/20"
            : "ring-1 ring-transparent",
        )}
      >
        <span
          className={cn(
            "flex h-9 w-9 shrink-0 items-center justify-center rounded-lg transition-colors",
            isSelected
              ? "bg-emerald-500/12 text-emerald-700 dark:text-emerald-300"
              : "bg-muted/50 text-muted-foreground",
          )}
        >
          <FileText size={18} weight={isSelected ? "fill" : "regular"} />
        </span>
        <div className="min-w-0 flex-1">
          <p
            className={cn(
              "truncate text-sm font-medium leading-snug",
              isSelected ? "text-foreground" : "text-foreground/90",
            )}
          >
            {doc.title}
          </p>
          <p className="mt-0.5 text-xs tabular-nums text-muted-foreground">
            {t("viewer.pageCountShort", { count: doc.pageCount })}
          </p>
        </div>
      </button>
    );
  };

  if (tabs.length === 0) {
    return null;
  }

  return (
    <AnimatePresence>
      {open ? (
        <motion.aside
          initial={{ width: 0, opacity: 0 }}
          animate={{ width: 380, opacity: 1 }}
          exit={{ width: 0, opacity: 0 }}
          transition={{ duration: 0.24, ease: [0.16, 1, 0.3, 1] }}
          className="public-viewer-glass mr-3 mb-3 flex h-[calc(100%-0.75rem)] min-h-0 shrink-0 flex-col overflow-hidden rounded-2xl sm:mr-4"
          style={{ minWidth: open ? 380 : 0 }}
        >
          <div className="border-b border-border/60 px-4 py-3.5">
            <p className="text-base font-semibold tracking-tight">{t("viewer.workspaceTitle")}</p>
            <p className="mt-0.5 text-sm leading-snug text-muted-foreground">
              {t("viewer.workspaceSubtitle")}
            </p>
          </div>

          {tabs.length > 1 ? (
            <div className="border-b border-border/60 px-3 py-2.5">
              <div className="flex gap-1.5 overflow-x-auto scrollbar-hide">
                {tabs.map((tab) => {
                  const Icon = tab.icon;
                  const isActive = activeTab === tab.id;
                  return (
                    <button
                      key={tab.id}
                      type="button"
                      onClick={() => setActiveTab(tab.id)}
                      className={cn(
                        "inline-flex shrink-0 items-center gap-2 rounded-full px-3.5 py-2 text-sm font-medium transition-all",
                        isActive
                          ? "bg-emerald-500/10 text-emerald-800 shadow-sm ring-1 ring-emerald-500/25 dark:text-emerald-200"
                          : "bg-background/70 text-muted-foreground hover:bg-background hover:text-foreground",
                      )}
                    >
                      <Icon size={16} weight={isActive ? "fill" : "duotone"} />
                      {tab.label}
                    </button>
                  );
                })}
              </div>
            </div>
          ) : null}

          <div className="flex min-h-0 flex-1 flex-col overflow-hidden">
            {activeTab === "qa" && showAskTab && publicToken ? (
              <div className="flex min-h-0 flex-1 flex-col overflow-hidden">
                <UnifiedQAPanel
                  token={publicToken}
                  sessionToken={publicSessionToken}
                  qaEnabled={showAskTab}
                  onOpenCitation={onOpenCitation}
                  pendingQuestion={pendingAsk?.question}
                  pendingSubmit={pendingAsk?.submit}
                  onPendingQuestionConsumed={() => setPendingAsk(undefined)}
                />
              </div>
            ) : null}
            {activeTab === "faq" && showFaqTab ? (
              <div className="flex min-h-0 flex-1 flex-col overflow-hidden">
                <VisitorFaqPanel
                  faqs={pinnedFaqs}
                  onOpenCitation={onOpenCitation}
                  onAskQuestion={(question) => {
                    setPendingAsk({ question, submit: false });
                    setActiveTab("qa");
                  }}
                  onAskThis={(question) => {
                    setPendingAsk({ question, submit: true });
                    setActiveTab("qa");
                  }}
                />
              </div>
            ) : null}
            {activeTab === "requests" && showRequestsTab && publicToken ? (
              <div className="flex min-h-0 flex-1 flex-col overflow-hidden">
                <FileRequestPanel token={publicToken} sessionToken={publicSessionToken} />
              </div>
            ) : null}
            {activeTab === "documents" && showDocumentsTab ? (
              <div className="min-h-0 flex-1 overflow-y-auto overscroll-contain p-2.5">
                {hasFolderStructure ? (
                  folderGroups.map((folder) => {
                    const isOpen = expandedFolders.has(folder.path);
                    return (
                      <div key={folder.path} className="mb-1">
                        <button
                          type="button"
                          onClick={() =>
                            setExpandedFolders((prev) => {
                              const next = new Set(prev);
                              if (next.has(folder.path)) next.delete(folder.path);
                              else next.add(folder.path);
                              return next;
                            })
                          }
                          className="flex w-full items-center gap-2.5 rounded-xl px-3 py-2.5 text-left text-sm font-medium text-muted-foreground transition-colors hover:bg-background/70 hover:text-foreground"
                          aria-expanded={isOpen}
                        >
                          <Folder
                            size={18}
                            weight={isOpen ? "fill" : "duotone"}
                            className={cn(
                              "shrink-0 transition-colors",
                              isOpen ? "text-foreground/75" : "text-muted-foreground",
                            )}
                          />
                          <span className="min-w-0 flex-1 truncate">{folder.name}</span>
                          <CaretRight
                            size={14}
                            weight="bold"
                            className={cn(
                              "shrink-0 text-muted-foreground/80 transition-transform",
                              isOpen && "rotate-90",
                            )}
                          />
                        </button>
                        {isOpen
                          ? folder.items.map(({ doc, index }) => renderDocButton(doc, index, true))
                          : null}
                      </div>
                    );
                  })
                ) : (
                  documents?.map((d, i) => renderDocButton(d, i, false))
                )}
              </div>
            ) : null}
          </div>
        </motion.aside>
      ) : null}
    </AnimatePresence>
  );
}
