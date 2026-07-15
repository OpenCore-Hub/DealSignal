import { useMemo, useState } from "react";
import { FileText, Plus, MagnifyingGlass } from "@phosphor-icons/react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { Checkbox } from "@/components/ui/checkbox";
import { useTranslation } from "react-i18next";
import type { DealRoomDocumentItem, DealRoomFolder, Document } from "@/types";

function parentPath(path: string): string | null {
  if (path === "/") return null;
  const idx = path.lastIndexOf("/");
  if (idx <= 0) return "/";
  return path.slice(0, idx);
}

interface FolderNode {
  folder: DealRoomFolder;
  children: FolderNode[];
}

function buildFolderTree(folders: DealRoomFolder[]): FolderNode[] {
  const sorted = [...folders].filter((f) => f.path !== "/");
  const map = new Map<string, FolderNode>();
  for (const f of sorted) map.set(f.path, { folder: f, children: [] });
  const roots: FolderNode[] = [];
  for (const f of sorted) {
    const node = map.get(f.path)!;
    const pp = parentPath(f.path);
    if (pp === "/" || pp === null) {
      roots.push(node);
    } else {
      const parent = map.get(pp);
      if (parent) parent.children.push(node);
      else roots.push(node);
    }
  }
  return roots;
}

function flattenTree(nodes: FolderNode[], depth: number): { folder: DealRoomFolder; depth: number }[] {
  let flat: { folder: DealRoomFolder; depth: number }[] = [];
  for (const node of nodes) {
    flat.push({ folder: node.folder, depth });
    flat = flat.concat(flattenTree(node.children, depth + 1));
  }
  return flat;
}

interface DocumentPickerProps {
  workspaceDocuments: Document[];
  roomDocuments: DealRoomDocumentItem[];
  folders: DealRoomFolder[];
  onAdd: (documentIds: string[], folderPath: string) => void;
  disabled?: boolean;
  initialFolderPath?: string;
  allowFolderChange?: boolean;
}

export function DocumentPicker({
  workspaceDocuments,
  roomDocuments,
  folders,
  onAdd,
  disabled,
  initialFolderPath,
  allowFolderChange = true,
}: DocumentPickerProps) {
  const { t } = useTranslation("dealRooms");
  const { t: tc } = useTranslation("common");
  const [search, setSearch] = useState("");
  const [selectedFolder, setSelectedFolder] = useState<string>(initialFolderPath ?? folders[0]?.path ?? "");
  const [selectedIds, setSelectedIds] = useState<Set<string>>(new Set());

  const roomDocumentIds = useMemo(() => new Set(roomDocuments.map((d) => d.document_id)), [roomDocuments]);

  const availableDocuments = useMemo(() => {
    const q = search.trim().toLowerCase();
    return workspaceDocuments
      .filter((d) => !roomDocumentIds.has(d.id))
      .filter((d) => !q || d.title.toLowerCase().includes(q) || d.fileName.toLowerCase().includes(q));
  }, [workspaceDocuments, roomDocumentIds, search]);

  const folderTree = useMemo(() => {
    const roots = buildFolderTree(folders);
    return flattenTree(roots, 0);
  }, [folders]);

  const toggleSelection = (id: string) => {
    setSelectedIds((prev) => {
      const next = new Set(prev);
      if (next.has(id)) next.delete(id);
      else next.add(id);
      return next;
    });
  };

  const handleAdd = () => {
    if (selectedIds.size === 0) return;
    onAdd(Array.from(selectedIds), selectedFolder);
    setSelectedIds(new Set());
  };

  return (
    <div className="space-y-4 rounded-lg border border-border p-4">
      <h4 className="text-sm font-medium flex items-center gap-2">
        <Plus size={16} />
        {t("documents.addFromWorkspace")}
      </h4>
      <div className="flex flex-col gap-3 sm:flex-row">
        <div className="relative flex-1">
          <MagnifyingGlass size={16} className="absolute left-2.5 top-1/2 -translate-y-1/2 text-muted-foreground" />
          <Input
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            placeholder={tc("search")}
            className="pl-8"
          />
        </div>
        {allowFolderChange && (
          <div className="flex items-center gap-2">
            <Label htmlFor="picker-folder" className="sr-only">
              {t("documents.folder")}
            </Label>
            <Select value={selectedFolder} onValueChange={(v) => v && setSelectedFolder(v)}>
              <SelectTrigger id="picker-folder" className="w-48">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                {folderTree.map(({ folder, depth }) => (
                  <SelectItem key={folder.path} value={folder.path}>
                    <span style={{ paddingLeft: `${depth * 16}px` }}>
                      {depth > 0 && "└ "}{folder.name}
                    </span>
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>
        )}
      </div>

      <div className="max-h-60 overflow-y-auto rounded-md border border-border">
        {availableDocuments.length === 0 ? (
          <p className="p-4 text-sm text-muted-foreground">{t("documents.noAvailable")}</p>
        ) : (
          <ul className="divide-y divide-border">
            {availableDocuments.map((doc) => (
              <li
                key={doc.id}
                className="flex items-center gap-3 p-3 hover:bg-muted/50"
              >
                <Checkbox
                  id={`picker-doc-${doc.id}`}
                  checked={selectedIds.has(doc.id)}
                  onCheckedChange={() => toggleSelection(doc.id)}
                  disabled={disabled}
                />
                <FileText size={18} className="text-muted-foreground" />
                <label
                  htmlFor={`picker-doc-${doc.id}`}
                  className="flex-1 cursor-pointer text-sm font-medium"
                >
                  {doc.title}
                </label>
              </li>
            ))}
          </ul>
        )}
      </div>

      <div className="flex justify-end">
        <Button
          onClick={handleAdd}
          disabled={selectedIds.size === 0 || disabled}
          className="gap-1.5"
        >
          <Plus size={16} />
          {t("documents.addSelected", { count: selectedIds.size })}
        </Button>
      </div>
    </div>
  );
}
