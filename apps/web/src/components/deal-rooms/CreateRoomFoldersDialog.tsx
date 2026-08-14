import { useEffect, useState } from "react";
import { FolderSimple, Plus } from "@phosphor-icons/react";
import { useTranslation } from "react-i18next";
import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import {
  groupCreateFolderDrafts,
  seedCreateFolderDrafts,
  toCreateFolderPayload,
  type CreateFolderDraft,
} from "@/lib/createRoomFolders";
import { cn } from "@/lib/utils";

type SeedFolder = { name: string; path?: string; description?: string };

function FolderRow({
  folder,
  editing,
  nested,
  muted,
  onToggle,
  onRename,
  onStartRename,
  onStopRename,
  renameLabel,
  namePlaceholder,
  keepLabel,
}: {
  folder: CreateFolderDraft;
  editing: boolean;
  nested?: boolean;
  muted?: boolean;
  onToggle: (checked: boolean) => void;
  onRename: (name: string) => void;
  onStartRename: () => void;
  onStopRename: () => void;
  renameLabel: string;
  namePlaceholder: string;
  keepLabel: string;
}) {
  return (
    <div
      className={cn(
        "flex items-center gap-2.5 py-2",
        nested && "pl-1",
        muted && "opacity-45",
      )}
    >
      <Checkbox
        checked={folder.selected}
        disabled={muted}
        onCheckedChange={(checked) => onToggle(checked === true)}
        aria-label={keepLabel}
      />
      <FolderSimple
        size={nested ? 16 : 18}
        weight="light"
        className={cn("shrink-0", folder.selected ? "text-foreground/40" : "text-foreground/20")}
        aria-hidden
      />
      {editing ? (
        <Input
          autoFocus
          value={folder.name}
          aria-label={namePlaceholder}
          placeholder={namePlaceholder}
          onChange={(event) => onRename(event.target.value)}
          onBlur={onStopRename}
          onKeyDown={(event) => {
            if (event.key === "Enter" || event.key === "Escape") {
              event.preventDefault();
              onStopRename();
            }
          }}
          className="h-8 border-border/50 bg-transparent px-2 text-[13.5px] shadow-none"
        />
      ) : (
        <span
          className={cn(
            "min-w-0 flex-1 truncate text-[13.5px] tracking-[-0.02em]",
            !folder.selected && "text-muted-foreground/70",
          )}
        >
          {folder.name || namePlaceholder}
        </span>
      )}
      {!editing ? (
        <button
          type="button"
          onClick={onStartRename}
          className="shrink-0 text-[11px] font-medium tracking-[0.08em] text-muted-foreground/80 uppercase transition-colors hover:text-foreground"
        >
          {renameLabel}
        </button>
      ) : null}
    </div>
  );
}

export function CreateRoomFoldersDialog({
  open,
  onOpenChange,
  templateName,
  folders,
  creating,
  onConfirm,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  templateName: string;
  folders: SeedFolder[];
  creating: boolean;
  onConfirm: (folders: { name: string; path: string; description?: string }[]) => void;
}) {
  const { t } = useTranslation("dealRooms");
  const [drafts, setDrafts] = useState<CreateFolderDraft[]>([]);
  const [editingId, setEditingId] = useState<string | null>(null);
  const seedKey = folders.map((folder) => `${folder.path ?? ""}:${folder.name}`).join("|");

  useEffect(() => {
    if (!open) return;
    const next = seedCreateFolderDrafts(folders);
    setDrafts(next);
    setEditingId(next.find((item) => !item.name)?.id ?? null);
  }, [open, seedKey, folders]);

  const groups = groupCreateFolderDrafts(drafts);
  const selectedCount = toCreateFolderPayload(drafts).length;

  const updateDraft = (id: string, patch: Partial<CreateFolderDraft>) => {
    setDrafts((prev) => prev.map((item) => (item.id === id ? { ...item, ...patch } : item)));
  };

  const addFolder = (parentId: string | null) => {
    const id = `added-${Date.now()}`;
    setDrafts((prev) => {
      const next = [...prev];
      const entry: CreateFolderDraft = { id, name: "", path: "", selected: true, parentId };
      if (!parentId) {
        next.push(entry);
        return next;
      }
      const lastChild = [...next].reverse().find((item) => item.parentId === parentId);
      const parentIndex = next.findIndex((item) => item.id === parentId);
      const insertAt = lastChild
        ? next.findIndex((item) => item.id === lastChild.id) + 1
        : parentIndex + 1;
      next.splice(insertAt, 0, entry);
      return next.map((item) => (item.id === parentId ? { ...item, selected: true } : item));
    });
    setEditingId(id);
  };

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent
        className="gap-0 overflow-hidden p-0 sm:max-w-[34rem]"
        showCloseButton={false}
      >
        <DialogHeader className="items-start gap-2 px-5 pt-5 pb-4">
          <DialogTitle className="text-[1.05rem] font-medium tracking-[-0.03em]">
            {t("new.foldersDialog.title")}
          </DialogTitle>
          <DialogDescription className="max-w-[28rem] border-l border-foreground/12 pl-3 text-[12.5px] leading-[1.55] tracking-[-0.011em] text-muted-foreground">
            {t("new.foldersDialog.subtitle", { name: templateName })}
          </DialogDescription>
        </DialogHeader>

        <div className="max-h-[min(26rem,52vh)] overflow-y-auto border-y border-foreground/[0.05] px-4">
          {groups.length === 0 ? (
            <p className="py-8 text-[13px] leading-relaxed text-muted-foreground">
              {t("new.foldersDialog.empty")}
            </p>
          ) : (
            <ul className="divide-y divide-foreground/[0.045]">
              {groups.map(({ root, children }) => (
                <li key={root.id} className="py-1.5">
                  <FolderRow
                    folder={root}
                    editing={editingId === root.id}
                    onToggle={(checked) => updateDraft(root.id, { selected: checked })}
                    onRename={(name) => updateDraft(root.id, { name, path: "", description: "" })}
                    onStartRename={() => setEditingId(root.id)}
                    onStopRename={() => setEditingId(null)}
                    renameLabel={t("new.foldersDialog.rename")}
                    namePlaceholder={t("new.foldersDialog.addPlaceholder")}
                    keepLabel={t("new.foldersDialog.select", { name: root.name })}
                  />
                  {root.selected ? (
                    <div className="ml-[1.65rem] border-l border-foreground/[0.08] pl-3">
                      {children.map((child) => (
                        <FolderRow
                          key={child.id}
                          folder={child}
                          nested
                          editing={editingId === child.id}
                          onToggle={(checked) => updateDraft(child.id, { selected: checked })}
                          onRename={(name) => updateDraft(child.id, { name, path: "", description: "" })}
                          onStartRename={() => setEditingId(child.id)}
                          onStopRename={() => setEditingId(null)}
                          renameLabel={t("new.foldersDialog.rename")}
                          namePlaceholder={t("new.foldersDialog.addPlaceholder")}
                          keepLabel={t("new.foldersDialog.select", { name: child.name })}
                        />
                      ))}
                      <button
                        type="button"
                        onClick={() => addFolder(root.id)}
                        className="mb-1 flex items-center gap-1.5 py-1.5 text-[12px] tracking-[-0.01em] text-muted-foreground transition-colors hover:text-foreground"
                      >
                        <Plus size={13} weight="light" aria-hidden />
                        {t("new.foldersDialog.addSubfolder")}
                      </button>
                    </div>
                  ) : null}
                </li>
              ))}
            </ul>
          )}
        </div>

        <div className="px-4 py-3">
          <button
            type="button"
            onClick={() => addFolder(null)}
            className="flex w-full items-center gap-2.5 rounded-lg border border-dashed border-foreground/15 px-3 py-2.5 text-left text-[13px] text-muted-foreground transition-colors hover:border-foreground/30 hover:bg-foreground/[0.02] hover:text-foreground"
          >
            <Plus size={16} weight="light" aria-hidden />
            {t("new.foldersDialog.add")}
          </button>
        </div>

        <DialogFooter className="mb-0 px-5 pt-4 pb-6 sm:justify-center sm:gap-8">
          <Button
            type="button"
            variant="outline"
            className="h-10 min-w-[6.5rem] bg-background"
            onClick={() => onOpenChange(false)}
            disabled={creating}
          >
            {t("new.cancel")}
          </Button>
          <Button
            type="button"
            data-testid="confirm-create-room"
            className="h-10 min-w-[8.5rem]"
            disabled={creating || selectedCount === 0}
            onClick={() => onConfirm(toCreateFolderPayload(drafts))}
          >
            {creating ? t("new.creating") : t("new.foldersDialog.create")}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
