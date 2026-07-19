import { DocumentScopeSection } from "./DocumentScopeSection";
import type { DealRoomFolder, DealRoomFolderDocs } from "@/types";

interface DocumentsTabProps {
  folders: DealRoomFolder[];
  documents: DealRoomFolderDocs[];
  selectedPaths: string[];
  onChange: (paths: string[]) => void;
  disabled?: boolean;
}

export function DocumentsTab({
  folders,
  documents,
  selectedPaths,
  onChange,
  disabled,
}: DocumentsTabProps) {
  return (
    <DocumentScopeSection
      folders={folders}
      documents={documents}
      selectedPaths={selectedPaths}
      onChange={onChange}
      disabled={disabled}
    />
  );
}
