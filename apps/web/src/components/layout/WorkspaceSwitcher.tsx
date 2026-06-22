import { useEffect, useState } from "react";
import { useNavigate, useParams } from "react-router";
import { Check, Buildings, Plus } from "@phosphor-icons/react";
import { useTranslation } from "react-i18next";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuGroup,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { useUIStore } from "@/stores/uiStore";
import { api } from "@/lib/api";
import { toast } from "sonner";
import type { Workspace } from "@/types";

export function WorkspaceSwitcher() {
  const { t: tLayout } = useTranslation("layout");
  const { t: tCommon } = useTranslation("common");
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

  useEffect(() => {
    if (activeWorkspace && activeWorkspace.id !== currentWorkspace?.id) {
      setCurrentWorkspace(activeWorkspace);
    }
  }, [activeWorkspace, currentWorkspace, setCurrentWorkspace]);

  if (!activeWorkspace) {
    return (
      <div className="flex items-center gap-3 rounded-md px-2 py-2 text-muted-foreground">
        <Buildings size={20} />
        <span className="text-sm">{tCommon("loading")}</span>
      </div>
    );
  }

  return (
    <DropdownMenu open={open} onOpenChange={setOpen}>
      <DropdownMenuTrigger
        className="flex w-full items-center gap-3 rounded-md px-2 py-2 text-left transition-colors hover:bg-muted focus-visible:ring-2 focus-visible:ring-ring"
        aria-label={tLayout("workspaceSwitcher.switchWorkspace")}
      >
        <div className="flex h-8 w-8 shrink-0 items-center justify-center rounded-md bg-primary text-primary-foreground text-sm font-bold">
          {tCommon(activeWorkspace.name).slice(0, 1)}
        </div>
        <div className="flex-1 overflow-hidden">
          <p className="truncate text-sm font-medium">{tCommon(activeWorkspace.name)}</p>
          <p className="truncate text-caption text-muted-foreground">
            {activeWorkspace.slug}
          </p>
        </div>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="start" className="w-56">
        <DropdownMenuGroup>
          <DropdownMenuLabel className="text-caption text-muted-foreground">
            {tLayout("workspaceSwitcher.label")}
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
              <span className="flex-1">{tCommon(workspace.name)}</span>
              {workspace.id === activeWorkspace.id && (
                <Check size={16} weight="bold" />
              )}
            </DropdownMenuItem>
          ))}
        </DropdownMenuGroup>
        <DropdownMenuSeparator />
        <DropdownMenuItem
          className="gap-2"
          onClick={() =>
            toast.info(tLayout("workspaceSwitcher.createWorkspaceComingSoon"))
          }
        >
          <Plus size={16} />
          <span>{tLayout("workspaceSwitcher.createWorkspace")}</span>
        </DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
  );
}
