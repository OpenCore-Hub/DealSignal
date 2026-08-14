import { useTranslation } from "react-i18next";
import { motion } from "motion/react";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import { cn } from "@/lib/utils";

const enterEase = [0.32, 0.72, 0, 1] as const;

export interface DocumentExistsDialogProps {
  open: boolean;
  fileName: string;
  onOverwrite: () => void;
  onDiscard: () => void;
}

function fileExtension(name: string): string {
  const i = name.lastIndexOf(".");
  if (i <= 0 || i === name.length - 1) return "";
  return name.slice(i + 1).toUpperCase();
}

export function DocumentExistsDialog({
  open,
  fileName,
  onOverwrite,
  onDiscard,
}: DocumentExistsDialogProps) {
  const { t } = useTranslation("documents");
  const extension = fileExtension(fileName);

  return (
    <Dialog
      open={open}
      onOpenChange={(isOpen) => {
        if (!isOpen) onDiscard();
      }}
    >
      <DialogContent
        data-testid="document-exists-dialog"
        showCloseButton={false}
        className={cn(
          "z-[60] gap-0 overflow-hidden border-0 bg-transparent p-[5px] shadow-none sm:max-w-[30rem]",
          "rounded-[1.65rem]",
          "ring-1 ring-foreground/[0.06]",
          "bg-[color-mix(in_oklch,var(--muted)_72%,var(--background))]",
        )}
      >
        <div
          className={cn(
            "overflow-hidden rounded-[calc(1.65rem-5px)] bg-background",
            "shadow-[inset_0_1px_0_rgba(255,255,255,0.72)]",
            "dark:shadow-[inset_0_1px_0_rgba(255,255,255,0.06)]",
          )}
        >
          <div className="space-y-5 px-6 pb-2 pt-6 sm:px-7 sm:pt-7">
            <DialogHeader className="space-y-0 text-left">
              <motion.div
                initial={{ opacity: 0, y: 10 }}
                animate={{ opacity: 1, y: 0 }}
                transition={{ duration: 0.55, ease: enterEase }}
                className="space-y-3"
              >
                <p className="font-mono text-[10px] font-medium uppercase tracking-[0.22em] text-muted-foreground/70">
                  {t("upload.replaceEyebrow")}
                </p>
                <DialogTitle className="text-[1.35rem] font-semibold leading-[1.15] tracking-[-0.03em]">
                  {t("upload.replaceTitle")}
                </DialogTitle>
                <div
                  className={cn(
                    "flex items-start gap-2.5 rounded-[1.05rem] px-3.5 py-3",
                    "bg-[color-mix(in_oklch,var(--muted)_38%,var(--background))]",
                    "ring-1 ring-foreground/[0.05]",
                    "shadow-[inset_0_1px_0_rgba(255,255,255,0.55)]",
                    "dark:shadow-[inset_0_1px_0_rgba(255,255,255,0.04)]",
                  )}
                >
                  {extension ? (
                    <span className="mt-0.5 shrink-0 rounded-md px-1.5 py-0.5 font-mono text-[10px] tracking-[0.14em] text-muted-foreground ring-1 ring-foreground/[0.08]">
                      {extension}
                    </span>
                  ) : null}
                  <p className="min-w-0 break-words text-[13px] leading-snug text-foreground/90">
                    {fileName}
                  </p>
                </div>
                <DialogDescription className="max-w-[42ch] text-[13px] leading-relaxed text-muted-foreground">
                  {t("upload.replaceLead")}
                </DialogDescription>
              </motion.div>
            </DialogHeader>
          </div>

          <DialogFooter
            className={cn(
              "!mx-0 !mb-0 flex-row items-center justify-center gap-8",
              "border-t border-foreground/[0.06] bg-transparent",
              "px-6 py-5 sm:px-7 sm:!justify-center sm:space-x-0",
            )}
          >
            <Button
              type="button"
              variant="ghost"
              size="lg"
              onClick={onDiscard}
              className={cn(
                "h-11 min-w-[8.5rem] rounded-xl px-6 text-[13px] font-medium tracking-tight",
                "bg-rose-50 text-rose-950 ring-1 ring-rose-200/70",
                "hover:bg-rose-100 hover:text-rose-950 hover:ring-rose-300/80",
                "dark:bg-rose-950/35 dark:text-rose-50 dark:ring-rose-800/50",
                "dark:hover:bg-rose-950/50 dark:hover:text-rose-50",
                "active:scale-[0.98]",
              )}
            >
              {t("upload.replaceCancel")}
            </Button>
            <Button
              type="button"
              variant="default"
              size="lg"
              onClick={onOverwrite}
              className={cn(
                "h-11 min-w-[8.5rem] rounded-xl px-6 text-[13px] font-medium tracking-tight",
                "shadow-[inset_0_1px_0_rgba(255,255,255,0.12)]",
                "active:scale-[0.98]",
              )}
            >
              {t("upload.replaceConfirm")}
            </Button>
          </DialogFooter>
        </div>
      </DialogContent>
    </Dialog>
  );
}
