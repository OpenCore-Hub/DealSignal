import { useEffect, useState } from "react";
import { useNavigate, useParams } from "react-router";
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
import { api } from "@/lib/api";
import type { Workspace } from "@/types";

export function WorkspaceSwitcher() {
  const navigate = useNavigate();
  const { workspaceSlug } = useParams<{ workspaceSlug: string }>();
  const { currentWorkspace, setCurrentWorkspace } = useUIStore();
  const [open, setOpen] = useState(false);
  const [workspaces, setWorkspaces] = useState<Workspace[]>([]);

  useEffect(() => {
    let cancelled = false;
    api.getWorkspaces().then((res) => {
      if (!cancelled) setWorkspaces(res.data);
    });
    return () => {
      cancelled = true;
    };
  }, []);

  const activeWorkspace =
    currentWorkspace ||
    workspaces.find((w) => w.slug === workspaceSlug) ||
    workspaces[0];

  if (!activeWorkspace) {
    return (
      <div className="flex items-center gap-3 rounded-md px-2 py-2 text-muted-foreground">
        <Buildings size={20} />
        <span className="text-sm">加载中…</span>
      </div>
    );
  }

  return (
    <DropdownMenu open={open} onOpenChange={setOpen}>
      <DropdownMenuTrigger
        className="flex w-full items-center gap-3 rounded-md px-2 py-2 text-left transition-colors hover:bg-muted focus-visible:ring-2 focus-visible:ring-ring"
        aria-label="切换工作区"
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
          工作区
        </DropdownMenuLabel>
        {workspaces.map((workspace) => (
          <DropdownMenuItem
            key={workspace.id}
            className="gap-2"
            onClick={() => {
              setCurrentWorkspace(workspace);
              navigate(`/${workspace.slug}/dashboard`);
            }}
          >
            <Buildings size={16} />
            <span className="flex-1">{workspace.name}</span>
            {workspace.id === activeWorkspace.id && (
              <Check size={16} weight="bold" />
            )}
          </DropdownMenuItem>
        ))}
        <DropdownMenuSeparator />
        <DropdownMenuItem
          className="gap-2"
          onClick={() => {}}
          disabled
          title="创建工作区需后端支持"
        >
          <Plus size={16} />
          <span>创建工作区</span>
        </DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
  );
}
