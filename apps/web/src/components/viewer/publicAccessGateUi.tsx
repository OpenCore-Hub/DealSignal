import type { CSSProperties, ReactNode } from "react";
import { Prohibit, ShieldCheck, Sparkle, Spinner } from "@phosphor-icons/react";
import { useTranslation } from "react-i18next";
import { Button } from "@/components/ui/button";
import { cn } from "@/lib/utils";

export const publicGateInputClassName =
  "rounded-xl border-border/70 bg-background/80 shadow-sm focus-visible:ring-emerald-500/30";

export const publicGateTextareaClassName =
  "rounded-xl border-border/70 bg-background/80 shadow-sm focus-visible:ring-emerald-500/30";

export function PublicGatePageShell({
  children,
  className,
}: {
  children: ReactNode;
  className?: string;
}) {
  return (
    <div
      className={cn(
        "public-viewer-shell relative flex min-h-[100dvh] items-center justify-center p-4 sm:p-6",
        className
      )}
    >
      {children}
    </div>
  );
}

export function PublicGateCard({
  children,
  className,
  style,
  dimmed = false,
}: {
  children: ReactNode;
  className?: string;
  style?: CSSProperties;
  dimmed?: boolean;
}) {
  return (
    <div
      className={cn(
        "public-viewer-glass overflow-hidden rounded-3xl",
        dimmed && "scale-[0.985] opacity-70 blur-[2px] transition-[filter,opacity,transform] duration-300",
        !dimmed && "transition-[filter,opacity,transform] duration-300",
        className
      )}
      style={style}
    >
      {children}
    </div>
  );
}

export function PublicGateCardHeader({
  title,
  subtitle,
  icon = "shield",
}: {
  title: ReactNode;
  subtitle?: ReactNode;
  icon?: "shield" | "sparkle" | "prohibit";
}) {
  const Icon = icon === "sparkle" ? Sparkle : icon === "prohibit" ? Prohibit : ShieldCheck;

  return (
    <div className="border-b border-border/60 px-6 py-5">
      <div className="flex items-start gap-3">
        <div className="flex h-10 w-10 shrink-0 items-center justify-center rounded-2xl bg-gradient-to-br from-emerald-600 to-slate-900 text-white shadow-sm">
          <Icon size={18} weight={icon === "sparkle" ? "fill" : "duotone"} />
        </div>
        <div className="min-w-0 flex-1">
          <h1 className="text-lg font-semibold tracking-tight text-foreground">{title}</h1>
          {subtitle ? (
            <p className="mt-1 text-sm leading-relaxed text-muted-foreground">{subtitle}</p>
          ) : null}
        </div>
      </div>
    </div>
  );
}

export function PublicGateCardBody({
  children,
  className,
}: {
  children: ReactNode;
  className?: string;
}) {
  return <div className={cn("space-y-4 px-6 py-5", className)}>{children}</div>;
}

export function PublicGateInviteBanner({ children }: { children: ReactNode }) {
  return (
    <div className="rounded-2xl border border-emerald-500/15 bg-emerald-500/5 px-4 py-3 text-sm leading-relaxed text-foreground">
      {children}
    </div>
  );
}

export function PublicGateNdaFrame({
  children,
  className,
  interactive = false,
  onClick,
  onKeyDown,
  title,
  "aria-label": ariaLabel,
}: {
  children: ReactNode;
  className?: string;
  interactive?: boolean;
  onClick?: () => void;
  onKeyDown?: (e: React.KeyboardEvent) => void;
  title?: string;
  "aria-label"?: string;
}) {
  return (
    <div
      className={cn(
        "min-h-0 flex-1 rounded-2xl border border-border/60 bg-background/90 shadow-inner",
        interactive ? "cursor-zoom-in overflow-y-auto overscroll-contain" : "overflow-hidden",
        className
      )}
      onClick={onClick}
      onKeyDown={onKeyDown}
      title={title}
      role={interactive ? "button" : undefined}
      tabIndex={interactive ? 0 : undefined}
      aria-label={ariaLabel}
    >
      {children}
    </div>
  );
}

export function PublicGatePrimaryButton({
  className,
  ...props
}: React.ComponentProps<typeof Button>) {
  return (
    <Button
      className={cn(
        "h-11 rounded-xl bg-foreground px-5 text-background shadow-sm hover:bg-foreground/90",
        className
      )}
      {...props}
    />
  );
}

export function PublicGateSecondaryButton({
  className,
  ...props
}: React.ComponentProps<typeof Button>) {
  return (
    <Button
      variant="outline"
      className={cn(
        "h-11 rounded-xl border-border/70 bg-background/70 px-5 shadow-sm hover:bg-background",
        className
      )}
      {...props}
    />
  );
}

export function PublicGateLoadingScreen() {
  const { t } = useTranslation("documents");

  return (
    <PublicGatePageShell>
      <PublicGateCard className="w-full max-w-md">
        <PublicGateCardBody className="flex flex-col items-center py-10 text-center">
          <Spinner size={28} className="animate-spin text-emerald-600" />
          <p className="mt-4 text-sm font-medium text-foreground">{t("viewer.loading")}</p>
          <p className="mt-1 text-xs text-muted-foreground">{t("viewer.gateLoadingHint")}</p>
        </PublicGateCardBody>
      </PublicGateCard>
    </PublicGatePageShell>
  );
}

export function PublicGateFatalErrorScreen({ message }: { message: string }) {
  return (
    <PublicGatePageShell>
      <PublicGateCard className="w-full max-w-md">
        <PublicGateCardHeader title={message} icon="prohibit" />
      </PublicGateCard>
    </PublicGatePageShell>
  );
}

export function PublicLinkErrorScreen({
  title,
  description,
  onBackHome,
}: {
  title: string;
  description: string;
  onBackHome: () => void;
}) {
  const { t } = useTranslation(["documents", "common"]);

  return (
    <PublicGatePageShell>
      <PublicGateCard className="w-full max-w-md">
        <PublicGateCardHeader title={title} subtitle={description} icon="prohibit" />
        <PublicGateCardBody className="pt-2">
          <PublicGateSecondaryButton className="w-full" onClick={onBackHome}>
            {t("common:backToHome")}
          </PublicGateSecondaryButton>
        </PublicGateCardBody>
      </PublicGateCard>
    </PublicGatePageShell>
  );
}
