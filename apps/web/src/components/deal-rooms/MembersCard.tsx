import { useState } from "react";
import { Users, X, Shield, User, Crown, Eye } from "@phosphor-icons/react";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { api } from "@/lib/api";
import { useTranslation } from "react-i18next";
import { toast } from "sonner";
import type { DealRoomMember } from "@/types";

interface MembersCardProps {
  roomId: string;
  members: DealRoomMember[];
  isAdmin?: boolean;
  onChanged: () => void;
}

const roleIcons: Record<DealRoomMember["role"], typeof User> = {
  owner: Crown,
  admin: Shield,
  member: User,
  viewer: Eye,
};

export function MembersCard({ roomId, members, isAdmin = true, onChanged }: MembersCardProps) {
  const { t } = useTranslation("dealRooms");
  const { t: tc } = useTranslation("common");
  const [removingId, setRemovingId] = useState<string | null>(null);

  const invitees = members.filter((m) => m.role !== "owner");

  const handleRemove = async (member: DealRoomMember) => {
    if (!confirm(t("members.removeConfirm", { email: member.email }))) return;
    setRemovingId(member.id);
    try {
      await api.removeDealRoomMember(roomId, member.id);
      toast.success(t("members.removed", { email: member.email }));
      onChanged();
    } catch (e) {
      toast.error(e instanceof Error ? e.message : tc("error.deleteFailed"));
    } finally {
      setRemovingId(null);
    }
  };

  return (
    <Card>
      <CardHeader>
        <CardTitle className="text-h2 flex items-center gap-2">
          <Users size={20} />
          {t("detail.invitees")}
        </CardTitle>
      </CardHeader>
      <CardContent>
        {invitees.length === 0 ? (
          <p className="text-body text-muted-foreground">{t("members.empty")}</p>
        ) : (
          <ul className="space-y-2">
            {invitees.map((member) => {
              const RoleIcon = roleIcons[member.role];
              return (
                <li
                  key={member.id}
                  className="flex items-center justify-between gap-3 rounded-md border border-border p-3"
                >
                  <div className="flex items-center gap-3 min-w-0">
                    <div className="flex h-8 w-8 shrink-0 items-center justify-center rounded-full bg-muted text-muted-foreground">
                      <RoleIcon size={16} />
                    </div>
                    <div className="min-w-0">
                      <p className="truncate text-sm font-medium">
                        {member.name ?? member.email}
                      </p>
                      <p className="text-caption text-muted-foreground truncate">{member.email}</p>
                    </div>
                  </div>
                  <div className="flex items-center gap-2 shrink-0">
                    <Badge variant="secondary">{t(`members.roles.${member.role}`)}</Badge>
                    {member.nda_status === "signed" && (
                      <Badge variant="outline" className="text-success-600 border-success-200">
                        {t("members.ndaSigned")}
                      </Badge>
                    )}
                    {isAdmin && member.role !== "owner" && (
                      <Button
                        variant="ghost"
                        size="icon-sm"
                        aria-label={t("members.remove", { email: member.email })}
                        onClick={() => handleRemove(member)}
                        disabled={removingId === member.id}
                      >
                        <X size={16} />
                      </Button>
                    )}
                  </div>
                </li>
              );
            })}
          </ul>
        )}
      </CardContent>
    </Card>
  );
}
