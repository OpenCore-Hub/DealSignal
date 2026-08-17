import { useEffect, useMemo, useState } from "react";
import { Combobox } from "@base-ui/react/combobox";
import { CaretDown, CaretLeft, Check, FileText, MagnifyingGlass } from "@phosphor-icons/react";
import { motion } from "motion/react";
import { useTranslation } from "react-i18next";
import { Skeleton } from "@/components/ui/skeleton";
import { useAsyncData } from "@/hooks/useAsyncData";
import { useReducedMotion } from "@/hooks/useReducedMotion";
import { api } from "@/lib/api";
import {
  activeInsightRooms,
  collectDealRoomDocumentIds,
  documentTitle,
  filterInsightDocuments,
  filterInsightRooms,
  insightDocScope,
  insightRoomPickerMode,
  type InsightDocScope,
} from "@/lib/insightsDocumentPicker";
import { cn } from "@/lib/utils";
import type { Document } from "@/types";

const EASE = [0.16, 1, 0.3, 1] as const;

interface InsightsDocumentPickerProps {
  documents: Document[];
  selectedDocId: string;
  onSelectedDocIdChange: (id: string) => void;
}

export function InsightsDocumentPicker({
  documents,
  selectedDocId,
  onSelectedDocIdChange,
}: InsightsDocumentPickerProps) {
  const { t } = useTranslation("insights");
  const reducedMotion = useReducedMotion();
  const [docScope, setDocScope] = useState<InsightDocScope>("library");
  const [docQuery, setDocQuery] = useState("");
  const [roomQuery, setRoomQuery] = useState("");
  const [roomFilterId, setRoomFilterId] = useState("");
  const [roomPane, setRoomPane] = useState<"rooms" | "files">("files");

  const untitled = t("pages.untitledDocument");
  const selectedDoc = documents.find((d) => d.id === selectedDocId);

  useEffect(() => {
    if (selectedDoc) setDocScope(insightDocScope(selectedDoc));
  }, [selectedDoc]);

  const { data: rooms } = useAsyncData(async () => {
    try {
      const res = await api.getDealRooms({ page: 1, page_size: 100 });
      return activeInsightRooms(res.data ?? []);
    } catch {
      return [];
    }
  }, []);

  const { data: roomDocIds, loading: loadingRoomDocs } = useAsyncData(
    async () => {
      if (!roomFilterId) return null;
      const res = await api.getDealRoomDocuments(roomFilterId);
      return collectDealRoomDocumentIds(res.data ?? []);
    },
    [roomFilterId],
  );

  const scopeCounts = useMemo(() => {
    const counts = { library: 0, deal_room: 0 };
    for (const doc of documents) {
      counts[insightDocScope(doc)] += 1;
    }
    return counts;
  }, [documents]);

  const roomChoices = rooms ?? [];
  const roomMode = insightRoomPickerMode(roomChoices.length);

  useEffect(() => {
    if (docScope === "deal_room" && roomMode === "browse" && !roomFilterId) {
      setRoomPane("rooms");
    }
  }, [docScope, roomMode, roomFilterId]);
  const browsingRooms = docScope === "deal_room" && roomMode === "browse" && roomPane === "rooms";
  const showRoomRail = docScope === "deal_room" && roomMode === "rail";
  const visibleRooms = useMemo(
    () => filterInsightRooms(roomChoices, roomQuery),
    [roomChoices, roomQuery],
  );
  const selectedRoom = roomChoices.find((room) => room.id === roomFilterId);

  const activeRoomDocIds = !roomFilterId
    ? null
    : loadingRoomDocs
      ? new Set<string>()
      : roomDocIds;

  const filteredDocuments = useMemo(
    () => filterInsightDocuments(documents, docScope, docQuery, activeRoomDocIds),
    [documents, docScope, docQuery, activeRoomDocIds],
  );

  const scopeLabel = (scope: InsightDocScope) =>
    scope === "deal_room" ? t("pages.scopeDealRoom") : t("pages.scopeLibrary");

  const resetPickerFilters = () => {
    setDocQuery("");
    setRoomQuery("");
    setRoomFilterId("");
    setRoomPane("files");
    if (selectedDoc) setDocScope(insightDocScope(selectedDoc));
  };

  const switchScope = (next: InsightDocScope) => {
    if (next === docScope) return;
    setDocScope(next);
    setDocQuery("");
    setRoomQuery("");
    setRoomFilterId("");
    setRoomPane(next === "deal_room" && roomMode === "browse" ? "rooms" : "files");
  };

  const selectRoom = (id: string) => {
    setRoomFilterId(id);
    setDocQuery("");
    setRoomQuery("");
    setRoomPane("files");
  };

  return (
    <Combobox.Root
      value={selectedDocId || null}
      onValueChange={(next) => {
        if (next) onSelectedDocIdChange(next);
      }}
      onInputValueChange={setDocQuery}
      onOpenChange={(open) => {
        if (open) {
          if (docScope === "deal_room" && roomMode === "browse") {
            setRoomPane("rooms");
          }
          return;
        }
        resetPickerFilters();
      }}
    >
      <Combobox.Trigger
        data-testid="insights-document-picker"
        className={cn(
          "group flex h-9 w-full items-center gap-2 rounded-lg border border-input bg-background/80 px-2.5 text-sm outline-none",
          "shadow-[inset_0_1px_0_rgba(255,255,255,0.6)] transition-[border-color,box-shadow] duration-200 ease-[cubic-bezier(0.16,1,0.3,1)]",
          "hover:border-foreground/20",
          "focus-visible:border-ring focus-visible:ring-3 focus-visible:ring-ring/50",
          "sm:w-[22rem] sm:max-w-[22rem] sm:self-end",
        )}
      >
        <span className="min-w-0 flex-1 truncate text-left tracking-[-0.01em]">
          {selectedDoc ? documentTitle(selectedDoc, untitled) : t("pages.selectPlaceholder")}
        </span>
        {selectedDoc ? (
          <span className="shrink-0 rounded-md bg-muted/80 px-1.5 py-0.5 text-[11px] leading-none text-muted-foreground">
            {scopeLabel(insightDocScope(selectedDoc))}
          </span>
        ) : null}
        <Combobox.Icon
          render={
            <CaretDown
              size={14}
              weight="light"
              className="shrink-0 text-muted-foreground transition-transform duration-200 group-data-[popup-open]:rotate-180"
            />
          }
        />
      </Combobox.Trigger>

      <Combobox.Portal>
        <Combobox.Positioner
          className="isolate z-50"
          align="end"
          side="bottom"
          sideOffset={6}
        >
          <Combobox.Popup
            className={cn(
              "z-50 w-[min(24rem,var(--available-width))] min-w-[var(--anchor-width)] origin-[var(--transform-origin)] overflow-hidden rounded-xl bg-popover text-popover-foreground outline-none",
              "ring-1 ring-black/5 dark:ring-white/10",
              "shadow-[0_16px_40px_-16px_rgba(15,23,42,0.22)]",
              "data-[side=bottom]:slide-in-from-top-2 data-[side=top]:slide-in-from-bottom-2",
              "data-open:animate-in data-open:fade-in-0 data-open:zoom-in-95",
              "data-closed:animate-out data-closed:fade-out-0 data-closed:zoom-out-95",
            )}
          >
            <div className="p-1.5">
              <div
                role="tablist"
                aria-label={t("pages.scopeFilter")}
                className="grid grid-cols-2 rounded-lg bg-muted/70 p-0.5"
              >
                {(["library", "deal_room"] as const).map((scope) => {
                  const active = docScope === scope;
                  return (
                    <button
                      key={scope}
                      type="button"
                      role="tab"
                      aria-selected={active}
                      data-testid={`insights-doc-scope-${scope}`}
                      onMouseDown={(event) => event.preventDefault()}
                      onClick={() => switchScope(scope)}
                      className={cn(
                        "relative isolate inline-flex h-8 items-center justify-center gap-1.5 rounded-md px-2 text-[13px] tracking-[-0.01em] transition-colors duration-200",
                        active ? "text-foreground" : "text-muted-foreground hover:text-foreground/80",
                      )}
                    >
                      {active ? (
                        <motion.span
                          layoutId={reducedMotion ? undefined : "insights-doc-scope-pill"}
                          className="absolute inset-0 -z-10 rounded-md bg-background shadow-[0_1px_2px_rgba(15,23,42,0.06)] ring-1 ring-black/5 dark:ring-white/10"
                          transition={{ type: "spring", stiffness: 420, damping: 34 }}
                        />
                      ) : null}
                      <span>{scopeLabel(scope)}</span>
                      <span className="font-mono text-[11px] tabular-nums text-muted-foreground">
                        {scopeCounts[scope]}
                      </span>
                    </button>
                  );
                })}
              </div>
            </div>

            {showRoomRail ? (
              <div
                data-testid="insights-doc-room-filter"
                role="radiogroup"
                aria-label={t("pages.roomFilter")}
                className="flex gap-0 overflow-x-auto border-t border-border/70 px-1.5 [scrollbar-width:none] [&::-webkit-scrollbar]:hidden"
              >
                <RoomRailItem
                  testId="insights-doc-room-all"
                  label={t("pages.roomAllShort")}
                  selected={!roomFilterId}
                  reducedMotion={reducedMotion}
                  onSelect={() => selectRoom("")}
                />
                {roomChoices.map((room) => (
                  <RoomRailItem
                    key={room.id}
                    testId={`insights-doc-room-${room.id}`}
                    label={room.name}
                    selected={roomFilterId === room.id}
                    reducedMotion={reducedMotion}
                    onSelect={() => selectRoom(room.id)}
                  />
                ))}
              </div>
            ) : null}

            {browsingRooms ? (
              <>
                <SearchField
                  value={roomQuery}
                  placeholder={t("pages.roomSearchPlaceholder")}
                  testId="insights-doc-room-search"
                  onChange={setRoomQuery}
                />
                <div className="max-h-64 overflow-auto p-1">
                  <RoomBrowseRow
                    testId="insights-doc-room-all"
                    name={t("pages.roomAll")}
                    meta={String(scopeCounts.deal_room)}
                    selected={!roomFilterId}
                    onSelect={() => selectRoom("")}
                  />
                  {visibleRooms.map((room) => (
                    <RoomBrowseRow
                      key={room.id}
                      testId={`insights-doc-room-${room.id}`}
                      name={room.name}
                      meta={String(room.documentCount)}
                      selected={roomFilterId === room.id}
                      onSelect={() => selectRoom(room.id)}
                    />
                  ))}
                  {visibleRooms.length === 0 ? (
                    <p className="px-2 py-6 text-center text-[13px] text-muted-foreground">
                      {t("pages.noRoomMatches")}
                    </p>
                  ) : null}
                </div>
              </>
            ) : (
              <>
                {roomMode === "browse" && docScope === "deal_room" ? (
                  <button
                    type="button"
                    data-testid="insights-doc-room-back"
                    onMouseDown={(event) => event.preventDefault()}
                    onClick={() => {
                      setRoomPane("rooms");
                      setDocQuery("");
                    }}
                    className="flex w-full items-center gap-1.5 border-t border-border/70 px-3 py-2 text-left text-[13px] text-muted-foreground transition-colors hover:text-foreground"
                  >
                    <CaretLeft size={14} weight="light" />
                    <span className="min-w-0 flex-1 truncate tracking-[-0.01em]">
                      {selectedRoom?.name ?? t("pages.roomAll")}
                    </span>
                  </button>
                ) : null}
                <SearchField
                  combobox
                  inputKey={`${docScope}:${roomFilterId}`}
                  placeholder={
                    docScope === "deal_room"
                      ? t("pages.searchPlaceholderDealRoom")
                      : t("pages.searchPlaceholder")
                  }
                />
                {roomFilterId && loadingRoomDocs ? (
                  <div className="space-y-1.5 p-2">
                    <Skeleton className="h-8 rounded-md" />
                    <Skeleton className="h-8 rounded-md" />
                  </div>
                ) : (
                  <Combobox.List className="max-h-64 overflow-auto p-1">
                    {filteredDocuments.map((doc) => {
                      const title = documentTitle(doc, untitled);
                      return (
                        <Combobox.Item
                          key={doc.id}
                          value={doc.id}
                          className={cn(
                            "group relative flex w-full cursor-default items-center gap-2.5 rounded-lg px-2 py-2 text-left text-[13px] tracking-[-0.01em] outline-none select-none",
                            "transition-colors duration-150",
                            "hover:bg-foreground/[0.04] data-[highlighted]:bg-foreground/[0.04]",
                          )}
                        >
                          <FileText
                            size={15}
                            weight="light"
                            className="shrink-0 text-muted-foreground"
                          />
                          <span className="min-w-0 flex-1 truncate">{title}</span>
                          <Combobox.ItemIndicator className="ml-auto text-foreground">
                            <Check size={14} weight="light" />
                          </Combobox.ItemIndicator>
                        </Combobox.Item>
                      );
                    })}
                    {filteredDocuments.length === 0 ? (
                      <Combobox.Empty className="px-2 py-7 text-center text-[13px] text-muted-foreground">
                        {roomFilterId && !docQuery
                          ? t("pages.noRoomDocuments")
                          : t("pages.noSearchResults")}
                      </Combobox.Empty>
                    ) : null}
                  </Combobox.List>
                )}
              </>
            )}
          </Combobox.Popup>
        </Combobox.Positioner>
      </Combobox.Portal>
    </Combobox.Root>
  );
}

function RoomRailItem({
  testId,
  label,
  selected,
  reducedMotion,
  onSelect,
}: {
  testId: string;
  label: string;
  selected: boolean;
  reducedMotion: boolean;
  onSelect: () => void;
}) {
  return (
    <button
      type="button"
      role="radio"
      aria-checked={selected}
      data-testid={testId}
      onMouseDown={(event) => event.preventDefault()}
      onClick={onSelect}
      className={cn(
        "relative shrink-0 px-2.5 py-2 text-[13px] tracking-[-0.01em] transition-colors duration-200",
        selected ? "text-foreground" : "text-muted-foreground hover:text-foreground/80",
      )}
    >
      <span className="max-w-[9rem] truncate">{label}</span>
      {selected ? (
        <motion.span
          layoutId={reducedMotion ? undefined : "insights-doc-room-underline"}
          className="absolute inset-x-2 bottom-0 h-px bg-foreground"
          transition={{ type: "spring", stiffness: 420, damping: 34 }}
        />
      ) : null}
    </button>
  );
}

function RoomBrowseRow({
  testId,
  name,
  meta,
  selected,
  onSelect,
}: {
  testId: string;
  name: string;
  meta: string;
  selected: boolean;
  onSelect: () => void;
}) {
  return (
    <button
      type="button"
      data-testid={testId}
      onMouseDown={(event) => event.preventDefault()}
      onClick={onSelect}
      className={cn(
        "flex w-full items-center gap-2 rounded-lg px-2 py-2 text-left text-[13px] tracking-[-0.01em] transition-colors",
        "hover:bg-foreground/[0.04] active:scale-[0.99]",
        selected ? "text-foreground" : "text-foreground/90",
      )}
    >
      <span className="min-w-0 flex-1 truncate">{name}</span>
      <span className="font-mono text-[11px] tabular-nums text-muted-foreground">{meta}</span>
      {selected ? <Check size={14} weight="light" className="shrink-0" /> : null}
    </button>
  );
}

function SearchField({
  combobox,
  inputKey,
  placeholder,
  value,
  testId,
  onChange,
}: {
  combobox?: boolean;
  inputKey?: string;
  placeholder: string;
  value?: string;
  testId?: string;
  onChange?: (value: string) => void;
}) {
  const fieldClass = cn(
    "flex h-8 w-full bg-transparent text-[13px] tracking-[-0.01em] outline-none",
    "placeholder:text-muted-foreground/70 focus-visible:outline-none focus-visible:ring-0",
  );
  return (
    <div className="flex items-center gap-2 border-t border-border/70 px-3 py-1.5">
      <MagnifyingGlass size={14} weight="light" className="shrink-0 text-muted-foreground/70" />
      {combobox ? (
        <Combobox.Input key={inputKey} placeholder={placeholder} className={fieldClass} />
      ) : (
        <input
          data-testid={testId}
          value={value}
          placeholder={placeholder}
          onMouseDown={(event) => event.stopPropagation()}
          onChange={(event) => onChange?.(event.target.value)}
          className={fieldClass}
        />
      )}
    </div>
  );
}
