import { useState } from "react";
import { CaretDown, CircleNotch } from "@phosphor-icons/react";
import { useTranslation } from "react-i18next";
import { toast } from "sonner";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuGroup,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { api } from "@/lib/api";
import { cn } from "@/lib/utils";
import type { DealRoomKnowledgeQASession } from "@/types";

interface KnowledgeSessionHistoryMenuProps {
  roomId: string;
  activeSessionId: string | null;
  onOpenSession: (sessionId: string) => Promise<void>;
  className?: string;
}

export function KnowledgeSessionHistoryMenu({
  roomId,
  activeSessionId,
  onOpenSession,
  className,
}: KnowledgeSessionHistoryMenuProps) {
  const { t } = useTranslation("dealRooms");
  const [open, setOpen] = useState(false);
  const [loading, setLoading] = useState(false);
  const [loadingMore, setLoadingMore] = useState(false);
  const [openingId, setOpeningId] = useState<string | null>(null);
  const [items, setItems] = useState<DealRoomKnowledgeQASession[]>([]);
  const [nextCursor, setNextCursor] = useState<string | undefined>();

  const loadPage = async (cursor?: string, append = false) => {
    if (append) setLoadingMore(true);
    else setLoading(true);
    try {
      const res = await api.listDealRoomKnowledgeSessions(roomId, {
        limit: 20,
        cursor,
      });
      setItems((prev) => (append ? [...prev, ...(res.items ?? [])] : (res.items ?? [])));
      setNextCursor(res.nextCursor);
    } catch {
      toast.error(t("knowledge.sessionHistoryLoadFailed"));
      if (!append) {
        setItems([]);
        setNextCursor(undefined);
      }
    } finally {
      setLoading(false);
      setLoadingMore(false);
    }
  };

  return (
    <DropdownMenu
      open={open}
      onOpenChange={(next) => {
        setOpen(next);
        if (next) void loadPage();
      }}
    >
      <DropdownMenuTrigger
        data-testid="deal-room-knowledge-session-history"
        className={cn(
          "inline-flex items-center gap-1.5 rounded-full border border-border/80 bg-background/80 px-2.5 py-1 text-[11px] font-medium text-foreground/80 backdrop-blur-sm transition-colors hover:bg-muted/50 hover:text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring",
          className,
        )}
      >
        {t("knowledge.sessionHistory")}
        <CaretDown size={11} weight="bold" className="text-foreground/45" />
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end" className="w-80">
        <DropdownMenuGroup>
          <DropdownMenuLabel>{t("knowledge.sessionHistory")}</DropdownMenuLabel>
        </DropdownMenuGroup>
        <DropdownMenuSeparator />
        {loading ? (
          <div className="flex items-center justify-center gap-2 px-2 py-6 text-xs text-muted-foreground">
            <CircleNotch size={14} className="animate-spin" />
            {t("knowledge.sessionHistory")}
          </div>
        ) : items.length === 0 ? (
          <p className="px-2 py-4 text-center text-xs text-muted-foreground">
            {t("knowledge.sessionHistoryEmpty")}
          </p>
        ) : (
          <>
            <DropdownMenuGroup>
              {items.map((session) => {
                const label =
                  (session.title || session.questionPreview || "").trim() ||
                  t("knowledge.sessionUntitled");
                const selected = session.id === activeSessionId;
                return (
                  <DropdownMenuItem
                    key={session.id}
                    data-testid={`deal-room-knowledge-session-${session.id}`}
                    disabled={openingId === session.id}
                    className={cn("items-start py-2", selected && "bg-accent/70")}
                    onClick={() => {
                      void (async () => {
                        setOpeningId(session.id);
                        try {
                          await onOpenSession(session.id);
                          setOpen(false);
                        } catch {
                          toast.error(t("knowledge.sessionOpenFailed"));
                        } finally {
                          setOpeningId(null);
                        }
                      })();
                    }}
                  >
                    <div className="min-w-0 flex-1 space-y-0.5">
                      <p className="truncate text-sm font-medium text-foreground">{label}</p>
                      <p className="text-[11px] text-muted-foreground">
                        {session.status === "active"
                          ? t("knowledge.sessionStatusActive")
                          : t("knowledge.sessionStatusClosed")}
                        {" · "}
                        {t("knowledge.sessionTurnsShort", {
                          count: session.turnCount ?? 0,
                        })}
                      </p>
                    </div>
                    {openingId === session.id ? (
                      <CircleNotch size={14} className="mt-0.5 animate-spin" />
                    ) : null}
                  </DropdownMenuItem>
                );
              })}
            </DropdownMenuGroup>
            {nextCursor ? (
              <>
                <DropdownMenuSeparator />
                <DropdownMenuItem
                  data-testid="deal-room-knowledge-session-load-more"
                  disabled={loadingMore}
                  onClick={(e) => {
                    e.preventDefault();
                    void loadPage(nextCursor, true);
                  }}
                >
                  {loadingMore ? (
                    <CircleNotch size={14} className="mr-1.5 animate-spin" />
                  ) : null}
                  {t("knowledge.sessionLoadMore")}
                </DropdownMenuItem>
              </>
            ) : null}
          </>
        )}
      </DropdownMenuContent>
    </DropdownMenu>
  );
}
