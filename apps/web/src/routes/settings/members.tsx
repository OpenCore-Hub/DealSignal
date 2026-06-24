import { useState } from "react";
import { Users, EnvelopeSimple } from "@phosphor-icons/react";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Badge } from "@/components/ui/badge";
import { Skeleton } from "@/components/ui/skeleton";
import { RowActions } from "@/components/common/RowActions";
import { api } from "@/lib/api";
import { getInitials } from "@/lib/formatters";
import { useTranslation } from "react-i18next";
import { useAsyncData } from "@/hooks/useAsyncData";

export function SettingsMembersPage() {
  const { t } = useTranslation("settings");
  const { data: members = [], loading, error, refetch } = useAsyncData(
    () => api.getWorkspaceMembers().then((res) => res.data),
    []
  );
  const [email, setEmail] = useState("");
  const [inviting, setInviting] = useState(false);

  const handleInvite = async () => {
    if (!email.trim()) return;
    setInviting(true);
    try {
      await api.inviteWorkspaceMember(email.trim(), "member");
      setEmail("");
      refetch();
    } finally {
      setInviting(false);
    }
  };

  return (
    <div className="space-y-6">
      <Card>
        <CardHeader>
          <CardTitle className="text-h2 flex items-center gap-2">
            <Users size={20} />
            {t("members.title")}
          </CardTitle>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="flex gap-2">
            <Input
              placeholder={t("members.emailPlaceholder")}
              className="max-w-sm"
              value={email}
              onChange={(e) => setEmail(e.target.value)}
              onKeyDown={(e) => e.key === "Enter" && handleInvite()}
            />
            <Button onClick={handleInvite} disabled={!email.trim() || inviting}>
              <EnvelopeSimple size={16} className="mr-1.5" />
              {inviting ? t("members.inviting") : t("members.invite")}
            </Button>
          </div>

          {error ? (
            <div className="rounded-lg border border-error-500/20 bg-error-100 p-4">
              <p className="text-sm font-medium text-error-500">{t("members.loadFailed")}</p>
              <p className="text-caption mt-1 text-error-500/80">{error}</p>
              <Button variant="outline" size="sm" className="mt-3" onClick={refetch}>
                {t("members.retry")}
              </Button>
            </div>
          ) : loading ? (
            <Skeleton className="h-40" />
          ) : (
            <ul className="divide-y divide-border">
              {(members ?? []).map((member) => (
                <li key={member.id} className="flex items-center justify-between py-3">
                  <div className="flex items-center gap-3">
                    <div className="flex h-9 w-9 items-center justify-center rounded-full bg-muted text-xs font-medium">
                      {getInitials(member.name)}
                    </div>
                    <div>
                      <p className="text-sm font-medium">{member.name}</p>
                      <p className="text-caption text-muted-foreground">{member.email}</p>
                    </div>
                  </div>
                  <div className="flex items-center gap-3">
                    <Badge variant={member.status === "active" ? "default" : "secondary"}>{member.role}</Badge>
                    <RowActions
                      actions={[
                        { label: t("members.editRole"), onClick: () => {}, disabled: true, title: t("members.editRoleDisabled") },
                        { label: t("members.remove"), onClick: () => {}, destructive: true, disabled: true, title: t("members.removeDisabled") },
                      ]}
                    />
                  </div>
                </li>
              ))}
            </ul>
          )}
        </CardContent>
      </Card>
    </div>
  );
}
