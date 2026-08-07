import { useEffect, useMemo, useState } from "react";
import { AnimatePresence, motion } from "motion/react";
import {
  CaretRight,
  ChatCenteredDots,
  FileText,
  Folder,
  X,
} from "@phosphor-icons/react";
import { useTranslation } from "react-i18next";
import { FileRequestPanel } from "./FileRequestPanel";
import { UnifiedQAPanel } from "./UnifiedQAPanel";
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

interface DocSummary {
  id: string;
  title: string;
  pageCount: number;
  folderPath?: string;
}

interface VisitorWorkspacePanelProps {
  open: boolean;
  onClose: () => void;
  documents?: DocSummary[];
  selectedDocIndex?: number;
  onSelectDoc?: (index: number) => void;
  qaEnabled?: boolean;
  fileRequestsEnabled?: boolean;
  publicToken?: string;
  publicSessionToken?: string;
}

export function VisitorWorkspacePanel({
  open,
  onClose,
  documents,
  selectedDocIndex = 0,
  onSelectDoc,
  qaEnabled,
  fileRequestsEnabled,
  publicToken,
  publicSessionToken,
}: VisitorWorkspacePanelProps) {
  const { t } = useTranslation("documents");
  const hasFolderStructure = shouldGroupDocumentsByFolder(documents ?? []);
  const documentCount = documents?.length ?? 0;
  const hasMultipleDocuments = documentCount > 1;
  const showDocumentsTab = hasMultipleDocuments;
  const showAskTab = Boolean(qaEnabled);
  const showRequestsTab = Boolean(fileRequestsEnabled);

  const visibleTabs = useMemo(
    () =>
      visitorWorkspaceTabs({
        documentCount,
        fileRequestsEnabled: showRequestsTab,
        qaEnabled: showAskTab,
      }),
    [documentCount, showAskTab, showRequestsTab],
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
    showRequestsTab
      ? { id: "requests", label: t("viewer.sidebarRequests"), icon: FileText }
      : null,
  ].filter(Boolean) as Array<{
    id: VisitorWorkspaceTab;
    label: string;
    icon: typeof FileText;
  }>;

  const renderDocButton = (doc: DocSummary, index: number, indented: boolean) => (
    <button
      key={doc.id}
      type="button"
      onClick={() => onSelectDoc?.(index)}
      className={cn(
        "flex w-full items-start gap-3 rounded-xl py-2.5 text-left transition-colors hover:bg-background/70",
        indented ? "px-3 pl-7" : "px-3",
        index === selectedDocIndex
          ? "bg-emerald-500/8 ring-1 ring-emerald-500/20"
          : "ring-1 ring-transparent",
      )}
    >
      <FileText
        size={16}
        className={cn(
          "mt-0.5 shrink-0",
          index === selectedDocIndex ? "text-emerald-700 dark:text-emerald-300" : "text-muted-foreground",
        )}
      />
      <div className="min-w-0 flex-1">
        <p
          className={cn(
            "truncate text-sm font-medium",
            index === selectedDocIndex ? "text-foreground" : "text-foreground/90",
          )}
        >
          {doc.title}
        </p>
        <p className="text-xs text-muted-foreground">
          {t("viewer.pageCountShort", { count: doc.pageCount })}
        </p>
      </div>
    </button>
  );

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
          className="public-viewer-glass mr-3 mb-3 flex h-[calc(100%-0.75rem)] shrink-0 flex-col overflow-hidden rounded-2xl sm:mr-4"
          style={{ minWidth: open ? 380 : 0 }}
        >
          <div className="flex items-center justify-between border-b border-border/60 px-4 py-3">
            <div>
              <p className="text-sm font-semibold tracking-tight">{t("viewer.workspaceTitle")}</p>
              <p className="text-xs text-muted-foreground">{t("viewer.workspaceSubtitle")}</p>
            </div>
            <button
              type="button"
              onClick={onClose}
              className="flex h-8 w-8 items-center justify-center rounded-xl text-muted-foreground transition-colors hover:bg-background/70 hover:text-foreground"
              aria-label={t("viewer.sidebarClose")}
            >
              <X size={16} />
            </button>
          </div>

          {tabs.length > 1 ? (
            <div className="border-b border-border/60 px-3 py-2">
              <div className="flex gap-1 overflow-x-auto scrollbar-hide">
                {tabs.map((tab) => {
                  const Icon = tab.icon;
                  const isActive = activeTab === tab.id;
                  return (
                    <button
                      key={tab.id}
                      type="button"
                      onClick={() => setActiveTab(tab.id)}
                      className={cn(
                        "inline-flex shrink-0 items-center gap-1.5 rounded-full px-3 py-1.5 text-xs font-medium transition-all",
                        isActive
                          ? "bg-foreground text-background shadow-sm"
                          : "bg-background/70 text-muted-foreground hover:text-foreground",
                      )}
                    >
                      <Icon size={13} weight={isActive ? "fill" : "regular"} />
                      {tab.label}
                    </button>
                  );
                })}
              </div>
            </div>
          ) : null}

          <div className="min-h-0 flex-1 overflow-hidden">
            {activeTab === "qa" && showAskTab && publicToken ? (
              <UnifiedQAPanel
                token={publicToken}
                sessionToken={publicSessionToken}
                qaEnabled={showAskTab}
              />
            ) : null}
            {activeTab === "requests" && showRequestsTab && publicToken ? (
              <FileRequestPanel token={publicToken} sessionToken={publicSessionToken} />
            ) : null}
            {activeTab === "documents" && showDocumentsTab ? (
              <div className="h-full overflow-y-auto p-2">
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
                          className="flex w-full items-center gap-2 rounded-xl px-3 py-2 text-left text-xs font-semibold text-muted-foreground transition-colors hover:bg-background/70 hover:text-foreground"
                          aria-expanded={isOpen}
                        >
                          <Folder size={14} className="shrink-0" />
                          <span className="min-w-0 flex-1 truncate">{folder.name}</span>
                          <CaretRight
                            size={12}
                            className={cn("shrink-0 transition-transform", isOpen && "rotate-90")}
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
