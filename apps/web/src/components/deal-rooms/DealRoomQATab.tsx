import { useMemo, useState, useEffect } from "react";
import { ChatCircleText } from "@phosphor-icons/react";
import { useTranslation } from "react-i18next";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { OwnerAskInboxPanel } from "@/components/ask/OwnerAskInboxPanel";
import { useOwnerAskCitationNavigation } from "@/lib/ownerAskCitation";
import { useAsyncData } from "@/hooks/useAsyncData";
import { api } from "@/lib/api";
import type { Link } from "@/types";

interface DealRoomQATabProps {
  roomId: string;
  /** Deep-link from dashboard or share surface (?linkId=). */
  initialLinkId?: string;
}

export function DealRoomQATab({ roomId, initialLinkId }: DealRoomQATabProps) {
  const { t } = useTranslation("dealRooms");
  const [linkFilter, setLinkFilter] = useState<string>(() => initialLinkId ?? "all");
  const onOpenCitation = useOwnerAskCitationNavigation(roomId);

  const { data: links } = useAsyncData(async () => {
    const linksRes = await api.getDealRoomLinks(roomId);
    return linksRes.data ?? [];
  }, [roomId]);

  const linkNameById = useMemo(() => {
    const map = new Map<string, string>();
    for (const link of links ?? []) {
      map.set(link.id, link.name?.trim() || link.documentTitle || link.id);
    }
    return map;
  }, [links]);

  useEffect(() => {
    if (!initialLinkId) return;
    if (links && links.some((link) => link.id === initialLinkId)) {
      setLinkFilter(initialLinkId);
    }
  }, [initialLinkId, links]);

  return (
    <div className="space-y-4">
      <Card>
        <CardHeader className="pb-3">
          <div className="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
            <div className="space-y-1">
              <CardTitle className="text-h2 flex items-center gap-2">
                <ChatCircleText size={20} />
                {t("qa.title")}
              </CardTitle>
              <CardDescription>{t("qa.description")}</CardDescription>
            </div>
            {(links?.length ?? 0) > 0 && (
              <Select
                value={linkFilter}
                onValueChange={(value) => {
                  if (value) setLinkFilter(value);
                }}
              >
                <SelectTrigger className="w-[220px]" aria-label={t("qa.filterByLink")}>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="all">{t("qa.filterAll")}</SelectItem>
                  {(links ?? []).map((link: Link) => (
                    <SelectItem key={link.id} value={link.id}>
                      {linkNameById.get(link.id) ?? link.id}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            )}
          </div>
        </CardHeader>
        <CardContent>
          <OwnerAskInboxPanel
            scope={{ type: "room", roomId, linkFilter }}
            i18nNs="dealRooms"
            linkLabels={linkNameById}
            onOpenCitation={onOpenCitation}
          />
        </CardContent>
      </Card>
    </div>
  );
}
