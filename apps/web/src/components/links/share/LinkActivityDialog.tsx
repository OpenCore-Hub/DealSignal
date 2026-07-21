import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import {
  ArrowsIn,
  ArrowsOut,
  MagnifyingGlassMinus,
  MagnifyingGlassPlus,
} from "@phosphor-icons/react";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogTitle,
  DialogTrigger,
} from "@/components/ui/dialog";
import { cn } from "@/lib/utils";
import { AnalyticsTab } from "./AnalyticsTab";
import type { Link } from "@/types";

interface LinkActivityDialogProps {
  link: Link;
  children?: React.ReactElement;
  open?: boolean;
  onOpenChange?: (open: boolean) => void;
}

/** Window size steps for the activity dialog (not fullscreen). */
export type ActivityWindowSize = "sm" | "md" | "lg";

const SIZE_STEPS: ActivityWindowSize[] = ["sm", "md", "lg"];

const SIZE_CLASS: Record<ActivityWindowSize | "full", string> = {
  sm: "sm:max-w-xl max-h-[75vh]",
  md: "sm:max-w-3xl max-h-[90vh]",
  lg: "sm:max-w-5xl max-h-[94vh]",
  full: [
    "!top-3 !left-3 !right-3 !bottom-3",
    "!translate-x-0 !translate-y-0",
    "!w-auto !max-w-none !h-auto !max-h-none",
  ].join(" "),
};

export function LinkActivityDialog({
  link,
  children,
  open: openProp,
  onOpenChange,
}: LinkActivityDialogProps) {
  const { t } = useTranslation("linkShare");
  const [openState, setOpenState] = useState(false);
  const open = openProp ?? openState;
  const setOpen = (value: boolean) => {
    setOpenState(value);
    onOpenChange?.(value);
  };
  const [size, setSize] = useState<ActivityWindowSize>("md");
  const [fullscreen, setFullscreen] = useState(false);

  useEffect(() => {
    if (!open) {
      setSize("md");
      setFullscreen(false);
    }
  }, [open]);

  const sizeIndex = SIZE_STEPS.indexOf(size);
  const canShrink = !fullscreen && sizeIndex > 0;
  const canEnlarge = !fullscreen && sizeIndex < SIZE_STEPS.length - 1;

  const shrink = () => {
    if (!canShrink) return;
    setSize(SIZE_STEPS[sizeIndex - 1]);
  };

  const enlarge = () => {
    if (!canEnlarge) return;
    setSize(SIZE_STEPS[sizeIndex + 1]);
  };

  const toggleFullscreen = () => {
    setFullscreen((v) => !v);
  };

  return (
    <Dialog open={open} onOpenChange={setOpen}>
      {children && <DialogTrigger render={children} />}
      <DialogContent
        className={cn(
          "flex flex-col",
          fullscreen ? SIZE_CLASS.full : SIZE_CLASS[size],
        )}
      >
        <DialogTitle className="sr-only">{t("activity.title")}</DialogTitle>

        <div
          className="absolute top-2 right-12 flex items-center gap-0.5"
          role="toolbar"
          aria-label={t("activity.windowControls")}
        >
          <Button
            type="button"
            variant="ghost"
            size="icon-sm"
            disabled={!canShrink}
            onClick={shrink}
            aria-label={t("activity.shrink")}
            title={t("activity.shrink")}
          >
            <MagnifyingGlassMinus size={16} />
          </Button>
          <Button
            type="button"
            variant="ghost"
            size="icon-sm"
            disabled={!canEnlarge}
            onClick={enlarge}
            aria-label={t("activity.enlarge")}
            title={t("activity.enlarge")}
          >
            <MagnifyingGlassPlus size={16} />
          </Button>
          <Button
            type="button"
            variant="ghost"
            size="icon-sm"
            onClick={toggleFullscreen}
            aria-label={
              fullscreen ? t("activity.exitFullscreen") : t("activity.fullscreen")
            }
            title={
              fullscreen ? t("activity.exitFullscreen") : t("activity.fullscreen")
            }
            aria-pressed={fullscreen}
          >
            {fullscreen ? <ArrowsIn size={16} /> : <ArrowsOut size={16} />}
          </Button>
        </div>

        <div className="min-h-0 flex-1 overflow-y-auto py-2">
          {open ? <AnalyticsTab link={link} logs={[]} /> : null}
        </div>
      </DialogContent>
    </Dialog>
  );
}
