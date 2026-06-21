import { useEffect, useState } from "react";
import { Users, EnvelopeSimple, DotsThree } from "@phosphor-icons/react";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Badge } from "@/components/ui/badge";
import { Skeleton } from "@/components/ui/skeleton";
import { api, getInitials } from "@/lib/api";
import type { WorkspaceMember } from "@/types";

export function SettingsMembersPage() {
  const [members, setMembers] = useState<WorkspaceMember[]>([]);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    api.getWorkspaceMembers().then((res) => {
      setMembers(res.data);
      setLoading(false);
    });
  }, []);

  return (
    <div className="space-y-6">
      <Card>
        <CardHeader>
          <CardTitle className="text-h2 flex items-center gap-2">
            <Users size={20} />
            成员管理
          </CardTitle>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="flex gap-2">
            <Input placeholder="邮箱地址" className="max-w-sm" />
            <Button>
              <EnvelopeSimple size={16} className="mr-1.5" />
              邀请
            </Button>
          </div>

          {loading ? (
            <Skeleton className="h-40" />
          ) : (
            <ul className="divide-y divide-border">
              {members.map((member) => (
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
                    <Badge variant={member.status === "active" ? "default" : "secondary"}>
                      {member.role}
                    </Badge>
                    <Button size="icon-xs" variant="ghost">
                      <DotsThree size={16} />
                    </Button>
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
