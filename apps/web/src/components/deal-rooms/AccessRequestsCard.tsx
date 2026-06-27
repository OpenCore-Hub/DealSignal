import { useState } from "react";
import { UserFocus, Check, X } from "@phosphor-icons/react";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { api } from "@/lib/api";
import { useTranslation } from "react-i18next";
import { toast } from "sonner";
import type { DealRoomAccessRequest } from "@/types";

interface AccessRequestsCardProps {
  roomId: string;
  requests: DealRoomAccessRequest[];
  isAdmin?: boolean;
  onChanged: () => void;
}

export function AccessRequestsCard({ roomId, requests, isAdmin = true, onChanged }: AccessRequestsCardProps) {
  const { t } = useTranslation("dealRooms");
  const { t: tc } = useTranslation("common");
  const [actingId, setActingId] = useState<string | null>(null);

  const pendingRequests = requests.filter((r) => r.status === "pending");

  const handleApprove = async (request: DealRoomAccessRequest) => {
    setActingId(request.id);
    try {
      await api.approveDealRoomAccessRequest(roomId, request.id);
      toast.success(t("access.approved", { email: request.email }));
      onChanged();
    } catch (e) {
      toast.error(e instanceof Error ? e.message : tc("error.saveFailed"));
    } finally {
      setActingId(null);
    }
  };

  const handleReject = async (request: DealRoomAccessRequest) => {
    if (!confirm(t("access.rejectConfirm", { email: request.email }))) return;
    setActingId(request.id);
    try {
      await api.rejectDealRoomAccessRequest(roomId, request.id);
      toast.success(t("access.rejected", { email: request.email }));
      onChanged();
    } catch (e) {
      toast.error(e instanceof Error ? e.message : tc("error.saveFailed"));
    } finally {
      setActingId(null);
    }
  };

  return (
    <Card>
      <CardHeader>
        <CardTitle className="text-h2 flex items-center gap-2">
          <UserFocus size={20} />
          {t("detail.accessRequests")}
        </CardTitle>
      </CardHeader>
      <CardContent>
        {pendingRequests.length === 0 ? (
          <p className="text-body text-muted-foreground">{t("access.empty")}</p>
        ) : (
          <ul className="space-y-2">
            {pendingRequests.map((request) => (
              <li
                key={request.id}
                className="flex flex-col gap-2 rounded-md border border-border p-3 sm:flex-row sm:items-center sm:justify-between"
              >
                <div className="min-w-0">
                  <p className="truncate text-sm font-medium">{request.email}</p>
                  {request.reason && (
                    <p className="text-caption text-muted-foreground line-clamp-2">{request.reason}</p>
                  )}
                </div>
                {isAdmin && (
                  <div className="flex items-center gap-2 shrink-0">
                    <Button
                      size="sm"
                      variant="outline"
                      className="gap-1"
                      onClick={() => handleReject(request)}
                      disabled={actingId === request.id}
                    >
                      <X size={14} />
                      {t("access.reject")}
                    </Button>
                    <Button
                      size="sm"
                      className="gap-1"
                      onClick={() => handleApprove(request)}
                      disabled={actingId === request.id}
                    >
                      <Check size={14} />
                      {t("access.approve")}
                    </Button>
                  </div>
                )}
                {!isAdmin && <Badge variant="secondary">{t("access.pending")}</Badge>}
              </li>
            ))}
          </ul>
        )}
      </CardContent>
    </Card>
  );
}
