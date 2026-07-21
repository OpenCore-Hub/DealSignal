import { useEffect, useMemo, useState } from "react";
import { motion, AnimatePresence } from "motion/react";
import { CaretRight, ChatCenteredDots, FileText, Folder, X } from "@phosphor-icons/react";
import { useTranslation } from "react-i18next";
import { FileRequestPanel } from "./FileRequestPanel";
import { UnifiedQAPanel } from "./UnifiedQAPanel";

interface DocSummary {
  id: string;
  title: string;
  pageCount: number;
  folderPath?: string;
}

type SidebarTab = "documents" | "qa" | "requests";

interface FolderGroup {
  path: string;
  name: string;
  items: { doc: DocSummary; index: number }[];
}

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

function normalizeFolderPath(path?: string): string {
  if (!path || path === "") return "/";
  return path.length > 1 && path.endsWith("/") ? path.replace(/\/+$/, "") : path;
}

function folderDisplayName(path: string, rootLabel: string): string {
  const segments = path.split("/").filter(Boolean);
  return segments.length === 0 ? rootLabel : (segments[segments.length - 1] ?? path);
}

export function shouldGroupDocumentsByFolder(documents: DocSummary[]): boolean {
  const paths = new Set(documents.map((d) => normalizeFolderPath(d.folderPath)));
  if (paths.size > 1) return true;
  const only = [...paths][0];
  return only !== undefined && only !== "/";
}

function groupDocumentsByFolder(
  documents: DocSummary[],
  rootLabel: string
): FolderGroup[] {
  const map = new Map<string, FolderGroup>();
  documents.forEach((doc, index) => {
    const path = normalizeFolderPath(doc.folderPath);
    let group = map.get(path);
    if (!group) {
      group = { path, name: folderDisplayName(path, rootLabel), items: [] };
      map.set(path, group);
    }
    group.items.push({ doc, index });
  });
  return Array.from(map.values()).sort((a, b) => a.path.localeCompare(b.path));
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
  const hasFolderStructure = shouldGroupDocumentsByFolder(documents ?? []);
  const [activeTab, setActiveTab] = useState<SidebarTab>(
    hasFolderStructure ? "documents" : qaAvailable ? "qa" : "documents"
  );
  const hasAnyFeature = qaAvailable || fileRequestsEnabled;

  const folderGroups = useMemo(
    () =>
      documents && documents.length > 0
        ? groupDocumentsByFolder(documents, t("documents:viewer.folderRoot"))
        : [],
    [documents, t]
  );

  const [expandedFolders, setExpandedFolders] = useState<Set<string>>(() => {
    if (!documents?.length) return new Set<string>();
    return new Set([normalizeFolderPath(documents[selectedDocIndex]?.folderPath)]);
  });

  useEffect(() => {
    if (!hasFolderStructure || !documents?.length) return;
    const selectedPath = normalizeFolderPath(documents[selectedDocIndex]?.folderPath);
    setExpandedFolders((prev) => {
      if (prev.has(selectedPath)) return prev;
      const next = new Set(prev);
      next.add(selectedPath);
      return next;
    });
  }, [documents, hasFolderStructure, selectedDocIndex]);

  const toggleFolder = (path: string) => {
    setExpandedFolders((prev) => {
      const next = new Set(prev);
      if (next.has(path)) {
        next.delete(path);
      } else {
        next.add(path);
      }
      return next;
    });
  };

  const renderDocButton = (doc: DocSummary, index: number, indented: boolean) => (
    <button
      key={doc.id}
      type="button"
      onClick={() => onSelectDoc?.(index)}
      className={`flex w-full items-start gap-3 py-2.5 text-left transition-colors hover:bg-muted/50 ${
        indented ? "px-4 pl-8" : "px-4"
      } ${
        index === selectedDocIndex
          ? "bg-primary/5 border-l-2 border-l-primary"
          : "border-l-2 border-l-transparent"
      }`}
    >
      <FileText
        size={16}
        className={`mt-0.5 shrink-0 ${
          index === selectedDocIndex ? "text-primary" : "text-muted-foreground"
        }`}
      />
      <div className="min-w-0 flex-1">
        <p
          className={`text-sm font-medium truncate ${
            index === selectedDocIndex ? "text-primary" : "text-foreground"
          }`}
        >
          {doc.title}
        </p>
        <p className="text-xs text-muted-foreground">
          {doc.pageCount} {t("documents:viewer.pages")}
        </p>
      </div>
    </button>
  );

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
                ) : hasFolderStructure ? (
                  folderGroups.map((folder) => {
                    const isOpen = expandedFolders.has(folder.path);
                    return (
                      <div key={folder.path}>
                        <button
                          type="button"
                          onClick={() => toggleFolder(folder.path)}
                          className="flex w-full items-center gap-2 px-4 py-2 text-left text-xs font-semibold text-muted-foreground hover:bg-muted/40 hover:text-foreground transition-colors"
                          aria-expanded={isOpen}
                        >
                          <Folder size={14} className="shrink-0" />
                          <span className="min-w-0 flex-1 truncate">{folder.name}</span>
                          <CaretRight
                            size={12}
                            className={`shrink-0 transition-transform ${isOpen ? "rotate-90" : ""}`}
                          />
                        </button>
                        {isOpen &&
                          folder.items.map(({ doc, index }) =>
                            renderDocButton(doc, index, true)
                          )}
                      </div>
                    );
                  })
                ) : (
                  documents.map((d, i) => renderDocButton(d, i, false))
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
