import { useState, useMemo } from "react";
import { useLocation, useNavigate, useParams } from "react-router";
import { Users, MagnifyingGlass } from "@phosphor-icons/react";
import { Input } from "@/components/ui/input";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
import { PageHeader } from "@/components/common/PageHeader";
import { HeatBadge } from "@/components/common/HeatBadge";
import { EmptyState } from "@/components/common/EmptyState";
import { api } from "@/lib/api";
import { formatDuration, formatRelativeTime } from "@/lib/formatters";
import { useAsyncData } from "@/hooks/useAsyncData";
import { useTranslation } from "react-i18next";
import type { Contact } from "@/types";
import { MarketingBatchDialog } from "@/components/marketing/MarketingBatchDialog";

export type { Contact };

export function ContactsPage() {
  const { t, i18n } = useTranslation("contacts");
  const { t: tc } = useTranslation("common");
  const navigate = useNavigate();
  const { workspaceSlug } = useParams<{ workspaceSlug: string }>();
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
  const { data: contacts, loading, error, refetch } = useAsyncData(
    async () => {
      const res = await api.getContacts();
      return res.data;
    },
    []
  );

  const filtered = useMemo(() => {
    const list = contacts ?? [];
    const q = query.toLowerCase();
    return list.filter(
      (c) =>
        c.name.toLowerCase().includes(q) ||
        c.email.toLowerCase().includes(q) ||
        (c.organization?.toLowerCase().includes(q) ?? false)
    );
  }, [contacts, query]);

  return (
    <div className="space-y-6">
      <div className="flex items-start justify-between gap-4">
        <PageHeader title={t("page.title")} description={t("page.description")} />
        <MarketingBatchDialog contacts={contacts ?? []} />
      </div>

      <div className="relative max-w-sm">
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
          {filtered.map((contact) => (
            <Card
              key={contact.id}
              role="link"
              tabIndex={0}
              className="cursor-pointer transition-colors hover:bg-muted/50"
              onClick={() => openContact(contact.id)}
              onKeyDown={(e) => {
                if (e.key === "Enter" || e.key === " ") {
                  e.preventDefault();
                  openContact(contact.id);
                }
              }}
            >
              <CardContent className="flex items-start justify-between p-5">
                <div>
                  <p className="text-h3">{contact.name}</p>
                  <p className="text-caption text-muted-foreground">{contact.email}</p>
                  <p className="mt-2 text-caption text-muted-foreground">
                    {contact.organization || t("unknownOrganization")} · {t("visitCount", { count: contact.totalVisits })} · {t("totalDuration", { duration: formatDuration(contact.totalDurationSeconds, i18n.language) })}
                  </p>
                </div>
                <div className="text-right">
                  <HeatBadge level={contact.heatLevel} />
                  <p className="mt-1 text-caption text-muted-foreground">
                    {contact.lastSeenAt ? t("lastSeen", { time: formatRelativeTime(contact.lastSeenAt, i18n.language) }) : "-"}
                  </p>
                </div>
              </CardContent>
            </Card>
          ))}
        </div>
      )}
    </div>
  );
}
