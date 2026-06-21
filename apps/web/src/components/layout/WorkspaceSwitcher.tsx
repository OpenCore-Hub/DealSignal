import { useState } from "react";
import { useParams } from "react-router";
import { Check, Buildings, Plus } from "@phosphor-icons/react";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { useUIStore } from "@/stores/uiStore";
import { mockWorkspaces } from "@/lib/mocks/data";

export function WorkspaceSwitcher() {
  const { workspaceSlug } = useParams<{ workspaceSlug: string }>();
  const { currentWorkspace, setCurrentWorkspace } = useUIStore();
  const [open, setOpen] = useState(false);

  const activeWorkspace =
    currentWorkspace ||
    mockWorkspaces.find((w) => w.slug === workspaceSlug) ||
    mockWorkspaces[0];

  return (
    <DropdownMenu open={open} onOpenChange={setOpen}>
      <DropdownMenuTrigger
        className="flex w-full items-center gap-3 rounded-md px-2 py-2 text-left transition-colors hover:bg-muted focus-visible:ring-2 focus-visible:ring-ring"
        aria-label="切换 Workspace"
      >
        <div className="flex h-8 w-8 shrink-0 items-center justify-center rounded-md bg-primary text-primary-foreground text-sm font-bold">
          {activeWorkspace.name.slice(0, 1)}
        </div>
        <div className="flex-1 overflow-hidden">
          <p className="truncate text-sm font-medium">{activeWorkspace.name}</p>
          <p className="truncate text-caption text-muted-foreground">
            {activeWorkspace.slug}
          </p>
        </div>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="start" className="w-56">
        <DropdownMenuLabel className="text-caption text-muted-foreground">
          Workspace
        </DropdownMenuLabel>
        {mockWorkspaces.map((workspace) => (
          <DropdownMenuItem
            key={workspace.id}
            className="gap-2"
            onClick={() => setCurrentWorkspace(workspace)}
          >
            <Buildings size={16} />
            <span className="flex-1">{workspace.name}</span>
            {workspace.id === activeWorkspace.id && (
              <Check size={16} weight="bold" />
            )}
          </DropdownMenuItem>
        ))}
        <DropdownMenuSeparator />
        <DropdownMenuItem className="gap-2">
          <Plus size={16} />
          <span>创建 Workspace</span>
        </DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
  );
}
