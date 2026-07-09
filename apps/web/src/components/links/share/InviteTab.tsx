import { useTranslation } from "react-i18next";
import { Envelope, DotsThree } from "@phosphor-icons/react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Badge } from "@/components/ui/badge";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import type { LinkInvitation } from "@/types";
import { cn } from "@/lib/utils";
import { copyToClipboard } from "@/lib/clipboard";

interface InviteTabProps {
  linkId?: string;
  publicUrl?: string;
  emailsRaw: string;
  setEmailsRaw: (value: string) => void;
  invalid: string[];
  sending: boolean;
  invitations: LinkInvitation[];
  loading: boolean;
  onSend: () => void;
  onResend: (email: string) => void;
  onRevoke: (invitation: LinkInvitation) => void;
}

function formatSentAt(value?: string): string {
  if (!value) return "—";
  try {
    return new Date(value).toLocaleString();
  } catch {
    return value;
  }
}

export function InviteTab({
  linkId,
  publicUrl,
  emailsRaw,
  setEmailsRaw,
  invalid,
  sending,
  invitations,
  loading,
  onSend,
  onResend,
  onRevoke,
}: InviteTabProps) {
  const { t } = useTranslation("linkShare");

  const statusVariant = (status: string) => {
    switch (status) {
      case "verified":
        return "default";
      case "pending":
        return "warm";
      case "opened":
        return "cold";
      case "expired":
        return "hot";
      case "revoked":
      default:
        return "secondary";
    }
  };

  if (!linkId) {
    return (
      <div className="flex flex-col items-center justify-center gap-3 py-10 text-center">
        <Envelope size={40} className="text-muted-foreground" />
        <p className="text-sm text-muted-foreground">{t("invite.createLinkFirst")}</p>
      </div>
    );
  }

  return (
    <div className="space-y-4 py-2">
      <div className="space-y-2">
        <Label htmlFor="invite-emails">{t("invite.addViewers")}</Label>
        <div className="flex items-start gap-2">
          <div className="flex-1 space-y-2">
            <Input
              id="invite-emails"
              value={emailsRaw}
              onChange={(e) => setEmailsRaw(e.target.value)}
              placeholder={t("invite.addViewersPlaceholder")}
              disabled={sending}
            />
            <p className="text-xs text-muted-foreground">{t("invite.addViewersHint")}</p>
            {invalid.length > 0 && (
              <p className="text-xs text-destructive">
                {t("invite.invalidEmails", { emails: invalid.join(", ") })}
              </p>
            )}
          </div>
          <Button onClick={onSend} disabled={sending || !emailsRaw.trim()}>
            {sending ? t("invite.sending") : t("invite.sendInvitations")}
          </Button>
        </div>
      </div>

      {loading ? (
        <p className="py-4 text-center text-sm text-muted-foreground">{t("common:loading")}</p>
      ) : invitations.length === 0 ? (
        <div className="flex flex-col items-center justify-center gap-2 py-8 text-center">
          <p className="text-sm font-medium">{t("invite.emptyTitle")}</p>
          <p className="text-xs text-muted-foreground">{t("invite.emptyDescription")}</p>
        </div>
      ) : (
        <div className="overflow-hidden rounded-lg border">
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>{t("invite.table.email")}</TableHead>
                <TableHead>{t("invite.table.status")}</TableHead>
                <TableHead>{t("invite.table.sentAt")}</TableHead>
                <TableHead className="text-right">{t("invite.table.actions")}</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {invitations.map((inv) => (
                <TableRow key={inv.id} className={cn(inv.status === "revoked" && "opacity-50")}>
                  <TableCell>{inv.email}</TableCell>
                  <TableCell>
                    <Badge variant={statusVariant(inv.status)}>
                      {t(`invite.status.${inv.status}`)}
                    </Badge>
                  </TableCell>
                  <TableCell className="text-muted-foreground">
                    {formatSentAt(inv.createdAt ?? inv.expiresAt)}
                  </TableCell>
                  <TableCell className="text-right">
                    <DropdownMenu>
                      <DropdownMenuTrigger
                        render={(
                          <Button variant="ghost" size="icon">
                            <DotsThree size={16} />
                          </Button>
                        )}
                      />
                      <DropdownMenuContent align="end">
                        {publicUrl && (
                          <DropdownMenuItem
                            onClick={() =>
                              copyToClipboard(`${publicUrl}?inviteToken=${inv.token}`, t("invite.copied"))
                            }
                          >
                            {t("invite.copyInviteLink")}
                          </DropdownMenuItem>
                        )}
                        <DropdownMenuItem onClick={() => onResend(inv.email)}>
                          {t("invite.resend")}
                        </DropdownMenuItem>
                        <DropdownMenuItem
                          variant="destructive"
                          onClick={() => onRevoke(inv)}
                          disabled={inv.status === "revoked"}
                        >
                          {t("invite.revoke")}
                        </DropdownMenuItem>
                      </DropdownMenuContent>
                    </DropdownMenu>
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </div>
      )}
    </div>
  );
}
