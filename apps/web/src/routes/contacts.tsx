import { useEffect, useMemo, useState } from "react";
import { useLocation, useNavigate, useParams } from "react-router";
import { Users, MagnifyingGlass } from "@phosphor-icons/react";
import { Input } from "@/components/ui/input";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import { Checkbox } from "@/components/ui/checkbox";
import { Skeleton } from "@/components/ui/skeleton";
import { PageHeader } from "@/components/common/PageHeader";
import { HeatBadge } from "@/components/common/HeatBadge";
import { EmptyState } from "@/components/common/EmptyState";
import { api } from "@/lib/api";
import { formatDuration, formatRelativeTime } from "@/lib/formatters";
import { useAsyncData } from "@/hooks/useAsyncData";
import { useWorkspaceAccess } from "@/hooks/useWorkspaceAccess";
import { useTranslation } from "react-i18next";
import { cn } from "@/lib/utils";
import type { Contact } from "@/types";
import { MarketingBatchDialog } from "@/components/marketing/MarketingBatchDialog";

export type { Contact };

/** Workspace-scoped contacts list. Remounts on slug change so prior workspace data never leaks. */
export function ContactsPage() {
  const { workspaceSlug } = useParams<{ workspaceSlug: string }>();
  if (!workspaceSlug) return null;
  return <ContactsPageInner key={workspaceSlug} workspaceSlug={workspaceSlug} />;
}

function ContactsPageInner({ workspaceSlug }: { workspaceSlug: string }) {
  const { t, i18n } = useTranslation("contacts");
  const { t: tc } = useTranslation("common");
  const { canWrite } = useWorkspaceAccess(workspaceSlug);
  const navigate = useNavigate();
  const location = useLocation();

  const openContact = (contactId: string) => {
    navigate(`/${workspaceSlug}/contacts/${contactId}`, {
      state: {
        returnTo: location.pathname + location.search,
        returnLabel: t("detail.back"),
      },
    });
  };
  const [query, setQuery] = useState("");
  const [selectedIds, setSelectedIds] = useState<Set<string>>(() => new Set());
  // Fetch is always for the current workspace slug (API path + remount key).
  const { data: contacts, loading, error, refetch } = useAsyncData(async () => {
    const res = await api.getContacts(workspaceSlug);
    return res.data;
  }, [workspaceSlug]);

  // Drop stale selections when the contact list reloads.
  useEffect(() => {
    if (!contacts) return;
    const live = new Set(contacts.map((c) => c.id));
    setSelectedIds((prev) => {
      let changed = false;
      const next = new Set<string>();
      for (const id of prev) {
        if (live.has(id)) next.add(id);
        else changed = true;
      }
      return changed ? next : prev;
    });
  }, [contacts]);

  const filtered = useMemo(() => {
    const list = contacts ?? [];
    const q = query.toLowerCase();
    return list.filter(
      (c) =>
        c.name.toLowerCase().includes(q) ||
        c.email.toLowerCase().includes(q) ||
        (c.organization?.toLowerCase().includes(q) ?? false),
    );
  }, [contacts, query]);

  const selectedContacts = useMemo(() => {
    const list = contacts ?? [];
    if (selectedIds.size === 0) return [];
    return list.filter((c) => selectedIds.has(c.id));
  }, [contacts, selectedIds]);

  const allFilteredSelected =
    filtered.length > 0 && filtered.every((c) => selectedIds.has(c.id));
  const someFilteredSelected = filtered.some((c) => selectedIds.has(c.id));

  const toggleSelected = (contactId: string, checked: boolean | "indeterminate") => {
    setSelectedIds((prev) => {
      const next = new Set(prev);
      if (checked === true) next.add(contactId);
      else next.delete(contactId);
      return next;
    });
  };

  const toggleSelectAllFiltered = (checked: boolean | "indeterminate") => {
    setSelectedIds((prev) => {
      const next = new Set(prev);
      if (checked === true) {
        for (const c of filtered) next.add(c.id);
      } else {
        for (const c of filtered) next.delete(c.id);
      }
      return next;
    });
  };

  const clearSelection = () => setSelectedIds(new Set());

  return (
    <div className="space-y-6">
      <div className="flex items-start justify-between gap-4">
        <PageHeader title={t("page.title")} description={t("page.description")} />
        {canWrite ? (
          <MarketingBatchDialog
            workspaceSlug={workspaceSlug}
            contacts={selectedContacts}
            onSent={clearSelection}
          />
        ) : null}
      </div>

      <div className="flex flex-wrap items-center gap-3">
        <div className="relative max-w-sm flex-1 basis-64">
          <MagnifyingGlass
            size={18}
            className="absolute left-3 top-1/2 -translate-y-1/2 text-muted-foreground"
          />
          <Input
            placeholder={t("searchPlaceholder")}
            value={query}
            onChange={(e) => setQuery(e.target.value)}
            className="pl-9"
          />
        </div>
        {!loading && !error && filtered.length > 0 && canWrite ? (
          <div className="flex items-center gap-3">
            <label className="flex items-center gap-2 text-sm text-muted-foreground">
              <Checkbox
                checked={allFilteredSelected || someFilteredSelected}
                indeterminate={!allFilteredSelected && someFilteredSelected}
                onCheckedChange={toggleSelectAllFiltered}
                aria-label={t("selection.selectAll")}
                data-testid="contacts-select-all"
              />
              {t("selection.selectAll")}
            </label>
            {selectedIds.size > 0 ? (
              <>
                <span className="text-sm text-muted-foreground" data-testid="contacts-selected-count">
                  {t("selection.selectedCount", { count: selectedIds.size })}
                </span>
                <Button
                  type="button"
                  variant="ghost"
                  size="sm"
                  onClick={clearSelection}
                  data-testid="contacts-clear-selection"
                >
                  {t("selection.clear")}
                </Button>
              </>
            ) : null}
          </div>
        ) : null}
      </div>

      {error ? (
        <div className="flex flex-col items-center justify-center gap-4 rounded-lg border border-border bg-card p-12 text-center">
          <p className="text-body text-muted-foreground">{error}</p>
          <Button onClick={refetch}>{tc("retry")}</Button>
        </div>
      ) : loading ? (
        <div className="grid grid-cols-1 gap-4 md:grid-cols-2">
          <Skeleton className="h-28" />
          <Skeleton className="h-28" />
        </div>
      ) : filtered.length === 0 ? (
        <EmptyState
          icon={<Users size={48} />}
          title={t("empty.title")}
          description={t("empty.description")}
          size="large"
        />
      ) : (
        <div className="grid grid-cols-1 gap-4 md:grid-cols-2">
          {filtered.map((contact) => {
            const selected = selectedIds.has(contact.id);
            return (
              <Card
                key={contact.id}
                role="link"
                tabIndex={0}
                data-testid={`contact-card-${contact.id}`}
                data-selected={selected ? "true" : "false"}
                className={cn(
                  "cursor-pointer transition-colors hover:bg-muted/50",
                  selected && "border-primary/40 bg-primary/5",
                )}
                onClick={() => openContact(contact.id)}
                onKeyDown={(e) => {
                  if (e.key === "Enter" || e.key === " ") {
                    e.preventDefault();
                    openContact(contact.id);
                  }
                }}
              >
                <CardContent className="flex items-start gap-3 p-5">
                  {canWrite ? (
                    <div
                      className="pt-0.5"
                      onClick={(e) => e.stopPropagation()}
                      onKeyDown={(e) => e.stopPropagation()}
                    >
                      <Checkbox
                        checked={selected}
                        onCheckedChange={(v) => toggleSelected(contact.id, v)}
                        aria-label={t("selection.selectContact", { name: contact.name })}
                        data-testid={`contact-select-${contact.id}`}
                      />
                    </div>
                  ) : null}
                  <div className="min-w-0 flex-1">
                    <div className="flex items-start justify-between gap-3">
                      <div className="min-w-0">
                        <p className="text-h3 truncate">{contact.name}</p>
                        <p className="text-caption text-muted-foreground truncate">{contact.email}</p>
                        <p className="mt-2 text-caption text-muted-foreground">
                          {contact.organization || t("unknownOrganization")} ·{" "}
                          {t("visitCount", { count: contact.totalVisits })} ·{" "}
                          {t("totalDuration", {
                            duration: formatDuration(contact.totalDurationSeconds, i18n.language),
                          })}
                        </p>
                      </div>
                      <div className="shrink-0 text-right">
                        <HeatBadge level={contact.heatLevel} />
                        <p className="mt-1 text-caption text-muted-foreground">
                          {contact.lastSeenAt
                            ? t("lastSeen", {
                                time: formatRelativeTime(contact.lastSeenAt, i18n.language),
                              })
                            : "-"}
                        </p>
                      </div>
                    </div>
                  </div>
                </CardContent>
              </Card>
            );
          })}
        </div>
      )}
    </div>
  );
}
