import { DotsThree } from "@phosphor-icons/react";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";

export interface RowAction {
  label: string;
  icon?: React.ReactNode;
  onClick: () => void;
  destructive?: boolean;
  pro?: boolean;
}

interface RowActionsProps {
  actions: RowAction[];
}

export function RowActions({ actions }: RowActionsProps) {
  return (
    <DropdownMenu>
      <DropdownMenuTrigger
        className="flex h-8 w-8 items-center justify-center rounded-md text-muted-foreground transition-colors hover:bg-muted hover:text-foreground focus-visible:ring-2 focus-visible:ring-ring"
        aria-label="更多操作"
      >
        <DotsThree size={18} />
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end" className="w-48">
        {actions.map((action, index) => (
          <div key={action.label + index}>
            {index > 0 && action.destructive && <DropdownMenuSeparator />}
            <DropdownMenuItem
              onClick={action.onClick}
              className={action.destructive ? "text-error-500 focus:text-error-500" : ""}
            >
              {action.icon && <span className="mr-2">{action.icon}</span>}
              <span className="flex-1">{action.label}</span>
              {action.pro && (
                <span className="ml-2 rounded border border-border px-1.5 py-0 text-[10px] font-medium text-muted-foreground">
                  PRO
                </span>
              )}
            </DropdownMenuItem>
          </div>
        ))}
      </DropdownMenuContent>
    </DropdownMenu>
  );
}
