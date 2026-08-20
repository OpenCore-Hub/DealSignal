import { useEffect, useMemo, useState } from "react";
import { apiErrorMessage } from "@/lib/apiErrors";
import {
  CaretDown,
  CaretLeft,
  CaretRight,
  CaretUp,
  ChartLine,
  Copy,
  EnvelopeSimple,
  Link as LinkIcon,
  MagnifyingGlass,
  PencilSimple,
  Trash,
  UserPlus,
} from "@phosphor-icons/react";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Checkbox } from "@/components/ui/checkbox";
import { Input } from "@/components/ui/input";
import { Switch } from "@/components/ui/switch";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { useTranslation } from "react-i18next";
import { toast } from "sonner";
import { api } from "@/lib/api";
import { copyToClipboard } from "@/lib/clipboard";
import { useAsyncData } from "@/hooks/useAsyncData";
import { DealRoomShareDialog } from "./DealRoomShareDialog";
import { SendVerificationCodeDialog } from "./SendVerificationCodeDialog";
import { LinkActivityDialog } from "@/components/links/share";
import type { Link } from "@/types";

const PAGE_SIZE = 10;
const MAX_SEARCH_LEN = 200;

type CreatedSort = "desc" | "asc";

interface FolderPermissionsSectionProps {
  roomId: string;
  slug?: string;
  /** Bump to force-reload links after creates from outside this section (e.g. toolbar). */
  refreshKey?: number;
  /** Notify parent so documents/analytics active-link signals stay in sync. */
  onLinksChanged?: () => void;
  /** Open Access policy. Pass a link id to highlight that share-link's applicants. */
  onManageAccess?: (linkId?: string) => void;
  canManage?: boolean;
}

function formatDateTime(value: string | undefined, emptyLabel: string): string {
  if (!value) return emptyLabel;
  try {
    return new Date(value).toLocaleString();
  } catch {
    return value;
  }
}

function stopRowActivation(e: React.SyntheticEvent) {
  e.stopPropagation();
}

export function FolderPermissionsSection({
  roomId,
  slug,
  refreshKey = 0,
  onLinksChanged,
  onManageAccess,
  canManage = false,
}: FolderPermissionsSectionProps) {
  const { t } = useTranslation("dealRooms");
  const canJumpToAccessRequests = Boolean(onManageAccess && canManage);
  const emptyCell = t("common:emDash");
  const [page, setPage] = useState(1);
  // null = default newest-first load; first header click locks to desc, then toggles.
  const [createdSort, setCreatedSort] = useState<CreatedSort | null>(null);
  const activeCreatedSort: CreatedSort = createdSort ?? "desc";
  const [searchQuery, setSearchQuery] = useState("");
  const [debouncedQ, setDebouncedQ] = useState("");
  const [pageInput, setPageInput] = useState("1");

  useEffect(() => {
    const timer = window.setTimeout(() => {
      setDebouncedQ(searchQuery.trim().slice(0, MAX_SEARCH_LEN));
    }, 250);
    return () => window.clearTimeout(timer);
  }, [searchQuery]);

  useEffect(() => {
    setPage(1);
  }, [debouncedQ, activeCreatedSort, roomId]);

  const {
    data: pageResult,
    loading,
    refetch,
  } = useAsyncData(async () => {
    return api.getDealRoomLinks(roomId, {
      page,
      page_size: PAGE_SIZE,
      sort: activeCreatedSort === "asc" ? "created_at_asc" : "created_at_desc",
      q: debouncedQ || undefined,
    });
  }, [roomId, refreshKey, page, activeCreatedSort, debouncedQ]);

  const linkList = pageResult?.data ?? [];
  const total = pageResult?.pagination?.total ?? linkList.length;
  const pageSize = pageResult?.pagination?.page_size ?? PAGE_SIZE;
  const currentPage = pageResult?.pagination?.page ?? page;
  const totalPages = Math.max(1, Math.ceil(total / pageSize));
  const linkIdsKey = linkList.map((l) => l.id).join(",");

  useEffect(() => {
    // Accept server clamp only when the client page is past the last page
    // (e.g. after deletes). Avoid fighting intentional resets (search/sort → page 1)
    // against a stale paginated response.
    const serverPage = pageResult?.pagination?.page;
    if (serverPage == null || serverPage === page) return;
    if (page > totalPages) {
      setPage(serverPage);
    }
  }, [pageResult?.pagination?.page, page, totalPages]);

  useEffect(() => {
    setPageInput(String(currentPage));
  }, [currentPage]);

  const { data: pendingByLinkId, error: pendingError, refetch: refetchPending } = useAsyncData(async () => {
    if (linkList.length === 0) return {} as Record<string, number>;
    // Creator-scoped workspace inbox — empty for non-creators; one request vs N+1.
    const res = await api.getPendingLinkAccessRequests({
      scope: "deal_room",
      dealRoomId: roomId,
    });
    const onPage = new Set(linkList.map((link) => link.id));
    const counts: Record<string, number> = {};
    for (const request of res.data ?? []) {
      if (!onPage.has(request.link_id)) continue;
      counts[request.link_id] = (counts[request.link_id] ?? 0) + 1;
    }
    return counts;
  }, [roomId, refreshKey, linkIdsKey]);

  const [viewLink, setViewLink] = useState<Link | null>(null);
  const [createOpen, setCreateOpen] = useState(false);
  const [editLink, setEditLink] = useState<Link | null>(null);
  const [sendCodeLink, setSendCodeLink] = useState<Link | null>(null);
  const [deleteLink, setDeleteLink] = useState<Link | null>(null);
  const [deleteLoading, setDeleteLoading] = useState(false);
  const [selectedIds, setSelectedIds] = useState<Set<string>>(() => new Set());
  const [bulkDeleteOpen, setBulkDeleteOpen] = useState(false);
  const [bulkDeleteLoading, setBulkDeleteLoading] = useState(false);

  const allPageSelected =
    linkList.length > 0 && linkList.every((link) => selectedIds.has(link.id));
  const somePageSelected = linkList.some((link) => selectedIds.has(link.id));

  const handleCreateLink = () => {
    setCreateOpen(true);
  };

  const totalPending = useMemo(() => {
    if (!pendingByLinkId) return 0;
    return Object.values(pendingByLinkId).reduce((sum, n) => sum + n, 0);
  }, [pendingByLinkId]);

  const refreshAll = async () => {
    await refetch();
    await refetchPending();
    onLinksChanged?.();
  };

  const goToPage = (next: number) => {
    const clamped = Math.min(totalPages, Math.max(1, next));
    setPage(clamped);
  };

  const handlePageJump = () => {
    const n = Number.parseInt(pageInput, 10);
    if (Number.isFinite(n)) goToPage(n);
    else setPageInput(String(currentPage));
  };

  const cycleCreatedSort = () => {
    // Product cycle: first click → descending, next → ascending, then toggle.
    setCreatedSort((prev) => {
      if (prev === null) return "desc";
      return prev === "desc" ? "asc" : "desc";
    });
  };

  const handleActiveChange = async (linkId: string, checked: boolean) => {
    try {
      await api.updateLink(linkId, { status: checked ? "active" : "revoked" });
      await refetch();
      onLinksChanged?.();
    } catch (err) {
      toast.error(apiErrorMessage(err, { fallback: "saveFailed" }));
    }
  };

  const toggleRowSelected = (linkId: string, checked: boolean | "indeterminate") => {
    setSelectedIds((prev) => {
      const next = new Set(prev);
      if (checked === true) next.add(linkId);
      else next.delete(linkId);
      return next;
    });
  };

  const toggleSelectAllPage = (checked: boolean | "indeterminate") => {
    setSelectedIds((prev) => {
      const next = new Set(prev);
      if (checked === true) {
        for (const link of linkList) next.add(link.id);
      } else {
        for (const link of linkList) next.delete(link.id);
      }
      return next;
    });
  };

  const handleDelete = async () => {
    if (!deleteLink) return;
    setDeleteLoading(true);
    try {
      await api.deleteLink(deleteLink.id);
      toast.success(t("permissions.links.delete.success"));
      setSelectedIds((prev) => {
        const next = new Set(prev);
        next.delete(deleteLink.id);
        return next;
      });
      setDeleteLink(null);
      await refreshAll();
    } catch (err) {
      toast.error(apiErrorMessage(err, { fallback: "deleteFailed", messageKey: "dealRooms:permissions.links.delete.error" }));
    } finally {
      setDeleteLoading(false);
    }
  };

  const handleBulkDelete = async () => {
    const ids = Array.from(selectedIds);
    if (ids.length === 0) return;
    setBulkDeleteLoading(true);
    let succeeded = 0;
    let failed = 0;
    try {
      for (const id of ids) {
        try {
          await api.deleteLink(id);
          succeeded += 1;
        } catch {
          failed += 1;
        }
      }
      if (failed === 0) {
        toast.success(t("permissions.links.delete.bulkSuccess", { count: succeeded }));
      } else {
        toast.error(
          t("permissions.links.delete.bulkPartialError", { succeeded, failed }),
        );
      }
      setBulkDeleteOpen(false);
      setSelectedIds(new Set());
      setPage(1);
      await refreshAll();
    } finally {
      setBulkDeleteLoading(false);
    }
  };

  const pendingBannerClassName =
    "mb-4 w-full rounded-lg border border-amber-500/30 bg-amber-500/5 px-4 py-3 text-sm";
  const pendingBannerText = t("permissions.links.pendingRequestsBanner", { count: totalPending });
  const pendingBanner = pendingError ? (
    <div
      className="mb-4 rounded-lg border border-destructive/30 bg-destructive/5 px-4 py-3 text-sm"
      role="alert"
      data-testid="deal-room-pending-access-requests-error"
    >
      <p className="text-destructive">{t("permissions.links.pendingRequestsLoadFailed")}</p>
      <Button
        size="sm"
        variant="outline"
        className="mt-2"
        onClick={() => { void refetchPending(); }}
      >
        {t("common:retry")}
      </Button>
    </div>
  ) : totalPending > 0 ? (
    canJumpToAccessRequests ? (
      <button
        type="button"
        className={`${pendingBannerClassName} text-left transition-colors hover:bg-amber-500/10`}
        data-testid="deal-room-pending-access-requests"
        onClick={() => onManageAccess?.()}
      >
        {pendingBannerText}
      </button>
    ) : (
      <div
        className={pendingBannerClassName}
        role="status"
        data-testid="deal-room-pending-access-requests"
      >
        {pendingBannerText}
      </div>
    )
  ) : null;

  const showEmptyState = !loading && total === 0 && !debouncedQ;

  return (
    <>
      {showEmptyState ? (
        <div className="space-y-4" data-testid="deal-room-links-empty">
          {pendingBanner}
          <div className="flex flex-col items-center justify-center rounded-xl bg-rose-50/40 px-6 py-16 text-center dark:bg-rose-950/15">
            <LinkIcon size={40} className="mb-3 text-muted-foreground" />
            <p className="text-body text-muted-foreground">{t("permissions.links.emptyTitle")}</p>
            {canManage ? (
              <Button className="mt-4" onClick={handleCreateLink}>
                {t("permissions.links.createLink")}
              </Button>
            ) : null}
          </div>
        </div>
      ) : (
        <div className="space-y-3" data-testid="deal-room-links-table">
          {pendingBanner}
          <div
            className="flex flex-wrap items-center justify-end gap-2"
            data-testid="deal-room-links-toolbar"
          >
            <div className="relative w-full max-w-xs sm:w-64">
              <MagnifyingGlass
                size={16}
                className="pointer-events-none absolute left-2.5 top-1/2 -translate-y-1/2 text-muted-foreground"
                aria-hidden
              />
              <Input
                value={searchQuery}
                onChange={(e) => setSearchQuery(e.target.value.slice(0, MAX_SEARCH_LEN))}
                placeholder={t("permissions.links.searchPlaceholder")}
                aria-label={t("permissions.links.searchAria")}
                className="h-9 pl-8"
                maxLength={MAX_SEARCH_LEN}
              />
            </div>
            {canManage ? (
              <Button onClick={handleCreateLink} data-testid="deal-room-create-new-link">
                {t("permissions.links.createNewLink")}
              </Button>
            ) : null}
            {canManage ? (
              <Button
                variant="outline"
                disabled={selectedIds.size === 0 || bulkDeleteLoading}
                onClick={() => setBulkDeleteOpen(true)}
                data-testid="deal-room-bulk-delete-links"
              >
                <Trash size={14} className="mr-1.5" />
                {t("permissions.links.bulkDelete")}
              </Button>
            ) : null}
          </div>
          <div className="overflow-hidden rounded-lg border">
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead className="w-10">
                    <div
                      className="flex items-center"
                      onClick={stopRowActivation}
                      onPointerDown={stopRowActivation}
                    >
                      <Checkbox
                        checked={allPageSelected}
                        indeterminate={!allPageSelected && somePageSelected}
                        onCheckedChange={toggleSelectAllPage}
                        aria-label={t("permissions.links.selectAll")}
                        disabled={linkList.length === 0}
                      />
                    </div>
                  </TableHead>
                  <TableHead className="text-muted-foreground">
                    {t("permissions.links.table.name")}
                  </TableHead>
                  <TableHead className="text-muted-foreground">
                    {t("permissions.links.table.link")}
                  </TableHead>
                  <TableHead className="text-muted-foreground">
                    {t("permissions.links.table.visits")}
                  </TableHead>
                  <TableHead className="text-muted-foreground">
                    {t("permissions.links.table.lastViewed")}
                  </TableHead>
                  <TableHead className="text-muted-foreground">
                    <button
                      type="button"
                      className="inline-flex items-center gap-1 font-medium text-muted-foreground hover:text-foreground"
                      onClick={cycleCreatedSort}
                      aria-label={
                        activeCreatedSort === "asc"
                          ? t("permissions.links.table.sortCreatedAsc")
                          : t("permissions.links.table.sortCreatedDesc")
                      }
                      aria-sort={activeCreatedSort === "asc" ? "ascending" : "descending"}
                      data-testid="deal-room-links-sort-created"
                      data-sort={activeCreatedSort}
                    >
                      {t("permissions.links.table.createdAt")}
                      {activeCreatedSort === "asc" ? <CaretUp size={14} /> : <CaretDown size={14} />}
                    </button>
                  </TableHead>
                  <TableHead className="text-right text-muted-foreground">
                    {t("permissions.links.table.active")}
                  </TableHead>
                  <TableHead className="text-right text-muted-foreground">
                    {t("permissions.links.table.actions")}
                  </TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {loading ? (
                  <TableRow className="border-0 hover:bg-transparent">
                    <TableCell colSpan={8} className="py-8 text-center text-sm text-muted-foreground">
                      {t("common:loading")}
                    </TableCell>
                  </TableRow>
                ) : linkList.length === 0 ? (
                  <TableRow className="border-0 hover:bg-transparent">
                    <TableCell colSpan={8} className="py-8 text-center text-sm text-muted-foreground">
                      {t("permissions.links.noSearchResults")}
                    </TableCell>
                  </TableRow>
                ) : (
                  linkList.map((link) => {
                    const pendingCount = pendingByLinkId?.[link.id] ?? 0;
                    const displayName = link.name || t("permissions.links.table.name");
                    return (
                      <TableRow
                        key={link.id}
                        className="cursor-pointer"
                        onClick={() => setViewLink(link)}
                        data-testid={`deal-room-link-row-${link.id}`}
                      >
                        <TableCell onClick={stopRowActivation} onPointerDown={stopRowActivation}>
                          <Checkbox
                            checked={selectedIds.has(link.id)}
                            onCheckedChange={(checked) => toggleRowSelected(link.id, checked)}
                            aria-label={t("permissions.links.selectRow", { name: displayName })}
                          />
                        </TableCell>
                        <TableCell className="font-medium">
                          <div className="flex items-center gap-2">
                            <span>{displayName}</span>
                            {pendingCount > 0 ? (
                              canJumpToAccessRequests ? (
                                <button
                                  type="button"
                                  className="rounded-full"
                                  data-testid={`deal-room-link-pending-${link.id}`}
                                  aria-label={t("permissions.links.pendingRequestsBadgeOpen", {
                                    count: pendingCount,
                                  })}
                                  onClick={(e) => {
                                    stopRowActivation(e);
                                    onManageAccess?.(link.id);
                                  }}
                                  onPointerDown={stopRowActivation}
                                >
                                  <Badge variant="warm">
                                    {t("permissions.links.pendingRequestsBadge", {
                                      count: pendingCount,
                                    })}
                                  </Badge>
                                </button>
                              ) : (
                                <Badge variant="warm">
                                  {t("permissions.links.pendingRequestsBadge", {
                                    count: pendingCount,
                                  })}
                                </Badge>
                              )
                            ) : null}
                          </div>
                        </TableCell>
                        <TableCell
                          className="max-w-[14rem]"
                          onClick={stopRowActivation}
                          onPointerDown={stopRowActivation}
                        >
                          <div className="flex min-w-0 items-center gap-1">
                            <span
                              className="min-w-0 flex-1 truncate font-mono text-xs text-muted-foreground"
                              title={link.shortUrl}
                            >
                              {link.shortUrl}
                            </span>
                            <Button
                              type="button"
                              size="icon-sm"
                              variant="ghost"
                              className="shrink-0"
                              aria-label={t("share.copyLink")}
                              data-testid={`deal-room-link-copy-${link.id}`}
                              onClick={() => {
                                void copyToClipboard(link.shortUrl, t("share.copied"));
                              }}
                            >
                              <Copy size={14} />
                            </Button>
                          </div>
                        </TableCell>
                        <TableCell>{link.accessCount ?? 0}</TableCell>
                        <TableCell className="text-muted-foreground">
                          {formatDateTime(link.lastViewedAt, emptyCell)}
                        </TableCell>
                        <TableCell className="text-muted-foreground tabular-nums">
                          {formatDateTime(link.createdAt, emptyCell)}
                        </TableCell>
                        <TableCell
                          className="text-right"
                          onClick={stopRowActivation}
                          onPointerDown={stopRowActivation}
                        >
                          <Switch
                            checked={link.isActive ?? false}
                            onCheckedChange={(checked) => handleActiveChange(link.id, checked)}
                            disabled={!canManage}
                            aria-label={t("permissions.links.table.active")}
                          />
                        </TableCell>
                        <TableCell className="text-right" onClick={(e) => e.stopPropagation()}>
                          <div className="flex items-center justify-end gap-0.5">
                            <Button
                              type="button"
                              size="icon-sm"
                              variant="ghost"
                              className="shrink-0 text-muted-foreground"
                              aria-label={t("permissions.links.actions.view")}
                              title={t("permissions.links.actions.view")}
                              onClick={() => setViewLink(link)}
                            >
                              <ChartLine size={16} />
                            </Button>
                            {canManage ? (
                              <Button
                                type="button"
                                size="icon-sm"
                                variant="ghost"
                                className="shrink-0 text-muted-foreground"
                                aria-label={t("permissions.links.actions.edit")}
                                title={t("permissions.links.actions.edit")}
                                onClick={() => setEditLink(link)}
                              >
                                <PencilSimple size={16} />
                              </Button>
                            ) : null}
                            {pendingCount > 0 && canJumpToAccessRequests ? (
                              <Button
                                type="button"
                                size="icon-sm"
                                variant="ghost"
                                className="shrink-0 text-muted-foreground"
                                aria-label={t("permissions.links.actions.approveRequests")}
                                title={t("permissions.links.actions.approveRequests")}
                                onClick={() => onManageAccess(link.id)}
                              >
                                <UserPlus size={16} />
                              </Button>
                            ) : null}
                            {canManage && link.requireEmailVerification ? (
                              <Button
                                type="button"
                                size="icon-sm"
                                variant="ghost"
                                className="shrink-0 text-muted-foreground"
                                aria-label={t("permissions.links.actions.sendCode")}
                                title={t("permissions.links.actions.sendCode")}
                                onClick={() => setSendCodeLink(link)}
                              >
                                <EnvelopeSimple size={16} />
                              </Button>
                            ) : null}
                            {canManage ? (
                              <Button
                                type="button"
                                size="icon-sm"
                                variant="ghost"
                                className="shrink-0 text-muted-foreground hover:text-destructive"
                                aria-label={t("permissions.links.actions.delete")}
                                title={t("permissions.links.actions.delete")}
                                onClick={() => setDeleteLink(link)}
                              >
                                <Trash size={16} />
                              </Button>
                            ) : null}
                          </div>
                        </TableCell>
                      </TableRow>
                    );
                  })
                )}
              </TableBody>
            </Table>
          </div>
          <p className="text-caption text-muted-foreground">
            {t("permissions.links.table.visitsHint")}
          </p>

          <div
            className="flex flex-wrap items-center justify-end gap-2"
            data-testid="deal-room-links-pagination"
          >
            <span className="mr-1 text-xs text-muted-foreground">
              {t("permissions.links.pagination.pageOf", {
                page: currentPage,
                totalPages,
              })}
            </span>
            <Button
              type="button"
              size="sm"
              variant="ghost"
              disabled={currentPage <= 1 || loading}
              onClick={() => goToPage(1)}
              aria-label={t("permissions.links.pagination.first")}
            >
              {t("permissions.links.pagination.first")}
            </Button>
            <Button
              type="button"
              size="icon-sm"
              variant="outline"
              disabled={currentPage <= 1 || loading}
              onClick={() => goToPage(currentPage - 1)}
              aria-label={t("permissions.links.pagination.prev")}
            >
              <CaretLeft size={16} />
            </Button>
            <Input
              value={pageInput}
              onChange={(e) => setPageInput(e.target.value.replace(/[^\d]/g, ""))}
              onKeyDown={(e) => {
                if (e.key === "Enter") {
                  e.preventDefault();
                  handlePageJump();
                }
              }}
              className="h-8 w-14 text-center tabular-nums"
              aria-label={t("permissions.links.pagination.pageInput")}
              inputMode="numeric"
            />
            <Button
              type="button"
              size="sm"
              variant="outline"
              disabled={loading}
              onClick={handlePageJump}
            >
              {t("permissions.links.pagination.go")}
            </Button>
            <Button
              type="button"
              size="icon-sm"
              variant="outline"
              disabled={currentPage >= totalPages || loading}
              onClick={() => goToPage(currentPage + 1)}
              aria-label={t("permissions.links.pagination.next")}
            >
              <CaretRight size={16} />
            </Button>
            <Button
              type="button"
              size="sm"
              variant="ghost"
              disabled={currentPage >= totalPages || loading}
              onClick={() => goToPage(totalPages)}
              aria-label={t("permissions.links.pagination.last")}
            >
              {t("permissions.links.pagination.last")}
            </Button>
          </div>
        </div>
      )}

      {viewLink && (
        <LinkActivityDialog
          link={viewLink}
          open
          onOpenChange={(open) => !open && setViewLink(null)}
        />
      )}

      <DealRoomShareDialog
        roomId={roomId}
        slug={slug}
        open={createOpen}
        onChanged={refreshAll}
        onEditAccess={onManageAccess}
        onOpenChange={setCreateOpen}
      />

      {editLink && (
        <DealRoomShareDialog
          roomId={roomId}
          slug={slug}
          linkId={editLink.id}
          open
          onChanged={refreshAll}
          onEditAccess={onManageAccess}
          onOpenChange={(open) => !open && setEditLink(null)}
        />
      )}

      <SendVerificationCodeDialog
        link={sendCodeLink}
        open={!!sendCodeLink}
        onOpenChange={(open) => !open && setSendCodeLink(null)}
      />

      <Dialog open={!!deleteLink} onOpenChange={(open) => !open && setDeleteLink(null)}>
        <DialogContent className="sm:max-w-md">
          <DialogHeader>
            <DialogTitle>{t("permissions.links.delete.title")}</DialogTitle>
            <DialogDescription className="break-words">
              {t("permissions.links.delete.description", {
                name: deleteLink?.name || deleteLink?.shortUrl.split("/").pop(),
              })}
            </DialogDescription>
          </DialogHeader>
          <DialogFooter>
            <Button
              variant="outline"
              onClick={() => setDeleteLink(null)}
              disabled={deleteLoading}
            >
              {t("common:cancel")}
            </Button>
            <Button
              variant="destructive"
              disabled={deleteLoading}
              onClick={handleDelete}
            >
              {deleteLoading ? t("permissions.links.delete.loading") : t("common:delete")}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <Dialog open={bulkDeleteOpen} onOpenChange={(open) => !open && setBulkDeleteOpen(false)}>
        <DialogContent className="sm:max-w-md">
          <DialogHeader>
            <DialogTitle>{t("permissions.links.delete.bulkTitle")}</DialogTitle>
            <DialogDescription>
              {t("permissions.links.delete.bulkDescription", { count: selectedIds.size })}
            </DialogDescription>
          </DialogHeader>
          <DialogFooter>
            <Button
              variant="outline"
              onClick={() => setBulkDeleteOpen(false)}
              disabled={bulkDeleteLoading}
            >
              {t("common:cancel")}
            </Button>
            <Button
              variant="destructive"
              disabled={bulkDeleteLoading || selectedIds.size === 0}
              onClick={() => void handleBulkDelete()}
              data-testid="deal-room-bulk-delete-confirm"
            >
              {bulkDeleteLoading ? t("permissions.links.delete.loading") : t("common:delete")}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </>
  );
}
