import { DocumentScopeSection } from "./DocumentScopeSection";
import type { FolderScopeMode } from "./types";
import type { DealRoomFolder, DealRoomFolderDocs } from "@/types";

interface DocumentsTabProps {
  folders: DealRoomFolder[];
  documents: DealRoomFolderDocs[];
  selectedPaths: string[];
  scopeMode: FolderScopeMode;
  onChange: (next: { scopeMode: FolderScopeMode; selectedPaths: string[] }) => void;
  disabled?: boolean;
}

export function DocumentsTab({
  folders,
  documents,
  selectedPaths,
  scopeMode,
  onChange,
  disabled,
}: DocumentsTabProps) {
  return (
    <DocumentScopeSection
      folders={folders}
      documents={documents}
      selectedPaths={selectedPaths}
      scopeMode={scopeMode}
      onChange={onChange}
      disabled={disabled}
    />
  );
}
