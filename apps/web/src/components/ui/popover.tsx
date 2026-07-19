import * as React from "react";
import { Popover as PopoverPrimitive } from "@base-ui/react/popover";
import { cn } from "@/lib/utils";

const Popover = PopoverPrimitive.Root;
const PopoverTrigger = PopoverPrimitive.Trigger;
const PopoverPortal = PopoverPrimitive.Portal;
const PopoverClose = PopoverPrimitive.Close;
const PopoverArrow = PopoverPrimitive.Arrow;

function PopoverContent({
  className,
  children,
  side = "bottom",
  sideOffset = 8,
  align = "end",
  alignOffset = 0,
  ...props
}: React.ComponentProps<typeof PopoverPrimitive.Popup> &
  Pick<
    React.ComponentProps<typeof PopoverPrimitive.Positioner>,
    "side" | "sideOffset" | "align" | "alignOffset"
  >) {
  return (
    <PopoverPrimitive.Portal>
      <PopoverPrimitive.Positioner
        side={side}
        sideOffset={sideOffset}
        align={align}
        alignOffset={alignOffset}
      >
        <PopoverPrimitive.Popup
          className={cn(
            "z-50 w-auto rounded-lg border border-border bg-popover p-1.5 text-popover-foreground shadow-md outline-none animate-in fade-in-0 zoom-in-95 data-[side=bottom]:slide-in-from-top-2 data-[side=left]:slide-in-from-right-2 data-[side=right]:slide-in-from-left-2 data-[side=top]:slide-in-from-bottom-2 data-[state=closed]:animate-out data-[state=closed]:fade-out-0 data-[state=closed]:zoom-out-95",
            className
          )}
          {...props}
        >
          {children}
        </PopoverPrimitive.Popup>
      </PopoverPrimitive.Positioner>
    </PopoverPrimitive.Portal>
  );
}

export { Popover, PopoverTrigger, PopoverContent, PopoverPortal, PopoverClose, PopoverArrow };
