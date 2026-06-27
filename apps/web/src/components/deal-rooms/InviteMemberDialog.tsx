import { useState } from "react";
import { Envelope, UserPlus } from "@phosphor-icons/react";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { api } from "@/lib/api";
import { useTranslation } from "react-i18next";
import { toast } from "sonner";
import type { DealRoomMemberRole } from "@/types";

interface InviteMemberDialogProps {
  roomId: string;
  onInvited: () => void;
  children?: React.ReactNode;
}

export function InviteMemberDialog({ roomId, onInvited, children }: InviteMemberDialogProps) {
  const { t } = useTranslation("dealRooms");
  const { t: tc } = useTranslation("common");
  const [open, setOpen] = useState(false);
  const [email, setEmail] = useState("");
  const [role, setRole] = useState<DealRoomMemberRole>("viewer");
  const [submitting, setSubmitting] = useState(false);

  const handleInvite = async () => {
    if (!email.trim()) return;
    setSubmitting(true);
    try {
      await api.inviteDealRoomMember(roomId, { email: email.trim(), role });
      toast.success(t("members.invited", { email: email.trim() }));
      setEmail("");
      setRole("viewer");
      setOpen(false);
      onInvited();
    } catch (e) {
      toast.error(e instanceof Error ? e.message : tc("error.saveFailed"));
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <Dialog open={open} onOpenChange={setOpen}>
      <DialogTrigger render={children ? (children as React.ReactElement) : (
        <Button variant="outline" className="gap-1.5">
          <Envelope size={16} />
          {t("detail.invite")}
        </Button>
      )} />
      <DialogContent className="sm:max-w-md">
        <DialogHeader>
          <DialogTitle className="flex items-center gap-2">
            <UserPlus size={20} />
            {t("members.inviteTitle")}
          </DialogTitle>
          <DialogDescription>{t("members.inviteDescription")}</DialogDescription>
        </DialogHeader>
        <div className="space-y-4 py-2">
          <div className="space-y-2">
            <Label htmlFor="member-email">{t("members.email")}</Label>
            <Input
              id="member-email"
              type="email"
              value={email}
              onChange={(e) => setEmail(e.target.value)}
              placeholder={t("members.emailPlaceholder")}
            />
          </div>
          <div className="space-y-2">
            <Label htmlFor="member-role">{t("members.role")}</Label>
            <Select value={role} onValueChange={(v) => setRole(v as DealRoomMemberRole)}>
              <SelectTrigger id="member-role" className="w-full">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="viewer">{t("members.roles.viewer")}</SelectItem>
                <SelectItem value="member">{t("members.roles.member")}</SelectItem>
                <SelectItem value="admin">{t("members.roles.admin")}</SelectItem>
                <SelectItem value="owner">{t("members.roles.owner")}</SelectItem>
              </SelectContent>
            </Select>
          </div>
        </div>
        <DialogFooter>
          <Button variant="outline" onClick={() => setOpen(false)}>
            {tc("cancel")}
          </Button>
          <Button onClick={handleInvite} disabled={!email.trim() || submitting}>
            {submitting ? t("members.inviting") : t("detail.invite")}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
